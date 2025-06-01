package services

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/vsinha/mrp/pkg/application/dto"
	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
)

// EngineConfig holds configuration for MRP engine optimization
type EngineConfig struct {
	// EnableGCPacing enables GC tuning for large operations
	EnableGCPacing bool
	// MaxCacheEntries limits the explosion cache size (0 = unlimited)
	MaxCacheEntries int
}

// MRPService implements the MRP planning logic using clean architecture
type MRPService struct {
	config EngineConfig

	// Memoization cache for BOM explosions
	explosionCache map[dto.ExplosionCacheKey]*dto.ExplosionResult
	cacheMutex     sync.RWMutex
}

// NewMRPService creates a new MRP service with default configuration
func NewMRPService() *MRPService {
	return NewMRPServiceWithConfig(EngineConfig{
		EnableGCPacing:  true,
		MaxCacheEntries: 10000,
	})
}

// NewMRPServiceWithConfig creates a new MRP service with custom configuration
func NewMRPServiceWithConfig(config EngineConfig) *MRPService {
	return &MRPService{
		config:         config,
		explosionCache: make(map[dto.ExplosionCacheKey]*dto.ExplosionResult),
	}
}

// ExplodeDemand performs complete MRP explosion for the given demands
func (s *MRPService) ExplodeDemand(
	ctx context.Context,
	demands []*entities.DemandRequirement,
	bomRepo repositories.BOMRepository,
	itemRepo repositories.ItemRepository,
	inventoryRepo repositories.InventoryRepository,
	demandRepo repositories.DemandRepository,
) (*dto.MRPResult, error) {
	// Set GC pacing for large operations
	var oldGCPercent int
	if s.config.EnableGCPacing && len(demands) > 100 {
		oldGCPercent = int(debug.SetGCPercent(50)) // More aggressive GC for large operations
		defer debug.SetGCPercent(oldGCPercent)
	}

	// Pre-allocate result slices with estimated capacity for better performance
	estimatedOrders := len(demands) * 50 // Conservative estimate
	result := &dto.MRPResult{
		PlannedOrders:  make([]entities.PlannedOrder, 0, estimatedOrders),
		Allocations:    make([]entities.AllocationResult, 0, len(demands)*10),
		ShortageReport: make([]entities.Shortage, 0, estimatedOrders/2),
		ExplosionCache: make(map[dto.ExplosionCacheKey]*dto.ExplosionResult),
	}

	// Step 1: Explode all demands to gross requirements
	var allGrossRequirements []*entities.GrossRequirement

	for _, demand := range demands {
		grossReqs, err := s.explodeRequirements(
			ctx,
			demand.PartNumber,
			demand.TargetSerial,
			demand.NeedDate,
			demand.DemandSource,
			demand.Location,
			demand.Quantity,
			bomRepo,
			itemRepo,
			inventoryRepo,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to explode demand for %s: %w", demand.PartNumber, err)
		}
		allGrossRequirements = append(allGrossRequirements, grossReqs...)
	}

	// Step 2: Allocate available inventory against gross requirements
	allocations, netRequirements, err := s.allocateInventory(
		ctx,
		allGrossRequirements,
		inventoryRepo,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate inventory: %w", err)
	}

	result.Allocations = allocations

	// Step 3: Generate planned orders for net requirements
	plannedOrders, err := s.createPlannedOrders(netRequirements, itemRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to create planned orders: %w", err)
	}
	result.PlannedOrders = plannedOrders

	// Step 4: Identify shortages
	shortages := s.identifyShortages(netRequirements, plannedOrders)
	result.ShortageReport = shortages

	// Step 5: Copy explosion cache to result
	s.cacheMutex.RLock()
	for key, value := range s.explosionCache {
		result.ExplosionCache[key] = value
	}
	s.cacheMutex.RUnlock()

	// Clean cache if it's getting too large
	s.cleanCacheIfNeeded()

	return result, nil
}

