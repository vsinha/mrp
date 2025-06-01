package entities

import "time"

// OrderType represents the type of planned order
type OrderType int

const (
	Make OrderType = iota
	Buy
	Transfer
)

// String method for OrderType enum
func (o OrderType) String() string {
	switch o {
	case Make:
		return "Make"
	case Buy:
		return "Buy"
	case Transfer:
		return "Transfer"
	default:
		return "Unknown"
	}
}

// PlannedOrder represents a planned manufacturing or procurement order
type PlannedOrder struct {
	PartNumber   PartNumber
	Quantity     Quantity
	StartDate    time.Time
	DueDate      time.Time
	DemandTrace  string
	Location     string
	OrderType    OrderType
	TargetSerial string
}
