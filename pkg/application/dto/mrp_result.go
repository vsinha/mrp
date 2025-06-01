package dto

import (
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

// MRPResult contains the complete output of an MRP run
type MRPResult struct {
	PlannedOrders  []entities.PlannedOrder                `json:"planned_orders"`
	Allocations    []entities.AllocationResult            `json:"allocations"`
	ShortageReport []entities.Shortage                    `json:"shortages"`
	ExplosionCache map[ExplosionCacheKey]*ExplosionResult `json:"-"`
}

// ExplosionCacheKey is used for memoizing BOM explosion results
type ExplosionCacheKey struct {
	PartNumber        entities.PartNumber
	SerialEffectivity entities.SerialEffectivity
}

// ExplosionResult contains cached results of BOM explosion
type ExplosionResult struct {
	Requirements []entities.GrossRequirement
	LeadTimeDays int
	ComputedAt   time.Time
}
