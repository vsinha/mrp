package entities

import (
	"fmt"
	"time"
)

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
	PartNumber   PartNumber `json:"part_number"`
	Quantity     Quantity   `json:"quantity"`
	StartDate    time.Time  `json:"start_date"`
	DueDate      time.Time  `json:"due_date"`
	DemandTrace  string     `json:"demand_trace"`
	Location     string     `json:"location"`
	OrderType    OrderType  `json:"order_type"`
	TargetSerial string     `json:"target_serial"`
}

// NewPlannedOrder creates a validated PlannedOrder
func NewPlannedOrder(
	partNumber PartNumber,
	quantity Quantity,
	startDate, dueDate time.Time,
	demandTrace, location string,
	orderType OrderType,
	targetSerial string,
) (*PlannedOrder, error) {
	if string(partNumber) == "" {
		return nil, fmt.Errorf("part number cannot be empty")
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive, got %d", quantity)
	}
	if startDate.After(dueDate) {
		return nil, fmt.Errorf("start date %v cannot be after due date %v", startDate, dueDate)
	}
	if location == "" {
		return nil, fmt.Errorf("location cannot be empty")
	}

	return &PlannedOrder{
		PartNumber:   partNumber,
		Quantity:     quantity,
		StartDate:    startDate,
		DueDate:      dueDate,
		DemandTrace:  demandTrace,
		Location:     location,
		OrderType:    orderType,
		TargetSerial: targetSerial,
	}, nil
}
