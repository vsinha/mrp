package mrp

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// EngineConfig holds configuration for MRP engine optimization
type EngineConfig struct {
	// EnableGCPacing enables GC tuning for large operations
	EnableGCPacing bool
	// MaxCacheEntries limits the explosion cache size (0 = unlimited)
	MaxCacheEntries int
}

// Engine implements the MRP planning logic
type Engine struct {
	bomRepo       *BOMRepository
	inventoryRepo *InventoryRepository
	serialComp    *SerialComparator
	config        EngineConfig
	
	// Memoization cache for BOM explosions
	explosionCache map[ExplosionCacheKey]*ExplosionResult
	cacheMutex     sync.RWMutex
}

// NewEngine creates a new MRP engine with the provided repositories
func NewEngine(bomRepo *BOMRepository, inventoryRepo *InventoryRepository) *Engine {
	return NewEngineWithConfig(bomRepo, inventoryRepo, EngineConfig{
		EnableGCPacing:  true,
		MaxCacheEntries: 10000,
	})
}

// NewEngineWithConfig creates a new MRP engine with custom configuration
func NewEngineWithConfig(bomRepo *BOMRepository, inventoryRepo *InventoryRepository, config EngineConfig) *Engine {
	return &Engine{
		bomRepo:        bomRepo,
		inventoryRepo:  inventoryRepo,
		serialComp:     NewSerialComparator(),
		config:         config,
		explosionCache: make(map[ExplosionCacheKey]*ExplosionResult),
	}
}

// ExplodeDemand performs complete MRP explosion for the given demands
func (e *Engine) ExplodeDemand(ctx context.Context, demands []DemandRequirement) (*MRPResult, error) {
	// Set GC pacing for large operations
	var oldGCPercent int
	if e.config.EnableGCPacing && len(demands) > 100 {
		oldGCPercent = int(debug.SetGCPercent(50)) // More aggressive GC for large operations
		defer debug.SetGCPercent(oldGCPercent)
	}
	
	// Pre-allocate result slices with estimated capacity for better performance
	estimatedOrders := len(demands) * 50 // Conservative estimate
	result := &MRPResult{
		PlannedOrders:   make([]PlannedOrder, 0, estimatedOrders),
		Allocations:     make([]AllocationResult, 0, len(demands)*10),
		ShortageReport:  make([]Shortage, 0, estimatedOrders/2),
		ExplosionCache:  make(map[ExplosionCacheKey]*ExplosionResult),
	}
	
	// Step 1: Explode all demands to gross requirements
	var allGrossRequirements []GrossRequirement
	
	for _, demand := range demands {
		grossReqs, err := e.explodeRequirements(ctx, demand.PartNumber, demand.TargetSerial, 
			demand.NeedDate, demand.DemandSource, demand.Location, decimal.Decimal(demand.Quantity))
		if err != nil {
			return nil, fmt.Errorf("failed to explode demand for %s: %w", demand.PartNumber, err)
		}
		allGrossRequirements = append(allGrossRequirements, grossReqs...)
	}
	
	// Step 2: Allocate available inventory against gross requirements
	allocations, netRequirements, err := e.allocateInventory(ctx, allGrossRequirements)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate inventory: %w", err)
	}
	
	result.Allocations = allocations
	
	// Step 3: Generate planned orders for net requirements
	plannedOrders := e.createPlannedOrders(netRequirements)
	result.PlannedOrders = plannedOrders
	
	// Step 4: Identify shortages
	shortages := e.identifyShortages(netRequirements, plannedOrders)
	result.ShortageReport = shortages
	
	// Step 5: Copy explosion cache to result
	e.cacheMutex.RLock()
	for key, value := range e.explosionCache {
		result.ExplosionCache[key] = value
	}
	e.cacheMutex.RUnlock()
	
	// Clean cache if it's getting too large
	e.cleanCacheIfNeeded()
	
	return result, nil
}

