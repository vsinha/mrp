package entities

import (
	"fmt"
	"time"
)

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

// NewInventoryLot creates a validated InventoryLot
func NewInventoryLot(partNumber PartNumber, lotNumber, location string, quantity Quantity, receiptDate time.Time, status InventoryStatus) (*InventoryLot, error) {
	if string(partNumber) == "" {
		return nil, fmt.Errorf("part number cannot be empty")
	}
	if lotNumber == "" {
		return nil, fmt.Errorf("lot number cannot be empty")
	}
	if location == "" {
		return nil, fmt.Errorf("location cannot be empty")
	}
	if quantity < 0 {
		return nil, fmt.Errorf("quantity cannot be negative, got %d", quantity)
	}

	return &InventoryLot{
		PartNumber:  partNumber,
		LotNumber:   lotNumber,
		Location:    location,
		Quantity:    quantity,
		ReceiptDate: receiptDate,
		Status:      status,
	}, nil
}

// SerializedInventory represents serialized inventory items
type SerializedInventory struct {
	PartNumber   PartNumber
	SerialNumber string
	Location     string
	Status       InventoryStatus
	ReceiptDate  time.Time
}

// NewSerializedInventory creates a validated SerializedInventory
func NewSerializedInventory(partNumber PartNumber, serialNumber, location string, status InventoryStatus, receiptDate time.Time) (*SerializedInventory, error) {
	if string(partNumber) == "" {
		return nil, fmt.Errorf("part number cannot be empty")
	}
	if serialNumber == "" {
		return nil, fmt.Errorf("serial number cannot be empty")
	}
	if location == "" {
		return nil, fmt.Errorf("location cannot be empty")
	}

	return &SerializedInventory{
		PartNumber:   partNumber,
		SerialNumber: serialNumber,
		Location:     location,
		Status:       status,
		ReceiptDate:  receiptDate,
	}, nil
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
