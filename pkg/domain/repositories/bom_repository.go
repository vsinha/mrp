package repositories

import "github.com/vsinha/mrp/pkg/domain/entities"

// BOMRepository provides access to Bill of Materials data
type BOMRepository interface {
	GetBOMLines(partNumber entities.PartNumber) ([]*entities.BOMLine, error)
	GetEffectiveLines(partNumber entities.PartNumber, serial string) ([]*entities.BOMLine, error)
	GetAllBOMLines() ([]*entities.BOMLine, error)
	LoadBOMLines(lines []*entities.BOMLine) error
	
	// Alternate-aware methods
	
	// GetAlternateGroups returns BOM lines grouped by FindNumber for a parent part.
	// Returns map[FindNumber][]*BOMLine where each slice contains alternates for that position.
	GetAlternateGroups(parentPN entities.PartNumber) (map[int][]*entities.BOMLine, error)
	
	// GetEffectiveAlternates returns alternate BOM lines for a specific FindNumber and serial.
	// Filters by serial effectivity and groups alternates together.
	GetEffectiveAlternates(parentPN entities.PartNumber, findNumber int, targetSerial string) ([]*entities.BOMLine, error)
}
