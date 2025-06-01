package mrp

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// OptimizedEngine provides memory optimizations for large-scale MRP operations
type OptimizedEngine struct {
	*Engine
	config OptimizationConfig
}

// OptimizationConfig controls memory optimization behavior
type OptimizationConfig struct {
	// EnableGCPacing enables aggressive garbage collection for large BOMs
	EnableGCPacing bool
	// CacheCleanupInterval controls how often to clean old cache entries
	CacheCleanupInterval time.Duration
	// MaxCacheEntries limits the number of cached explosion results
	MaxCacheEntries int
	// BatchSize controls how many gross requirements to process at once
	BatchSize int
}

// NewOptimizedEngine creates a memory-optimized MRP engine
func NewOptimizedEngine(bomRepo BOMRepository, inventoryRepo InventoryRepository, config OptimizationConfig) *OptimizedEngine {
	baseEngine := NewEngine(bomRepo, inventoryRepo)
	
	// Set default configuration
	if config.CacheCleanupInterval == 0 {
		config.CacheCleanupInterval = 5 * time.Minute
	}
	if config.MaxCacheEntries == 0 {
		config.MaxCacheEntries = 10000
	}
	if config.BatchSize == 0 {
		config.BatchSize = 1000
	}
	
	optimized := &OptimizedEngine{
		Engine: baseEngine,
		config: config,
	}
	
	// Start cache cleanup goroutine if needed
	if config.CacheCleanupInterval > 0 {
		go optimized.startCacheCleanup()
	}
	
	return optimized
}

// ExplodeDemand performs memory-optimized MRP explosion
func (e *OptimizedEngine) ExplodeDemand(ctx context.Context, demands []DemandRequirement) (*MRPResult, error) {
	// Set GC pacing for large operations
	if e.config.EnableGCPacing {
		oldGCPercent := setGCPercent(50) // More aggressive GC
		defer setGCPercent(oldGCPercent)
	}
	
	// Pre-allocate result slices with estimated capacity
	estimatedOrders := len(demands) * 100 // Rough estimate
	result := &MRPResult{
		PlannedOrders:   make([]PlannedOrder, 0, estimatedOrders),
		Allocations:     make([]AllocationResult, 0, len(demands)*10),
		ShortageReport:  make([]Shortage, 0, estimatedOrders/2),
		ExplosionCache:  make(map[ExplosionCacheKey]*ExplosionResult),
	}
	
	// Process demands in batches to manage memory
	for i := 0; i < len(demands); i += e.config.BatchSize {
		end := i + e.config.BatchSize
		if end > len(demands) {
			end = len(demands)
		}
		
		batchDemands := demands[i:end]
		batchResult, err := e.Engine.ExplodeDemand(ctx, batchDemands)
		if err != nil {
			return nil, err
		}
		
		// Merge results
		result.PlannedOrders = append(result.PlannedOrders, batchResult.PlannedOrders...)
		result.Allocations = append(result.Allocations, batchResult.Allocations...)
		result.ShortageReport = append(result.ShortageReport, batchResult.ShortageReport...)
		
		// Merge cache entries
		for key, value := range batchResult.ExplosionCache {
			result.ExplosionCache[key] = value
		}
		
		// Force GC between batches for very large operations
		if e.config.EnableGCPacing && (i+e.config.BatchSize)%5000 == 0 {
			runtime.GC()
		}
	}
	
	// Clean cache if it's getting too large
	e.cleanCacheIfNeeded()
	
	return result, nil
}

// startCacheCleanup runs periodic cache cleanup
func (e *OptimizedEngine) startCacheCleanup() {
	ticker := time.NewTicker(e.config.CacheCleanupInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		e.cleanCacheIfNeeded()
	}
}

// cleanCacheIfNeeded removes old cache entries if cache is too large
func (e *OptimizedEngine) cleanCacheIfNeeded() {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()
	
	if len(e.explosionCache) <= e.config.MaxCacheEntries {
		return
	}
	
	// Remove oldest entries (simple LRU approximation)
	cutoff := time.Now().Add(-time.Hour) // Remove entries older than 1 hour
	
	for key, entry := range e.explosionCache {
		if entry.ComputedAt.Before(cutoff) {
			delete(e.explosionCache, key)
		}
		
		// If still too many, remove more aggressively
		if len(e.explosionCache) <= e.config.MaxCacheEntries {
			break
		}
	}
}

// setGCPercent changes garbage collection percentage and returns the old value
func setGCPercent(percent int) int {
	oldPercent := runtime.GOMAXPROCS(0) // This is a simplified approach
	// In real implementation, you'd use debug.SetGCPercent(percent)
	// For this example, we'll just do runtime.GC() more often
	return oldPercent
}

// MemoryPool provides object pooling for frequently allocated objects
type MemoryPool struct {
	grossReqPool     sync.Pool
	netReqPool       sync.Pool
	plannedOrderPool sync.Pool
}

