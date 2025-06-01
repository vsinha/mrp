package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
	"github.com/vsinha/mrp/pkg/domain/services"
)

// CriticalPathService performs critical path analysis on BOM structures
type CriticalPathService struct {
	bomRepo           repositories.BOMRepository
	itemRepo          repositories.ItemRepository
	inventoryRepo     repositories.InventoryRepository
	serialComp        *services.SerialComparator
	alternateSelector *AlternateSelector
}

// NewCriticalPathService creates a new critical path service
func NewCriticalPathService(
	bomRepo repositories.BOMRepository,
	itemRepo repositories.ItemRepository,
	inventoryRepo repositories.InventoryRepository,
	serialComp *services.SerialComparator,
) *CriticalPathService {
	alternateSelector := NewAlternateSelector(inventoryRepo, itemRepo)
	return &CriticalPathService{
		bomRepo:           bomRepo,
		itemRepo:          itemRepo,
		inventoryRepo:     inventoryRepo,
		serialComp:        serialComp,
		alternateSelector: alternateSelector,
	}
}

// AnalyzeCriticalPath performs critical path analysis for a given part and returns top N paths
func (cps *CriticalPathService) AnalyzeCriticalPath(ctx context.Context, partNumber entities.PartNumber, targetSerial string, location string, topN int) (*entities.CriticalPathAnalysis, error) {
	// Get all paths through the BOM
	allPaths, err := cps.findAllPaths(ctx, partNumber, targetSerial, location, 1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to find paths for %s: %w", partNumber, err)
	}

	if len(allPaths) == 0 {
		return &entities.CriticalPathAnalysis{
			TopLevelPart: partNumber,
			TargetSerial: targetSerial,
			Location:     location,
			AnalysisDate: time.Now(),
			TotalPaths:   0,
		}, nil
	}

	// Sort paths by effective lead time (descending)
	sort.Slice(allPaths, func(i, j int) bool {
		// Primary sort: effective lead time
		if allPaths[i].EffectiveLeadTime != allPaths[j].EffectiveLeadTime {
			return allPaths[i].EffectiveLeadTime > allPaths[j].EffectiveLeadTime
		}
		// Secondary sort: total lead time (ignoring inventory)
		if allPaths[i].TotalLeadTime != allPaths[j].TotalLeadTime {
			return allPaths[i].TotalLeadTime > allPaths[j].TotalLeadTime
		}
		// Tertiary sort: path length (longer paths first)
		return allPaths[i].PathLength > allPaths[j].PathLength
	})

	// Get top N paths
	topPaths := allPaths
	if len(allPaths) > topN {
		topPaths = allPaths[:topN]
	}

	analysis := &entities.CriticalPathAnalysis{
		TopLevelPart: partNumber,
		TargetSerial: targetSerial,
		Location:     location,
		AnalysisDate: time.Now(),
		CriticalPath: allPaths[0], // Longest path
		TopPaths:     topPaths,
		TotalPaths:   len(allPaths),
	}

	return analysis, nil
}

