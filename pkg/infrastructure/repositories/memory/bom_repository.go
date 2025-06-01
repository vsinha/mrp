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
	bomLines   []entities.BOMLine
	bomIndexes map[entities.PartNumber][]int
	serialComp *services.SerialComparator
}

// NewBOMRepository creates a memory-efficient BOM repository
func NewBOMRepository(expectedBOMLines int) *BOMRepository {
	return &BOMRepository{
		bomLines:   make([]entities.BOMLine, 0, expectedBOMLines),
		bomIndexes: make(map[entities.PartNumber][]int, expectedBOMLines/10),
		serialComp: services.NewSerialComparator(),
	}
}

// Verify interface compliance
var _ repositories.BOMRepository = (*BOMRepository)(nil)

// LoadBOMLines loads BOM lines into the repository
func (r *BOMRepository) LoadBOMLines(lines []*entities.BOMLine) error {
	for _, line := range lines {
		r.AddBOMLine(*line)
	}
	return nil
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
func (r *BOMRepository) GetEffectiveLines(
	partNumber entities.PartNumber,
	serial string,
) ([]*entities.BOMLine, error) {
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

// GetAlternateGroups returns BOM lines grouped by FindNumber for a parent part
func (r *BOMRepository) GetAlternateGroups(
	parentPN entities.PartNumber,
) (map[int][]*entities.BOMLine, error) {
	indexes, exists := r.bomIndexes[parentPN]
	if !exists {
		return make(map[int][]*entities.BOMLine), nil
	}

	groups := make(map[int][]*entities.BOMLine)
	for _, index := range indexes {
		line := r.bomLines[index]
		findNumber := line.FindNumber
		groups[findNumber] = append(groups[findNumber], &line)
	}

	return groups, nil
}

// GetEffectiveAlternates returns alternate BOM lines for a specific FindNumber and serial
func (r *BOMRepository) GetEffectiveAlternates(
	parentPN entities.PartNumber,
	findNumber int,
	targetSerial string,
) ([]*entities.BOMLine, error) {
	indexes, exists := r.bomIndexes[parentPN]
	if !exists {
		return []*entities.BOMLine{}, nil
	}

	var alternates []*entities.BOMLine
	for _, index := range indexes {
		line := r.bomLines[index]
		// Filter by FindNumber and serial effectivity
		if line.FindNumber == findNumber &&
			r.serialComp.IsSerialInRange(targetSerial, line.Effectivity) {
			alternates = append(alternates, &line)
		}
	}

	return alternates, nil
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
