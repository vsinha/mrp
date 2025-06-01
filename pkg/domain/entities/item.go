package entities

import "fmt"

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

// MakeBuyCode represents whether an item is made internally or purchased
type MakeBuyCode int

const (
	MakeBuyMake MakeBuyCode = iota
	MakeBuyBuy
)

// String method for MakeBuyCode enum
func (m MakeBuyCode) String() string {
	switch m {
	case MakeBuyMake:
		return "Make"
	case MakeBuyBuy:
		return "Buy"
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
	MaxOrderQty   Quantity
	SafetyStock   Quantity
	UnitOfMeasure string
	MakeBuyCode   MakeBuyCode
}

// NewItem creates a validated Item
func NewItem(
	partNumber PartNumber,
	description string,
	leadTimeDays int,
	lotSizeRule LotSizeRule,
	minOrderQty, maxOrderQty, safetyStock Quantity,
	unitOfMeasure string,
	makeBuyCode MakeBuyCode,
) (*Item, error) {
	// Validate inputs
	if string(partNumber) == "" {
		return nil, fmt.Errorf("part number cannot be empty")
	}
	if description == "" {
		return nil, fmt.Errorf("description cannot be empty")
	}
	if leadTimeDays <= 0 {
		return nil, fmt.Errorf("lead time must be positive, got %d", leadTimeDays)
	}
	if minOrderQty < 0 {
		return nil, fmt.Errorf("minimum order quantity cannot be negative, got %d", minOrderQty)
	}
	if maxOrderQty <= 0 {
		return nil, fmt.Errorf("maximum order quantity must be positive, got %d", maxOrderQty)
	}
	if maxOrderQty < minOrderQty {
		return nil, fmt.Errorf("maximum order quantity (%d) cannot be less than minimum order quantity (%d)", maxOrderQty, minOrderQty)
	}
	if safetyStock < 0 {
		return nil, fmt.Errorf("safety stock cannot be negative, got %d", safetyStock)
	}
	if unitOfMeasure == "" {
		return nil, fmt.Errorf("unit of measure cannot be empty")
	}

	// Business rule validation
	if (lotSizeRule == MinimumQty || lotSizeRule == StandardPack) && minOrderQty == 0 {
		return nil, fmt.Errorf(
			"lot sizing rule %s requires non-zero minimum order quantity",
			lotSizeRule,
		)
	}

	return &Item{
		PartNumber:    partNumber,
		Description:   description,
		LeadTimeDays:  leadTimeDays,
		LotSizeRule:   lotSizeRule,
		MinOrderQty:   minOrderQty,
		MaxOrderQty:   maxOrderQty,
		SafetyStock:   safetyStock,
		UnitOfMeasure: unitOfMeasure,
		MakeBuyCode:   makeBuyCode,
	}, nil
}
