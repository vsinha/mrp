package mrp

import (
	"context"
	"time"

	"github.com/vsinha/mrp/pkg/application/services/shared"
	"github.com/vsinha/mrp/pkg/domain/entities"
)

// MRPVisitor implements BOMNodeVisitor for MRP explosion
type MRPVisitor struct {
	demandTrace string
	needDate    time.Time
}

// MRPNodeData holds data for an MRP node during traversal
type MRPNodeData struct {
	SelfRequirement *entities.GrossRequirement
}

// NewMRPVisitor creates a new MRP visitor
func NewMRPVisitor(demandTrace string, needDate time.Time) *MRPVisitor {
	return &MRPVisitor{
		demandTrace: demandTrace,
		needDate:    needDate,
	}
}

// VisitNode creates a gross requirement for this node
func (v *MRPVisitor) VisitNode(
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

	nodeData := &MRPNodeData{
		SelfRequirement: req,
	}

	// Always continue traversal for MRP
	return nodeData, true, nil
}

// ProcessChildren combines this node's requirement with child requirements
func (v *MRPVisitor) ProcessChildren(
	ctx context.Context,
	nodeCtx shared.BOMNodeContext,
	nodeData interface{},
	childResults []interface{},
) (interface{}, error) {
	mrpNodeData := nodeData.(*MRPNodeData)

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