// findAllPaths recursively finds all paths through the BOM structure
func (cps *CriticalPathService) findAllPaths(ctx context.Context, partNumber entities.PartNumber, targetSerial string, location string, quantity entities.Quantity, level int) ([]entities.CriticalPath, error) {
	// Get item master data
	item, err := cps.itemRepo.GetItem(partNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get item %s: %w", partNumber, err)
	}

	// Check inventory availability
	hasInventory, inventoryQty, effectiveLeadTime := cps.checkInventoryAvailability(ctx, partNumber, location, quantity, item.LeadTimeDays)

	// Create node for this part
	node := entities.CriticalPathNode{
		PartNumber:        partNumber,
		Description:       item.Description,
		LeadTimeDays:      item.LeadTimeDays,
		Level:             level,
		HasInventory:      hasInventory,
		InventoryQty:      inventoryQty,
		RequiredQty:       quantity,
		EffectiveLeadTime: effectiveLeadTime,
	}

	// Get alternate groups for this part and select best alternates
	alternateGroups, err := cps.bomRepo.GetAlternateGroups(partNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get alternate groups for %s: %w", partNumber, err)
	}

	var selectedLines []*entities.BOMLine

	// For each FindNumber group, select the best effective alternate
	for findNumber := range alternateGroups {
		effectiveAlternates, err := cps.bomRepo.GetEffectiveAlternates(partNumber, findNumber, targetSerial)
		if err != nil {
			return nil, fmt.Errorf("failed to get effective alternates for %s find %d: %w", partNumber, findNumber, err)
		}

		if len(effectiveAlternates) > 0 {
			// Select the best alternate from effective ones
			selectedAlternate := cps.alternateSelector.SelectBestAlternate(effectiveAlternates)
			if selectedAlternate != nil {
				selectedLines = append(selectedLines, selectedAlternate)
			}
		}
	}

	// If no children (leaf node), return single path
	if len(selectedLines) == 0 {
		path := entities.CriticalPath{
			TotalLeadTime:     item.LeadTimeDays,
			EffectiveLeadTime: effectiveLeadTime,
			PathLength:        1,
			Path:              []entities.PartNumber{partNumber},
			PathDetails:       []entities.CriticalPathNode{node},
			BottleneckPart:    partNumber,
		}
		return []entities.CriticalPath{path}, nil
	}

	// Recursively get paths for all children
	var allChildPaths []entities.CriticalPath

	for _, line := range selectedLines {
		childQty := line.QtyPer * quantity
		childPaths, err := cps.findAllPaths(ctx, line.ChildPN, targetSerial, location, childQty, level+1)
		if err != nil {
			return nil, fmt.Errorf("failed to get paths for child %s: %w", line.ChildPN, err)
		}
		allChildPaths = append(allChildPaths, childPaths...)
	}

	// Build paths from this node through each child path
	var resultPaths []entities.CriticalPath

	for _, childPath := range allChildPaths {
		// Calculate cumulative times
		node.CumulativeTime = effectiveLeadTime + childPath.EffectiveLeadTime

		// Determine bottleneck (part with longest individual lead time in path)
		bottleneck := node.PartNumber
		if item.LeadTimeDays < cps.getLeadTimeForPart(childPath.BottleneckPart, childPath.PathDetails) {
			bottleneck = childPath.BottleneckPart
		}

		// Create new path by prepending this node
		newPath := entities.CriticalPath{
			TotalLeadTime:     item.LeadTimeDays + childPath.TotalLeadTime,
			EffectiveLeadTime: effectiveLeadTime + childPath.EffectiveLeadTime,
			PathLength:        1 + childPath.PathLength,
			Path:              append([]entities.PartNumber{partNumber}, childPath.Path...),
			PathDetails:       append([]entities.CriticalPathNode{node}, childPath.PathDetails...),
			BottleneckPart:    bottleneck,
		}

		resultPaths = append(resultPaths, newPath)
	}

	return resultPaths, nil
}

// checkInventoryAvailability checks if inventory is available and calculates effective lead time
func (cps *CriticalPathService) checkInventoryAvailability(ctx context.Context, partNumber entities.PartNumber, location string, requiredQty entities.Quantity, baseLeadTime int) (bool, entities.Quantity, int) {
	// Get available inventory
	lotInventory, err := cps.inventoryRepo.GetInventoryLots(partNumber, location)
	if err != nil {
		// If we can't get inventory info, assume no inventory
		return false, 0, baseLeadTime
	}

	serialInventory, err := cps.inventoryRepo.GetSerializedInventory(partNumber, location)
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
		// Simple model: reduce lead time proportionally
		coverageRatio := float64(totalAvailable) / float64(requiredQty)
		effectiveLeadTime := float64(baseLeadTime) * (1.0 - coverageRatio)
		return hasInventory, availableQty, int(effectiveLeadTime)
	} else {
		// No inventory - full lead time
		return false, 0, baseLeadTime
	}
}

// getLeadTimeForPart finds the lead time for a specific part in the path details
func (cps *CriticalPathService) getLeadTimeForPart(partNumber entities.PartNumber, pathDetails []entities.CriticalPathNode) int {
	for _, node := range pathDetails {
		if node.PartNumber == partNumber {
			return node.LeadTimeDays
		}
	}
	return 0
}
