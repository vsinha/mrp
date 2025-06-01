package mrp

import (
	"context"
	"fmt"
	"runtime"
)

// BOMRepository provides a memory-efficient BOM storage implementation
type BOMRepository struct {
	items       []Item
	itemsMap    map[PartNumber]int
	bomLines    []BOMLine
	bomIndexes  map[PartNumber][]int
	serialComp  *SerialComparator
}

// NewBOMRepository creates a memory-efficient BOM repository
func NewBOMRepository(expectedItems, expectedBOMLines int) *BOMRepository {
	return &BOMRepository{
		items:       make([]Item, 0, expectedItems),
		itemsMap:    make(map[PartNumber]int, expectedItems),
		bomLines:    make([]BOMLine, 0, expectedBOMLines),
		bomIndexes:  make(map[PartNumber][]int, expectedItems),
		serialComp:  NewSerialComparator(),
	}
}

// AddItem adds an item to the repository
func (r *BOMRepository) AddItem(item Item) {
	r.itemsMap[item.PartNumber] = len(r.items)
	r.items = append(r.items, item)
}

// AddBOMLine adds a BOM line to the repository
func (r *BOMRepository) AddBOMLine(line BOMLine) {
	index := len(r.bomLines)
	r.bomLines = append(r.bomLines, line)
	r.bomIndexes[line.ParentPN] = append(r.bomIndexes[line.ParentPN], index)
}

// GetEffectiveBOM returns the effective BOM lines for a part and target serial
func (r *BOMRepository) GetEffectiveBOM(ctx context.Context, pn PartNumber, serial string) ([]BOMLine, error) {
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
func (r *BOMRepository) GetItem(ctx context.Context, pn PartNumber) (*Item, error) {
	index, exists := r.itemsMap[pn]
	if !exists {
		return nil, fmt.Errorf("item not found: %s", pn)
	}
	return &r.items[index], nil
}

// GetAllBOMLines returns all BOM lines
func (r *BOMRepository) GetAllBOMLines(ctx context.Context) ([]BOMLine, error) {
	return r.bomLines, nil
}

// GetAllItems returns all items
func (r *BOMRepository) GetAllItems(ctx context.Context) ([]Item, error) {
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