// explodeRequirements recursively explodes a part's BOM with memoization
func (e *Engine) explodeRequirements(ctx context.Context, pn PartNumber, targetSerial string, 
	needDate time.Time, demandTrace string, location string, quantity decimal.Decimal) ([]GrossRequirement, error) {
	
	// Create cache key for memoization
	cacheKey := ExplosionCacheKey{
		PartNumber:        pn,
		SerialEffectivity: SerialEffectivity{FromSerial: targetSerial, ToSerial: targetSerial},
	}
	
	// Check cache first
	e.cacheMutex.RLock()
	cached, exists := e.explosionCache[cacheKey]
	e.cacheMutex.RUnlock()
	
	if exists {
		// Scale cached quantities by the current demand quantity
		var scaledRequirements []GrossRequirement
		for _, req := range cached.Requirements {
			scaledReq := req
			scaledReq.Quantity = Quantity(decimal.Decimal(req.Quantity).Mul(quantity))
			scaledReq.DemandTrace = demandTrace + " -> " + req.DemandTrace
			scaledReq.NeedDate = needDate.Add(-time.Duration(cached.LeadTimeDays) * 24 * time.Hour)
			scaledReq.Location = location
			scaledRequirements = append(scaledRequirements, scaledReq)
		}
		return scaledRequirements, nil
	}
	
	// Get item master data
	item, err := e.bomRepo.GetItem(ctx, pn)
	if err != nil {
		return nil, fmt.Errorf("failed to get item %s: %w", pn, err)
	}
	
	// Get effective BOM for this part and target serial
	bomLines, err := e.bomRepo.GetEffectiveBOM(ctx, pn, targetSerial)
	if err != nil {
		return nil, fmt.Errorf("failed to get BOM for %s: %w", pn, err)
	}
	
	// Filter BOM lines by serial effectivity
	effectiveLines := e.serialComp.ResolveSerialEffectivity(targetSerial, bomLines)
	
	var requirements []GrossRequirement
	
	// Always create requirement for this part itself
	req := GrossRequirement{
		PartNumber:   pn,
		Quantity:     Quantity(quantity),
		NeedDate:     needDate,
		DemandTrace:  demandTrace,
		Location:     location,
		TargetSerial: targetSerial,
	}
	requirements = append(requirements, req)
	
	// If this part has BOM lines, recursively explode child requirements
	if len(effectiveLines) > 0 {
		for _, line := range effectiveLines {
			childQty := decimal.Decimal(line.QtyPer).Mul(quantity)
			childNeedDate := needDate.Add(-time.Duration(item.LeadTimeDays) * 24 * time.Hour)
			childTrace := demandTrace + " -> " + string(line.ChildPN)
			
			childRequirements, err := e.explodeRequirements(ctx, line.ChildPN, targetSerial,
				childNeedDate, childTrace, location, childQty)
			if err != nil {
				return nil, fmt.Errorf("failed to explode child %s: %w", line.ChildPN, err)
			}
			
			requirements = append(requirements, childRequirements...)
		}
	}
	
	// Cache the base requirements (without scaling)
	baseRequirements := make([]GrossRequirement, len(requirements))
	copy(baseRequirements, requirements)
	
	// Scale back to unit quantity for caching
	for i := range baseRequirements {
		baseRequirements[i].Quantity = Quantity(decimal.Decimal(baseRequirements[i].Quantity).Div(quantity))
		// Remove current demand trace prefix for generic caching
		baseRequirements[i].DemandTrace = string(baseRequirements[i].PartNumber)
	}
	
	explosionResult := &ExplosionResult{
		Requirements: baseRequirements,
		LeadTimeDays: item.LeadTimeDays,
		ComputedAt:   time.Now(),
	}
	
	e.cacheMutex.Lock()
	e.explosionCache[cacheKey] = explosionResult
	e.cacheMutex.Unlock()
	
	return requirements, nil
}

// allocateInventory allocates available inventory against gross requirements
func (e *Engine) allocateInventory(ctx context.Context, grossRequirements []GrossRequirement) ([]AllocationResult, []NetRequirement, error) {
	// Group requirements by part number and location
	reqMap := make(map[string][]GrossRequirement)
	
	for _, req := range grossRequirements {
		key := fmt.Sprintf("%s:%s", req.PartNumber, req.Location)
		reqMap[key] = append(reqMap[key], req)
	}
	
	var allocations []AllocationResult
	var netRequirements []NetRequirement
	
	// Process each part/location combination
	for _, reqs := range reqMap {
		// Calculate total demand for this part/location
		totalDemand := decimal.Zero
		for _, req := range reqs {
			totalDemand = totalDemand.Add(decimal.Decimal(req.Quantity))
		}
		
		// Get first requirement for part number and location
		firstReq := reqs[0]
		
		// Get available inventory
		lotInventory, serialInventory, err := e.inventoryRepo.GetAvailableInventory(ctx, firstReq.PartNumber, firstReq.Location)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get inventory for %s at %s: %w", 
				firstReq.PartNumber, firstReq.Location, err)
		}
		
		// Allocate inventory using FIFO logic
		allocation, remaining := e.allocateFIFO(firstReq.PartNumber, firstReq.Location, 
			Quantity(totalDemand), lotInventory, serialInventory)
		
		if allocation.AllocatedQty.Decimal().IsPositive() {
			allocations = append(allocations, allocation)
		}
		
		// Create net requirements for remaining demand
		if remaining.Decimal().IsPositive() {
			// Distribute remaining demand proportionally across original requirements
			for _, req := range reqs {
				proportion := decimal.Decimal(req.Quantity).Div(totalDemand)
				netQty := remaining.Decimal().Mul(proportion)
				
				if netQty.IsPositive() {
					netReq := NetRequirement{
						PartNumber:   req.PartNumber,
						Quantity:     Quantity(netQty),
						NeedDate:     req.NeedDate,
						DemandTrace:  req.DemandTrace,
						Location:     req.Location,
						TargetSerial: req.TargetSerial,
					}
					netRequirements = append(netRequirements, netReq)
				}
			}
		}
	}
	
	return allocations, netRequirements, nil
}

