package repositories

import "github.com/vsinha/mrp/pkg/domain/entities"

// BOMRepository provides access to Bill of Materials data
type BOMRepository interface {
	GetBOMLines(partNumber entities.PartNumber) ([]*entities.BOMLine, error)
	GetEffectiveLines(partNumber entities.PartNumber, serial string) ([]*entities.BOMLine, error)
	GetAllBOMLines() ([]*entities.BOMLine, error)
	LoadBOMLines(lines []*entities.BOMLine) error
}
