package mrp

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/vsinha/mrp/pkg/application/dto"
	"github.com/vsinha/mrp/pkg/application/services/shared"
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

// DependencyNode represents a part in the dependency graph for forward scheduling
type DependencyNode struct {
	PartNumber     entities.PartNumber
	Item           *entities.Item
	GrossQuantity  entities.Quantity
	Level          int
	DirectChildren []entities.PartNumber // Parts this part depends on (immediate children only)
	DirectParents  []entities.PartNumber // Parts that depend on this part (immediate parents only)
}

// DependencyGraph maps part numbers to their dependency information
type DependencyGraph map[entities.PartNumber]*DependencyNode

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

// ExplodeDemand performs complete MRP explosion with forward scheduling for the given demands
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

	// MULTI-PASS FORWARD SCHEDULING APPROACH

	// Pass 1: Explode all demands to gross requirements using BOM traverser
	var allGrossRequirements []*entities.GrossRequirement
	targetSerial := "" // Will use first demand's target serial for dependency graph

	for _, demand := range demands {
		if targetSerial == "" {
			targetSerial = demand.TargetSerial
		}

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

	// Pass 2: Allocate available inventory against gross requirements FIRST
	allocations, netRequirements, err := s.allocateInventory(
		ctx,
		allGrossRequirements,
		inventoryRepo,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate inventory: %w", err)
	}

	result.Allocations = allocations

	// Pass 3: Build dependency graph from gross requirements and BOM structure
	depGraph, err := s.buildDependencyGraph(
		ctx,
		allGrossRequirements,
		bomRepo,
		itemRepo,
		targetSerial,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Pass 4: Topological sort to get proper scheduling order
	sortedParts := s.topologicalSort(depGraph)

	// Pass 5: Forward schedule with dependency timing and inventory consideration
	plannedOrders, err := s.scheduleForward(sortedParts, depGraph, allocations, netRequirements)
	if err != nil {
		return nil, fmt.Errorf("failed to perform forward scheduling: %w", err)
	}
	result.PlannedOrders = plannedOrders

	// Pass 6: Identify shortages
	shortages := s.identifyShortages(netRequirements, plannedOrders)
	result.ShortageReport = shortages

	// Pass 7: Copy explosion cache to result
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
	bomTraverser := shared.NewBOMTraverser(bomRepo, itemRepo, inventoryRepo)
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

// buildDependencyGraph constructs a dependency graph from gross requirements and BOM structure
func (s *MRPService) buildDependencyGraph(
	ctx context.Context,
	grossRequirements []*entities.GrossRequirement,
	bomRepo repositories.BOMRepository,
	itemRepo repositories.ItemRepository,
	targetSerial string,
) (DependencyGraph, error) {
	depGraph := make(DependencyGraph)

	// Create nodes for all parts from gross requirements
	for _, req := range grossRequirements {
		if _, exists := depGraph[req.PartNumber]; !exists {
			item, err := itemRepo.GetItem(req.PartNumber)
			if err != nil {
				return nil, fmt.Errorf("failed to get item %s: %w", req.PartNumber, err)
			}

			depGraph[req.PartNumber] = &DependencyNode{
				PartNumber:     req.PartNumber,
				Item:           item,
				GrossQuantity:  req.Quantity,
				Level:          0, // Will be calculated later
				DirectChildren: []entities.PartNumber{},
				DirectParents:  []entities.PartNumber{},
			}
		} else {
			// Accumulate quantities if part appears multiple times
			depGraph[req.PartNumber].GrossQuantity += req.Quantity
		}
	}

	// Build parent-child relationships by querying BOM for each part
	for partNumber := range depGraph {
		alternateGroups, err := bomRepo.GetAlternateGroups(partNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to get alternate groups for %s: %w", partNumber, err)
		}

		for findNumber := range alternateGroups {
			effectiveAlternates, err := bomRepo.GetEffectiveAlternates(
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

			if len(effectiveAlternates) > 0 {
				selectedAlternate := shared.SelectBestAlternateByPriority(effectiveAlternates)
				if selectedAlternate != nil {
					// Add parent-child relationship
					depGraph[partNumber].DirectChildren = append(
						depGraph[partNumber].DirectChildren,
						selectedAlternate.ChildPN,
					)

					// Add child-parent relationship if child exists in graph
					if childNode, exists := depGraph[selectedAlternate.ChildPN]; exists {
						childNode.DirectParents = append(childNode.DirectParents, partNumber)
					}
				}
			}
		}
	}

	// Calculate BOM levels (leaf parts = highest level, root parts = 0)
	s.calculateBOMLevels(depGraph)

	return depGraph, nil
}

// calculateBOMLevels assigns BOM levels to parts in the dependency graph
func (s *MRPService) calculateBOMLevels(depGraph DependencyGraph) {
	// Find all leaf parts (no children)
	var leafParts []entities.PartNumber
	for partNumber, node := range depGraph {
		if len(node.DirectChildren) == 0 {
			leafParts = append(leafParts, partNumber)
		}
	}

	// Use BFS to assign levels starting from leaf parts
	queue := make([]entities.PartNumber, 0, len(leafParts))
	visited := make(map[entities.PartNumber]bool)

	// Initialize leaf parts at level 0
	for _, leafPart := range leafParts {
		depGraph[leafPart].Level = 0
		queue = append(queue, leafPart)
		visited[leafPart] = true
	}

	// Process queue to assign levels to parent parts
	for len(queue) > 0 {
		currentPart := queue[0]
		queue = queue[1:]
		currentLevel := depGraph[currentPart].Level

		// Process all parents of current part
		for _, parentPN := range depGraph[currentPart].DirectParents {
			if parentNode, exists := depGraph[parentPN]; exists {
				// Parent level should be at least child level + 1
				newLevel := currentLevel + 1
				if newLevel > parentNode.Level {
					parentNode.Level = newLevel
				}

				// Add parent to queue if not already visited
				if !visited[parentPN] {
					visited[parentPN] = true
					queue = append(queue, parentPN)
				}
			}
		}
	}
}

// topologicalSort returns parts in dependency order (children before parents)
func (s *MRPService) topologicalSort(depGraph DependencyGraph) []entities.PartNumber {
	// Kahn's algorithm for topological sorting
	inDegree := make(map[entities.PartNumber]int)
	queue := make([]entities.PartNumber, 0)
	result := make([]entities.PartNumber, 0, len(depGraph))

	// Calculate in-degree for each node (number of dependencies)
	for partNumber := range depGraph {
		inDegree[partNumber] = len(depGraph[partNumber].DirectChildren)
	}

	// Add all nodes with no dependencies (leaf parts) to queue
	for partNumber, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, partNumber)
		}
	}

	// Process queue
	for len(queue) > 0 {
		// Remove node with no dependencies
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// For each parent of current node, reduce its in-degree
		for _, parentPN := range depGraph[current].DirectParents {
			inDegree[parentPN]--
			if inDegree[parentPN] == 0 {
				queue = append(queue, parentPN)
			}
		}
	}

	// Check for cycles (shouldn't happen in a valid BOM)
	if len(result) != len(depGraph) {
		// Return what we have - dependency cycle detection could be added here
		return result
	}

	return result
}

