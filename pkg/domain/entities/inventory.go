package entities

import "time"

// InventoryStatus represents the status of inventory
type InventoryStatus int

const (
	Available InventoryStatus = iota
	Allocated
	Quarantine
)

// String method for InventoryStatus enum
func (s InventoryStatus) String() string {
	switch s {
	case Available:
		return "Available"
	case Allocated:
		return "Allocated"
	case Quarantine:
		return "Quarantine"
	default:
		return "Unknown"
	}
}

// InventoryLot represents lot-controlled inventory
type InventoryLot struct {
	PartNumber  PartNumber
	LotNumber   string
	Location    string
	Quantity    Quantity
	ReceiptDate time.Time
	Status      InventoryStatus
}

// SerializedInventory represents serialized inventory items
type SerializedInventory struct {
	PartNumber   PartNumber
	SerialNumber string
	Location     string
	Status       InventoryStatus
	ReceiptDate  time.Time
}

// InventoryAllocation represents a specific allocation from inventory
type InventoryAllocation struct {
	LotNumber    string
	SerialNumber string
	Quantity     Quantity
	Location     string
}

// AllocationResult represents the result of inventory allocation
type AllocationResult struct {
	PartNumber      PartNumber
	Location        string
	AllocatedQty    Quantity
	RemainingDemand Quantity
	AllocatedFrom   []InventoryAllocation
}
