package incremental

import (
	"context"
	"time"

	"github.com/vsinha/mrp/pkg/application/services/shared"
	"github.com/vsinha/mrp/pkg/domain/entities"
)

type IncrementalMRPVisitor struct {
	demandTrace string
	needDate    time.Time
}

type IncrementalMRPNodeData struct {
	SelfRequirement *entities.GrossRequirement
}

func NewIncrementalMRPVisitor(demandTrace string, needDate time.Time) *IncrementalMRPVisitor {
	return &IncrementalMRPVisitor{
		demandTrace: demandTrace,
		needDate:    needDate,
	}
}

func (v *IncrementalMRPVisitor) VisitNode(
	ctx context.Context,
	nodeCtx shared.BOMNodeContext,
) (interface{}, bool, error) {
	// Create requirement for this part itself
	req := &entities.GrossRequirement{
		PartNumber:   nodeCtx.PartNumber,
		Quantity:     nodeCtx.Quantity,
		NeedDate:     v.needDate,
		DemandTrace:  v.demandTrace,
		Location:     nodeCtx.Location,
		TargetSerial: nodeCtx.TargetSerial,
	}

	nodeData := &IncrementalMRPNodeData{
		SelfRequirement: req,
	}

	// Always continue traversal for MRP
	return nodeData, true, nil
}

func (v *IncrementalMRPVisitor) ProcessChildren(
	ctx context.Context,
	nodeCtx shared.BOMNodeContext,
	nodeData interface{},
	childResults []interface{},
) (interface{}, error) {
	mrpNodeData := nodeData.(*IncrementalMRPNodeData)

	// Start with this node's requirement
	var allRequirements []*entities.GrossRequirement
	allRequirements = append(allRequirements, mrpNodeData.SelfRequirement)

	// Add all child requirements
	for _, childResult := range childResults {
		if childResult != nil {
			childRequirements := childResult.([]*entities.GrossRequirement)
			allRequirements = append(allRequirements, childRequirements...)
		}
	}

	return allRequirements, nil
}