// scheduleForward performs forward scheduling based on dependency graph and inventory allocation
func (s *MRPService) scheduleForward(
	sortedParts []entities.PartNumber,
	depGraph DependencyGraph,
	allocations []entities.AllocationResult,
	netRequirements []*entities.NetRequirement,
) ([]entities.PlannedOrder, error) {
	var allOrders []entities.PlannedOrder
	completionTimes := make(map[entities.PartNumber]time.Time)

	// Initialize completion times for parts with full inventory allocation
	for _, allocation := range allocations {
		if allocation.RemainingDemand == 0 {
			// Part is fully satisfied by inventory - available immediately
			completionTimes[allocation.PartNumber] = time.Now()
		}
	}

	// Create map of net requirements by part number for quick lookup
	netReqMap := make(map[entities.PartNumber]*entities.NetRequirement)
	for _, netReq := range netRequirements {
		if existing, exists := netReqMap[netReq.PartNumber]; exists {
			// Combine quantities if multiple net requirements for same part
			existing.Quantity += netReq.Quantity
		} else {
			netReqMap[netReq.PartNumber] = &entities.NetRequirement{
				PartNumber:   netReq.PartNumber,
				Quantity:     netReq.Quantity,
				NeedDate:     netReq.NeedDate,
				DemandTrace:  netReq.DemandTrace,
				Location:     netReq.Location,
				TargetSerial: netReq.TargetSerial,
			}
		}
	}

	// Schedule parts in dependency order
	for _, partNumber := range sortedParts {
		node := depGraph[partNumber]
		netReq := netReqMap[partNumber]

		// Skip parts that don't need production (fully covered by inventory)
		if netReq == nil || netReq.Quantity <= 0 {
			continue
		}

		// Calculate earliest start time based on when direct children complete
		earliestStart := s.calculateEarliestStartTime(node, completionTimes)

		// Apply lot sizing to net requirements
		orderQty := s.applyLotSizing(netReq.Quantity, node.Item)

		// Determine order type from item's make/buy code
		var orderType entities.OrderType
		switch node.Item.MakeBuyCode {
		case entities.MakeBuyMake:
			orderType = entities.Make
		case entities.MakeBuyBuy:
			orderType = entities.Buy
		default:
			orderType = entities.Make // Default fallback
		}

		// Split orders if they exceed max order quantity and schedule sequentially
		partOrders := s.splitOrderByMaxQtyForward(
			orderQty,
			node.Item,
			netReq,
			orderType,
			earliestStart,
		)
		allOrders = append(allOrders, partOrders...)

		// Record completion time for this part (when last order completes)
		if len(partOrders) > 0 {
			latestCompletion := partOrders[len(partOrders)-1].DueDate
			completionTimes[partNumber] = latestCompletion
		}
	}

	return allOrders, nil
}