// explodeRequirements recursively explodes a part's BOM with memoization using BOMTraverser
func (s *MRPService) explodeRequirements(
	ctx context.Context,
	pn entities.PartNumber,
	targetSerial string,
	needDate time.Time,
	demandTrace string,
	location string,
	quantity entities.Quantity,
	bomRepo repositories.BOMRepository,
	itemRepo repositories.ItemRepository,
	inventoryRepo repositories.InventoryRepository,
) ([]*entities.GrossRequirement, error) {

	// Create cache key for memoization
	cacheKey := dto.ExplosionCacheKey{
		PartNumber: pn,
		SerialEffectivity: entities.SerialEffectivity{
			FromSerial: targetSerial,
			ToSerial:   targetSerial,
		},
	}

	// Check cache first
	s.cacheMutex.RLock()
	cached, exists := s.explosionCache[cacheKey]
	s.cacheMutex.RUnlock()

	if exists {
		// Scale cached quantities by the current demand quantity
		var scaledRequirements []*entities.GrossRequirement
		for _, req := range cached.Requirements {
			scaledReq := &entities.GrossRequirement{
				PartNumber:   req.PartNumber,
				Quantity:     req.Quantity * quantity,
				NeedDate:     needDate.Add(-time.Duration(cached.LeadTimeDays) * 24 * time.Hour),
				DemandTrace:  demandTrace + " -> " + req.DemandTrace,
				Location:     location,
				TargetSerial: req.TargetSerial,
			}
			scaledRequirements = append(scaledRequirements, scaledReq)
		}
		return scaledRequirements, nil
	}

	// Use BOMTraverser with MRPVisitor to perform the explosion
	bomTraverser := NewBOMTraverser(bomRepo, itemRepo, inventoryRepo)
	visitor := NewMRPVisitor(demandTrace, needDate)
	result, err := bomTraverser.TraverseBOM(ctx, pn, targetSerial, location, quantity, 0, visitor)
	if err != nil {
		return nil, fmt.Errorf("failed to traverse BOM for %s: %w", pn, err)
	}

	requirements := result.([]*entities.GrossRequirement)

	// Get item master data for caching
	item, err := itemRepo.GetItem(pn)
	if err != nil {
		return nil, fmt.Errorf("failed to get item %s: %w", pn, err)
	}

	// Cache the base requirements (without scaling)
	baseRequirements := make([]entities.GrossRequirement, len(requirements))
	for i, req := range requirements {
		baseRequirements[i] = entities.GrossRequirement{
			PartNumber:   req.PartNumber,
			Quantity:     req.Quantity / quantity, // Scale back to unit quantity
			NeedDate:     req.NeedDate,
			DemandTrace:  string(req.PartNumber), // Generic trace for caching
			Location:     req.Location,
			TargetSerial: req.TargetSerial,
		}
	}

	explosionResult := &dto.ExplosionResult{
		Requirements: baseRequirements,
		LeadTimeDays: item.LeadTimeDays,
		ComputedAt:   time.Now(),
	}

	// Store in cache
	s.cacheMutex.Lock()
	s.explosionCache[cacheKey] = explosionResult
	s.cacheMutex.Unlock()

	return requirements, nil
}

// allocateInventory allocates available inventory against gross requirements
func (s *MRPService) allocateInventory(
	ctx context.Context,
	grossReqs []*entities.GrossRequirement,
	inventoryRepo repositories.InventoryRepository,
) ([]entities.AllocationResult, []*entities.NetRequirement, error) {
	var allocations []entities.AllocationResult
	var netRequirements []*entities.NetRequirement

	// Group requirements by part number and location
	reqGroups := make(map[string][]*entities.GrossRequirement)
	for _, req := range grossReqs {
		key := fmt.Sprintf("%s|%s", req.PartNumber, req.Location)
		reqGroups[key] = append(reqGroups[key], req)
	}

	// Process each group
	for _, reqs := range reqGroups {
		if len(reqs) == 0 {
			continue
		}

		firstReq := reqs[0]
		totalQty := entities.Quantity(0)
		for _, req := range reqs {
			totalQty += req.Quantity
		}

		// Try to allocate inventory
		allocation, err := inventoryRepo.AllocateInventory(
			firstReq.PartNumber,
			firstReq.Location,
			totalQty,
		)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"failed to allocate inventory for %s: %w",
				firstReq.PartNumber,
				err,
			)
		}

		allocations = append(allocations, *allocation)

		// Create net requirements for unallocated quantities
		if allocation.RemainingDemand > 0 {
			// Distribute remaining demand across original requirements
			remainingQty := allocation.RemainingDemand
			for _, req := range reqs {
				if remainingQty <= 0 {
					break
				}

				netQty := req.Quantity
				if netQty > remainingQty {
					netQty = remainingQty
				}

				if netQty > 0 {
					netReq := &entities.NetRequirement{
						PartNumber:   req.PartNumber,
						Quantity:     netQty,
						NeedDate:     req.NeedDate,
						DemandTrace:  req.DemandTrace,
						Location:     req.Location,
						TargetSerial: req.TargetSerial,
					}
					netRequirements = append(netRequirements, netReq)
					remainingQty -= netQty
				}
			}
		}
	}

	return allocations, netRequirements, nil
}

