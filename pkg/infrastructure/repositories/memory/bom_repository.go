package memory

import (
	"fmt"
	"runtime"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
	"github.com/vsinha/mrp/pkg/domain/services"
)

// BOMRepository provides a memory-efficient BOM storage implementation
type BOMRepository struct {
	items      []entities.Item
	itemsMap   map[entities.PartNumber]int
	bomLines   []entities.BOMLine
	bomIndexes map[entities.PartNumber][]int
	serialComp *services.SerialComparator
}

// NewBOMRepository creates a memory-efficient BOM repository
func NewBOMRepository(expectedItems, expectedBOMLines int) *BOMRepository {
	return &BOMRepository{
		items:      make([]entities.Item, 0, expectedItems),
		itemsMap:   make(map[entities.PartNumber]int, expectedItems),
		bomLines:   make([]entities.BOMLine, 0, expectedBOMLines),
		bomIndexes: make(map[entities.PartNumber][]int, expectedItems),
		serialComp: services.NewSerialComparator(),
	}
}

// Verify interface compliance
var _ repositories.BOMRepository = (*BOMRepository)(nil)
var _ repositories.ItemRepository = (*BOMRepository)(nil)

// LoadBOMLines loads BOM lines into the repository
func (r *BOMRepository) LoadBOMLines(lines []*entities.BOMLine) error {
	for _, line := range lines {
		r.AddBOMLine(*line)
	}
	return nil
}

// LoadItems loads items into the repository (ItemRepository interface)
func (r *BOMRepository) LoadItems(items []*entities.Item) error {
	for _, item := range items {
		r.AddItem(*item)
	}
	return nil
}

// AddItem adds an item to the repository
func (r *BOMRepository) AddItem(item entities.Item) {
	r.itemsMap[item.PartNumber] = len(r.items)
	r.items = append(r.items, item)
}

// GetItem returns item master data for a part number (ItemRepository interface)
func (r *BOMRepository) GetItem(partNumber entities.PartNumber) (*entities.Item, error) {
	index, exists := r.itemsMap[partNumber]
	if !exists {
		return nil, fmt.Errorf("item not found: %s", partNumber)
	}
	return &r.items[index], nil
}

// GetAllItems returns all items (ItemRepository interface)
func (r *BOMRepository) GetAllItems() ([]*entities.Item, error) {
	var items []*entities.Item
	for i := range r.items {
		items = append(items, &r.items[i])
	}
	return items, nil
}

// AddBOMLine adds a BOM line to the repository
func (r *BOMRepository) AddBOMLine(line entities.BOMLine) {
	index := len(r.bomLines)
	r.bomLines = append(r.bomLines, line)
	r.bomIndexes[line.ParentPN] = append(r.bomIndexes[line.ParentPN], index)
}

// GetBOMLines returns all BOM lines for a part number
func (r *BOMRepository) GetBOMLines(partNumber entities.PartNumber) ([]*entities.BOMLine, error) {
	indexes, exists := r.bomIndexes[partNumber]
	if !exists {
		return []*entities.BOMLine{}, nil
	}

	var lines []*entities.BOMLine
	for _, index := range indexes {
		line := r.bomLines[index]
		lines = append(lines, &line)
	}

	return lines, nil
}

// GetEffectiveLines returns the effective BOM lines for a part and target serial
func (r *BOMRepository) GetEffectiveLines(partNumber entities.PartNumber, serial string) ([]*entities.BOMLine, error) {
	indexes, exists := r.bomIndexes[partNumber]
	if !exists {
		return []*entities.BOMLine{}, nil
	}

	var effectiveLines []*entities.BOMLine
	for _, index := range indexes {
		line := r.bomLines[index]
		if r.serialComp.IsSerialInRange(serial, line.Effectivity) {
			effectiveLines = append(effectiveLines, &line)
		}
	}

	return effectiveLines, nil
}

// GetAllBOMLines returns all BOM lines
func (r *BOMRepository) GetAllBOMLines() ([]*entities.BOMLine, error) {
	var lines []*entities.BOMLine
	for i := range r.bomLines {
		lines = append(lines, &r.bomLines[i])
	}
	return lines, nil
}

// SaveBOMLine saves a BOM line to the repository
func (r *BOMRepository) SaveBOMLine(line *entities.BOMLine) error {
	r.AddBOMLine(*line)
	return nil
}

// SaveItem saves an item to the repository (ItemRepository interface)
func (r *BOMRepository) SaveItem(item *entities.Item) error {
	r.AddItem(*item)
	return nil
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
