package criticalpath

import (
	"context"

	"github.com/vsinha/mrp/pkg/application/services/shared"
	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
	"github.com/vsinha/mrp/pkg/domain/services"
)

// CriticalPathVisitor implements BOMNodeVisitor for critical path analysis
type CriticalPathVisitor struct {
	inventoryRepo repositories.InventoryRepository
	serialComp    *services.SerialComparator
}

// CriticalPathNodeData holds data for a critical path node during traversal
type CriticalPathNodeData struct {
	Node              entities.CriticalPathNode
	EffectiveLeadTime int
}

// NewCriticalPathVisitor creates a new critical path visitor
func NewCriticalPathVisitor(
	inventoryRepo repositories.InventoryRepository,
	serialComp *services.SerialComparator,
) *CriticalPathVisitor {
	return &CriticalPathVisitor{
		inventoryRepo: inventoryRepo,
		serialComp:    serialComp,
	}
}

// VisitNode creates a critical path node for this part
func (v *CriticalPathVisitor) VisitNode(
	ctx context.Context,
	nodeCtx shared.BOMNodeContext,
) (interface{}, bool, error) {
	var hasInventory bool
	var inventoryQty entities.Quantity
	var effectiveLeadTime int

	// Use allocation context if available, otherwise fall back to inventory check
	if nodeCtx.AllocationContext != nil {
		// Use allocation results to determine inventory coverage
		alloc := nodeCtx.AllocationContext
		hasInventory = alloc.HasAllocation
		inventoryQty = alloc.AllocatedQty

		// Calculate effective lead time based on allocation coverage
		if alloc.AllocatedQty >= nodeCtx.Quantity {
			// Full allocation coverage - zero lead time
			effectiveLeadTime = 0
		} else if alloc.AllocatedQty > 0 {
			// Partial allocation coverage - reduced lead time
			coverageRatio := float64(alloc.AllocatedQty) / float64(nodeCtx.Quantity)
			effectiveLeadTime = int(float64(nodeCtx.Item.LeadTimeDays) * (1.0 - coverageRatio))
		} else {
			// No allocation - full lead time
			effectiveLeadTime = nodeCtx.Item.LeadTimeDays
		}
	} else {
		// Fallback to inventory check if no allocation context
		hasInventory, inventoryQty, effectiveLeadTime = v.checkInventoryAvailability(
			ctx, nodeCtx.PartNumber, nodeCtx.Location, nodeCtx.Quantity, nodeCtx.Item.LeadTimeDays)
	}

	// Create node for this part
	node := entities.CriticalPathNode{
		PartNumber:        nodeCtx.PartNumber,
		Description:       nodeCtx.Item.Description,
		LeadTimeDays:      nodeCtx.Item.LeadTimeDays,
		Level:             nodeCtx.Level,
		HasInventory:      hasInventory,
		InventoryQty:      inventoryQty,
		RequiredQty:       nodeCtx.Quantity,
		EffectiveLeadTime: effectiveLeadTime,
	}

	nodeData := &CriticalPathNodeData{
		Node:              node,
		EffectiveLeadTime: effectiveLeadTime,
	}

	// Always continue traversal for critical path analysis
	return nodeData, true, nil
}

