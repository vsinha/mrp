package repositories

import "github.com/vsinha/mrp/pkg/domain/entities"

// DemandRepository provides access to demand data
type DemandRepository interface {
	GetDemands() ([]*entities.DemandRequirement, error)
	LoadDemands(demands []*entities.DemandRequirement) error
}
