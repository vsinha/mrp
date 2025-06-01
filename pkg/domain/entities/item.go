package entities

// PartNumber represents a unique part identifier
type PartNumber string

// Quantity represents an integer quantity value for discrete manufacturing units
type Quantity int64

// LotSizeRule represents the lot sizing rule for an item
type LotSizeRule int

const (
	LotForLot LotSizeRule = iota
	MinimumQty
	StandardPack
)

// String method for LotSizeRule enum
func (l LotSizeRule) String() string {
	switch l {
	case LotForLot:
		return "LotForLot"
	case MinimumQty:
		return "MinimumQty"
	case StandardPack:
		return "StandardPack"
	default:
		return "Unknown"
	}
}

// Item represents a manufacturing item with its properties
type Item struct {
	PartNumber    PartNumber
	Description   string
	LeadTimeDays  int
	LotSizeRule   LotSizeRule
	MinOrderQty   Quantity
	SafetyStock   Quantity
	UnitOfMeasure string
}