// createPlannedOrders generates planned orders for net requirements
func (s *MRPService) createPlannedOrders(
	netReqs []*entities.NetRequirement,
	itemRepo repositories.ItemRepository,
) ([]entities.PlannedOrder, error) {
	var orders []entities.PlannedOrder

	for _, netReq := range netReqs {
		// Get item to determine lead time and order type
		item, err := itemRepo.GetItem(netReq.PartNumber)
		if err != nil {
			// Create order with default values if item not found
			startDate := netReq.NeedDate.Add(-7 * 24 * time.Hour) // Default 7 day lead time
			order, createErr := entities.NewPlannedOrder(
				netReq.PartNumber,
				netReq.Quantity,
				startDate,
				netReq.NeedDate,
				netReq.DemandTrace,
				netReq.Location,
				entities.Make,
				netReq.TargetSerial,
			)
			if createErr != nil {
				return nil, fmt.Errorf(
					"failed to create planned order for %s: %w",
					netReq.PartNumber,
					createErr,
				)
			}
			orders = append(orders, *order)
			continue
		}

		// Apply lot sizing rules
		orderQty := s.applyLotSizing(netReq.Quantity, item)

		// Determine order type (simplified logic)
		orderType := entities.Make
		if item.LeadTimeDays > 30 {
			orderType = entities.Buy // Long lead time items are typically purchased
		}

		startDate := netReq.NeedDate.Add(-time.Duration(item.LeadTimeDays) * 24 * time.Hour)
		order, err := entities.NewPlannedOrder(
			netReq.PartNumber,
			orderQty,
			startDate,
			netReq.NeedDate,
			netReq.DemandTrace,
			netReq.Location,
			orderType,
			netReq.TargetSerial,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to create planned order for %s: %w",
				netReq.PartNumber,
				err,
			)
		}

		orders = append(orders, *order)
	}

	return orders, nil
}

// applyLotSizing applies lot sizing rules to determine order quantity
func (s *MRPService) applyLotSizing(
	netQty entities.Quantity,
	item *entities.Item,
) entities.Quantity {
	switch item.LotSizeRule {
	case entities.LotForLot:
		return netQty
	case entities.MinimumQty:
		if netQty < item.MinOrderQty {
			return item.MinOrderQty
		}
		return netQty
	case entities.StandardPack:
		// Round up to nearest standard pack size (using MinOrderQty as pack size)
		if item.MinOrderQty > 0 {
			packs := (netQty + item.MinOrderQty - 1) / item.MinOrderQty
			return packs * item.MinOrderQty
		}
		return netQty
	default:
		return netQty
	}
}

// identifyShortages identifies unfulfilled demand
func (s *MRPService) identifyShortages(
	netReqs []*entities.NetRequirement,
	orders []entities.PlannedOrder,
) []entities.Shortage {
	var shortages []entities.Shortage

	// Create map of planned orders by part/location for quick lookup
	orderMap := make(map[string]entities.Quantity)
	for _, order := range orders {
		key := fmt.Sprintf("%s|%s", order.PartNumber, order.Location)
		orderMap[key] += order.Quantity
	}

	// Check each net requirement against planned orders
	reqMap := make(map[string]entities.Quantity)
	for _, netReq := range netReqs {
		key := fmt.Sprintf("%s|%s", netReq.PartNumber, netReq.Location)
		reqMap[key] += netReq.Quantity
	}

	for key, totalReq := range reqMap {
		plannedQty := orderMap[key]
		if plannedQty < totalReq {
			// Find a representative net requirement for shortage details
			var shortageReq *entities.NetRequirement
			for _, netReq := range netReqs {
				reqKey := fmt.Sprintf("%s|%s", netReq.PartNumber, netReq.Location)
				if reqKey == key {
					shortageReq = netReq
					break
				}
			}

			if shortageReq != nil {
				shortage := entities.Shortage{
					PartNumber:   shortageReq.PartNumber,
					Location:     shortageReq.Location,
					ShortQty:     totalReq - plannedQty,
					NeedDate:     shortageReq.NeedDate,
					DemandTrace:  shortageReq.DemandTrace,
					TargetSerial: shortageReq.TargetSerial,
				}
				shortages = append(shortages, shortage)
			}
		}
	}

	return shortages
}

// cleanCacheIfNeeded removes old cache entries if cache size exceeds limit
func (s *MRPService) cleanCacheIfNeeded() {
	if s.config.MaxCacheEntries <= 0 {
		return
	}

	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	if len(s.explosionCache) > s.config.MaxCacheEntries {
		// Simple LRU eviction - remove oldest entries
		var oldestTime time.Time
		var oldestKey dto.ExplosionCacheKey

		for key, value := range s.explosionCache {
			if oldestTime.IsZero() || value.ComputedAt.Before(oldestTime) {
				oldestTime = value.ComputedAt
				oldestKey = key
			}
		}

		delete(s.explosionCache, oldestKey)
	}
}