// allocateFIFO performs FIFO allocation within location/lot
func (e *Engine) allocateFIFO(pn PartNumber, location string, demandQty Quantity, 
	lotInventory []InventoryLot, serialInventory []SerializedInventory) (AllocationResult, Quantity) {
	
	result := AllocationResult{
		PartNumber:      pn,
		Location:        location,
		AllocatedQty:    Quantity(decimal.Zero),
		RemainingDemand: demandQty,
		AllocatedFrom:   []InventoryAllocation{},
	}
	
	remaining := decimal.Decimal(demandQty)
	
	// First allocate from serialized inventory (FIFO by receipt date)
	for _, inv := range serialInventory {
		if inv.Status != Available || remaining.IsZero() {
			continue
		}
		
		// Serialized items are quantity 1
		allocated := decimal.NewFromInt(1)
		remaining = remaining.Sub(allocated)
		
		allocation := InventoryAllocation{
			SerialNumber: inv.SerialNumber,
			Quantity:     Quantity(allocated),
			Location:     location,
		}
		
		result.AllocatedFrom = append(result.AllocatedFrom, allocation)
		result.AllocatedQty = Quantity(decimal.Decimal(result.AllocatedQty).Add(allocated))
	}
	
	// Then allocate from lot inventory (FIFO by receipt date within lot)
	for _, lot := range lotInventory {
		if lot.Status != Available || remaining.IsZero() {
			continue
		}
		
		availableInLot := decimal.Decimal(lot.Quantity)
		allocateFromLot := decimal.Min(remaining, availableInLot)
		
		if allocateFromLot.IsPositive() {
			allocation := InventoryAllocation{
				LotNumber: lot.LotNumber,
				Quantity:  Quantity(allocateFromLot),
				Location:  location,
			}
			
			result.AllocatedFrom = append(result.AllocatedFrom, allocation)
			result.AllocatedQty = Quantity(decimal.Decimal(result.AllocatedQty).Add(allocateFromLot))
			remaining = remaining.Sub(allocateFromLot)
		}
	}
	
	result.RemainingDemand = Quantity(remaining)
	return result, Quantity(remaining)
}

// createPlannedOrders generates planned orders for net requirements
func (e *Engine) createPlannedOrders(netRequirements []NetRequirement) []PlannedOrder {
	var orders []PlannedOrder
	
	for _, req := range netRequirements {
		// Apply lot sizing rules (simplified - would need item master data)
		orderQty := decimal.Decimal(req.Quantity)
		
		// Calculate start date (simplified - assumes 30 day lead time)
		startDate := req.NeedDate.Add(-30 * 24 * time.Hour)
		
		order := PlannedOrder{
			PartNumber:   req.PartNumber,
			Quantity:     Quantity(orderQty),
			StartDate:    startDate,
			DueDate:      req.NeedDate,
			DemandTrace:  req.DemandTrace,
			Location:     req.Location,
			OrderType:    Make, // Simplified - would determine based on item type
			TargetSerial: req.TargetSerial,
		}
		
		orders = append(orders, order)
	}
	
	return orders
}

// identifyShortages identifies parts that cannot be fulfilled
func (e *Engine) identifyShortages(netRequirements []NetRequirement, plannedOrders []PlannedOrder) []Shortage {
	var shortages []Shortage
	
	// For this simplified implementation, treat all net requirements as potential shortages
	// In a full implementation, this would check supplier capacity, lead times, etc.
	for _, req := range netRequirements {
		shortage := Shortage{
			PartNumber:   req.PartNumber,
			Location:     req.Location,
			ShortQty:     req.Quantity,
			NeedDate:     req.NeedDate,
			DemandTrace:  req.DemandTrace,
			TargetSerial: req.TargetSerial,
		}
		shortages = append(shortages, shortage)
	}
	
	return shortages
}

// cleanCacheIfNeeded removes old cache entries if the cache is getting too large
func (e *Engine) cleanCacheIfNeeded() {
	if e.config.MaxCacheEntries <= 0 {
		return // Unlimited cache
	}
	
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()
	
	if len(e.explosionCache) > e.config.MaxCacheEntries {
		// Simple cache eviction: clear half the cache
		// In a production system, you might want LRU eviction
		newCache := make(map[ExplosionCacheKey]*ExplosionResult)
		count := 0
		target := e.config.MaxCacheEntries / 2
		
		for key, value := range e.explosionCache {
			if count < target {
				newCache[key] = value
				count++
			} else {
				break
			}
		}
		
		e.explosionCache = newCache
		
		// Force GC to clean up evicted cache entries
		if e.config.EnableGCPacing {
			runtime.GC()
		}
	}
}

// Helper method to convert Quantity to decimal.Decimal
func (q Quantity) Decimal() decimal.Decimal {
	return decimal.Decimal(q)
}