package dto

import (
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

// MRPResult contains the complete output of an MRP run
type MRPResult struct {
	PlannedOrders  []entities.PlannedOrder
	Allocations    []entities.AllocationResult
	ShortageReport []entities.Shortage
	ExplosionCache map[ExplosionCacheKey]*ExplosionResult
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