// calculateEarliestStartTime determines when a part can start based on child completion times
func (s *MRPService) calculateEarliestStartTime(
	node *DependencyNode,
	completionTimes map[entities.PartNumber]time.Time,
) time.Time {
	if len(node.DirectChildren) == 0 {
		// Leaf part - can start immediately (or based on material availability)
		return time.Now()
	}

	// Find latest completion time among direct children
	var latestChildCompletion time.Time
	for _, childPN := range node.DirectChildren {
		if childCompletion, exists := completionTimes[childPN]; exists {
			if childCompletion.After(latestChildCompletion) {
				latestChildCompletion = childCompletion
			}
		}
	}

	// If no children have completion times yet, start immediately
	if latestChildCompletion.IsZero() {
		return time.Now()
	}

	return latestChildCompletion
}

// splitOrderByMaxQtyForward splits orders with forward scheduling starting from earliest start time
func (s *MRPService) splitOrderByMaxQtyForward(
	totalQty entities.Quantity,
	item *entities.Item,
	netReq *entities.NetRequirement,
	orderType entities.OrderType,
	earliestStart time.Time,
) []entities.PlannedOrder {
	var orders []entities.PlannedOrder

	// If quantity is within max limit, create single order
	if totalQty <= item.MaxOrderQty {
		dueDate := earliestStart.Add(time.Duration(item.LeadTimeDays) * 24 * time.Hour)
		order, err := entities.NewPlannedOrder(
			netReq.PartNumber,
			totalQty,
			earliestStart,
			dueDate,
			netReq.DemandTrace,
			netReq.Location,
			orderType,
			netReq.TargetSerial,
		)
		if err == nil {
			orders = append(orders, *order)
		}
		return orders
	}

	// Split into multiple sequential orders
	remainingQty := totalQty
	orderNum := 1
	currentStartDate := earliestStart

	for remainingQty > 0 {
		// Calculate quantity for this order (limited by max order qty)
		thisOrderQty := remainingQty
		if thisOrderQty > item.MaxOrderQty {
			thisOrderQty = item.MaxOrderQty
		}

		// Calculate dates for this order
		dueDate := currentStartDate.Add(time.Duration(item.LeadTimeDays) * 24 * time.Hour)

		// Create demand trace that indicates this is part of a split order
		demandTrace := netReq.DemandTrace
		if orderNum > 1 {
			demandTrace = fmt.Sprintf("%s (Split %d)", netReq.DemandTrace, orderNum)
		}

		order, err := entities.NewPlannedOrder(
			netReq.PartNumber,
			thisOrderQty,
			currentStartDate,
			dueDate,
			demandTrace,
			netReq.Location,
			orderType,
			netReq.TargetSerial,
		)
		if err == nil {
			orders = append(orders, *order)
		}

		// Next order starts when this one completes (sequential production)
		currentStartDate = dueDate
		remainingQty -= thisOrderQty
		orderNum++
	}

	return orders
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