// ProcessChildren creates critical paths by combining this node with child paths
func (v *CriticalPathVisitor) ProcessChildren(
	ctx context.Context,
	nodeCtx shared.BOMNodeContext,
	nodeData interface{},
	childResults []interface{},
) (interface{}, error) {
	criticalPathNodeData := nodeData.(*CriticalPathNodeData)
	node := criticalPathNodeData.Node
	effectiveLeadTime := criticalPathNodeData.EffectiveLeadTime

	// If no children (leaf node), return single path
	if len(childResults) == 0 {
		path := entities.CriticalPath{
			TotalLeadTime:     nodeCtx.Item.LeadTimeDays,
			EffectiveLeadTime: effectiveLeadTime,
			PathLength:        1,
			Path:              []entities.PartNumber{nodeCtx.PartNumber},
			PathDetails:       []entities.CriticalPathNode{node},
			BottleneckPart:    nodeCtx.PartNumber,
		}
		return []entities.CriticalPath{path}, nil
	}

	// Collect all child paths
	var allChildPaths []entities.CriticalPath
	for _, childResult := range childResults {
		if childResult != nil {
			childPaths := childResult.([]entities.CriticalPath)
			allChildPaths = append(allChildPaths, childPaths...)
		}
	}

	// Build paths from this node through each child path
	var resultPaths []entities.CriticalPath

	for _, childPath := range allChildPaths {
		// Calculate cumulative times
		node.CumulativeTime = effectiveLeadTime + childPath.EffectiveLeadTime

		// Determine bottleneck (part with longest individual lead time in path)
		bottleneck := node.PartNumber
		if nodeCtx.Item.LeadTimeDays < v.getLeadTimeForPart(
			childPath.BottleneckPart,
			childPath.PathDetails,
		) {
			bottleneck = childPath.BottleneckPart
		}

		// Create new path by prepending this node
		newPath := entities.CriticalPath{
			TotalLeadTime:     nodeCtx.Item.LeadTimeDays + childPath.TotalLeadTime,
			EffectiveLeadTime: effectiveLeadTime + childPath.EffectiveLeadTime,
			PathLength:        1 + childPath.PathLength,
			Path:              append([]entities.PartNumber{nodeCtx.PartNumber}, childPath.Path...),
			PathDetails:       append([]entities.CriticalPathNode{node}, childPath.PathDetails...),
			BottleneckPart:    bottleneck,
		}

		resultPaths = append(resultPaths, newPath)
	}

	return resultPaths, nil
}

// checkInventoryAvailability checks if inventory is available and calculates effective lead time
func (v *CriticalPathVisitor) checkInventoryAvailability(
	ctx context.Context,
	partNumber entities.PartNumber,
	location string,
	requiredQty entities.Quantity,
	baseLeadTime int,
) (bool, entities.Quantity, int) {
	// Get available inventory
	lotInventory, err := v.inventoryRepo.GetInventoryLots(partNumber, location)
	if err != nil {
		// If we can't get inventory info, assume no inventory
		return false, 0, baseLeadTime
	}

	serialInventory, err := v.inventoryRepo.GetSerializedInventory(partNumber, location)
	if err != nil {
		// If we can't get serialized inventory info, use just lot inventory
		serialInventory = []*entities.SerializedInventory{}
	}

	// Calculate total available quantity
	var totalAvailable entities.Quantity

	// Add serialized inventory (each serial = 1 unit)
	for _, inv := range serialInventory {
		if inv.Status == entities.Available {
			totalAvailable += 1
		}
	}

	// Add lot inventory
	for _, lot := range lotInventory {
		if lot.Status == entities.Available {
			totalAvailable += lot.Quantity
		}
	}

	hasInventory := totalAvailable > 0
	availableQty := totalAvailable

	// Calculate effective lead time based on inventory coverage
	if totalAvailable >= requiredQty {
		// Full inventory coverage - zero lead time
		return hasInventory, availableQty, 0
	} else if totalAvailable > 0 {
		// Partial inventory coverage - reduced lead time
		partialReduction := float64(totalAvailable) / float64(requiredQty)
		effectiveLeadTime := int(float64(baseLeadTime) * (1.0 - partialReduction))
		return hasInventory, availableQty, effectiveLeadTime
	} else {
		// No inventory - full lead time
		return hasInventory, availableQty, baseLeadTime
	}
}

// getLeadTimeForPart finds the lead time for a specific part in the path details
func (v *CriticalPathVisitor) getLeadTimeForPart(
	partNumber entities.PartNumber,
	pathDetails []entities.CriticalPathNode,
) int {
	for _, node := range pathDetails {
		if node.PartNumber == partNumber {
			return node.LeadTimeDays
		}
	}
	return 0
}