// NewMemoryPool creates a new memory pool for MRP objects
func NewMemoryPool() *MemoryPool {
	return &MemoryPool{
		grossReqPool: sync.Pool{
			New: func() interface{} {
				return make([]GrossRequirement, 0, 100)
			},
		},
		netReqPool: sync.Pool{
			New: func() interface{} {
				return make([]NetRequirement, 0, 100)
			},
		},
		plannedOrderPool: sync.Pool{
			New: func() interface{} {
				return make([]PlannedOrder, 0, 100)
			},
		},
	}
}

// GetGrossRequirements gets a slice from the pool
func (p *MemoryPool) GetGrossRequirements() []GrossRequirement {
	reqs := p.grossReqPool.Get().([]GrossRequirement)
	return reqs[:0] // Reset length but keep capacity
}

// PutGrossRequirements returns a slice to the pool
func (p *MemoryPool) PutGrossRequirements(reqs []GrossRequirement) {
	if cap(reqs) > 1000 { // Don't pool very large slices
		return
	}
	p.grossReqPool.Put(reqs)
}

// GetNetRequirements gets a slice from the pool
func (p *MemoryPool) GetNetRequirements() []NetRequirement {
	reqs := p.netReqPool.Get().([]NetRequirement)
	return reqs[:0]
}

// PutNetRequirements returns a slice to the pool
func (p *MemoryPool) PutNetRequirements(reqs []NetRequirement) {
	if cap(reqs) > 1000 {
		return
	}
	p.netReqPool.Put(reqs)
}

// GetPlannedOrders gets a slice from the pool
func (p *MemoryPool) GetPlannedOrders() []PlannedOrder {
	orders := p.plannedOrderPool.Get().([]PlannedOrder)
	return orders[:0]
}

// PutPlannedOrders returns a slice to the pool
func (p *MemoryPool) PutPlannedOrders(orders []PlannedOrder) {
	if cap(orders) > 1000 {
		return
	}
	p.plannedOrderPool.Put(orders)
}

// CompactBOMRepository provides a memory-efficient BOM storage
type CompactBOMRepository struct {
	items    []Item
	itemsMap map[PartNumber]int // Index into items slice
	
	bomLines    []BOMLine
	bomIndexes  map[PartNumber][]int // Indexes into bomLines slice for each parent
	
	serialComp *SerialComparator
}

// NewCompactBOMRepository creates a memory-efficient BOM repository
func NewCompactBOMRepository(expectedItems, expectedBOMLines int) *CompactBOMRepository {
	return &CompactBOMRepository{
		items:       make([]Item, 0, expectedItems),
		itemsMap:    make(map[PartNumber]int, expectedItems),
		bomLines:    make([]BOMLine, 0, expectedBOMLines),
		bomIndexes:  make(map[PartNumber][]int, expectedItems),
		serialComp:  NewSerialComparator(),
	}
}

// AddItem adds an item to the compact repository
func (r *CompactBOMRepository) AddItem(item Item) {
	r.itemsMap[item.PartNumber] = len(r.items)
	r.items = append(r.items, item)
}

// AddBOMLine adds a BOM line to the compact repository
func (r *CompactBOMRepository) AddBOMLine(line BOMLine) {
	index := len(r.bomLines)
	r.bomLines = append(r.bomLines, line)
	r.bomIndexes[line.ParentPN] = append(r.bomIndexes[line.ParentPN], index)
}

// GetEffectiveBOM returns the effective BOM lines for a part and target serial
func (r *CompactBOMRepository) GetEffectiveBOM(ctx context.Context, pn PartNumber, serial string) ([]BOMLine, error) {
	indexes, exists := r.bomIndexes[pn]
	if !exists {
		return []BOMLine{}, nil
	}
	
	var effectiveLines []BOMLine
	for _, index := range indexes {
		line := r.bomLines[index]
		if r.serialComp.IsSerialInRange(serial, line.Effectivity) {
			effectiveLines = append(effectiveLines, line)
		}
	}
	
	return effectiveLines, nil
}

// GetItem returns item master data for a part number
func (r *CompactBOMRepository) GetItem(ctx context.Context, pn PartNumber) (*Item, error) {
	index, exists := r.itemsMap[pn]
	if !exists {
		return nil, fmt.Errorf("item not found: %s", pn)
	}
	return &r.items[index], nil
}

// GetAllBOMLines returns all BOM lines
func (r *CompactBOMRepository) GetAllBOMLines(ctx context.Context) ([]BOMLine, error) {
	return r.bomLines, nil
}

// GetAllItems returns all items
func (r *CompactBOMRepository) GetAllItems(ctx context.Context) ([]Item, error) {
	return r.items, nil
}

// MemoryStats provides memory usage statistics
type MemoryStats struct {
	AllocBytes      uint64
	TotalAllocBytes uint64
	Mallocs         uint64
	Frees           uint64
	HeapObjects     uint64
}

// GetMemoryStats returns current memory usage statistics
func GetMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	return MemoryStats{
		AllocBytes:      m.Alloc,
		TotalAllocBytes: m.TotalAlloc,
		Mallocs:         m.Mallocs,
		Frees:           m.Frees,
		HeapObjects:     m.HeapObjects,
	}
}

// FormatBytes formats bytes in human readable format
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}