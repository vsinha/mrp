package memory

import (
	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
)

// DemandRepository provides in-memory demand storage
type DemandRepository struct {
	demands []entities.DemandRequirement
}

// NewDemandRepository creates a new in-memory demand repository
func NewDemandRepository() *DemandRepository {
	return &DemandRepository{
		demands: []entities.DemandRequirement{},
	}
}

// Verify interface compliance
var _ repositories.DemandRepository = (*DemandRepository)(nil)

// LoadDemands loads demands into the repository
func (r *DemandRepository) LoadDemands(demands []*entities.DemandRequirement) error {
	for _, demand := range demands {
		r.demands = append(r.demands, *demand)
	}
	return nil
}

// GetDemands returns all demand requirements
func (r *DemandRepository) GetDemands() ([]*entities.DemandRequirement, error) {
	var demands []*entities.DemandRequirement
	for i := range r.demands {
		demands = append(demands, &r.demands[i])
	}
	return demands, nil
}
