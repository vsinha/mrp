package mrp

import (
	"context"
	"fmt"

	"github.com/vsinha/mrp/pkg/application/services/shared"
	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
)

// BOMNodeContext provides context information during BOM traversal
type BOMNodeContext struct {
	PartNumber        entities.PartNumber
	Item              *entities.Item
	Quantity          entities.Quantity
	TargetSerial      string
	Location          string
	Level             int
	AllocationContext *shared.AllocationContext // Optional allocation info
}

// BOMNodeVisitor defines the interface for processing nodes during BOM traversal
type BOMNodeVisitor interface {
	// VisitNode is called for each node in the BOM structure
	// Returns data to be passed to children and whether to continue traversal
	VisitNode(ctx context.Context, nodeCtx BOMNodeContext) (interface{}, bool, error)

	// ProcessChildren is called after visiting all children
	// Receives the node context, data from VisitNode, and results from children
	ProcessChildren(
		ctx context.Context,
		nodeCtx BOMNodeContext,
		nodeData interface{},
		childResults []interface{},
	) (interface{}, error)
}

// BOMTraverser provides common BOM traversal logic with alternate selection
type BOMTraverser struct {
	bomRepo       repositories.BOMRepository
	itemRepo      repositories.ItemRepository
	inventoryRepo repositories.InventoryRepository
	allocationMap shared.AllocationMap
}

// NewBOMTraverser creates a new BOM traverser
func NewBOMTraverser(
	bomRepo repositories.BOMRepository,
	itemRepo repositories.ItemRepository,
	inventoryRepo repositories.InventoryRepository,
) *BOMTraverser {
	return &BOMTraverser{
		bomRepo:       bomRepo,
		itemRepo:      itemRepo,
		inventoryRepo: inventoryRepo,
		allocationMap: shared.NewAllocationMap(),
	}
}

// SetAllocationContext updates the allocation information for parts
func (bt *BOMTraverser) SetAllocationContext(allocations []entities.AllocationResult) {
	bt.allocationMap = shared.NewAllocationMapFromResults(allocations)
}

// SetAllocationMap directly sets the allocation map
func (bt *BOMTraverser) SetAllocationMap(allocMap shared.AllocationMap) {
	bt.allocationMap = allocMap
}

// GetAllocationMap returns a copy of the current allocation map
func (bt *BOMTraverser) GetAllocationMap() shared.AllocationMap {
	// Return a copy to prevent external modifications
	copyMap := shared.NewAllocationMap()
	for key, context := range bt.allocationMap {
		copyMap[key] = context
	}
	return copyMap
}

// ClearAllocationContext removes allocation information
func (bt *BOMTraverser) ClearAllocationContext() {
	bt.allocationMap.Clear()
}

// TraverseBOM performs BOM traversal with alternate selection using the visitor pattern
func (bt *BOMTraverser) TraverseBOM(
	ctx context.Context,
	partNumber entities.PartNumber,
	targetSerial string,
	location string,
	quantity entities.Quantity,
	level int,
	visitor BOMNodeVisitor,
) (interface{}, error) {
	// Get item master data
	item, err := bt.itemRepo.GetItem(partNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get item %s: %w", partNumber, err)
	}

	// Get allocation context for this part
	allocationCtx := bt.allocationMap.Get(partNumber, location)

	// Create node context
	nodeCtx := BOMNodeContext{
		PartNumber:        partNumber,
		Item:              item,
		Quantity:          quantity,
		TargetSerial:      targetSerial,
		Location:          location,
		Level:             level,
		AllocationContext: allocationCtx,
	}

	// Visit this node
	nodeData, shouldContinue, err := visitor.VisitNode(ctx, nodeCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to visit node %s: %w", partNumber, err)
	}

	if !shouldContinue {
		// Visitor decided to stop traversal at this node
		return visitor.ProcessChildren(ctx, nodeCtx, nodeData, nil)
	}

	// Get alternate groups and select best alternates
	alternateGroups, err := bt.bomRepo.GetAlternateGroups(partNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get alternate groups for %s: %w", partNumber, err)
	}

	var childResults []interface{}

	// For each FindNumber group, select best alternate and traverse
	for findNumber := range alternateGroups {
		effectiveAlternates, err := bt.bomRepo.GetEffectiveAlternates(
			partNumber,
			findNumber,
			targetSerial,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to get effective alternates for %s find %d: %w",
				partNumber,
				findNumber,
				err,
			)
		}

		if len(effectiveAlternates) == 0 {
			continue // No effective alternates for this serial
		}

		// Select the best alternate from effective ones
		selectedAlternate := shared.SelectBestAlternateByPriority(effectiveAlternates)
		if selectedAlternate == nil {
			continue // No suitable alternate found
		}

		// Recursively traverse the selected alternate
		childQty := selectedAlternate.QtyPer * quantity
		childResult, err := bt.TraverseBOM(
			ctx,
			selectedAlternate.ChildPN,
			targetSerial,
			location,
			childQty,
			level+1,
			visitor,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to traverse child %s: %w",
				selectedAlternate.ChildPN,
				err,
			)
		}

		childResults = append(childResults, childResult)
	}

	// Let visitor process the children results
	return visitor.ProcessChildren(ctx, nodeCtx, nodeData, childResults)
}
