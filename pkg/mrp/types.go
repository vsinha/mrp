package mrp

import (
	"time"
)

// PartNumber represents a unique part identifier
type PartNumber string

// Quantity represents an integer quantity value for discrete manufacturing units
type Quantity int64

// Item represents a manufacturing item with its properties
type Item struct {
	PartNumber      PartNumber
	Description     string
	LeadTimeDays    int
	LotSizeRule     LotSizeRule
	MinOrderQty     Quantity
	SafetyStock     Quantity
	UnitOfMeasure   string
}

// SerialEffectivity defines the range of serials for which a BOM line is effective
type SerialEffectivity struct {
	FromSerial string
	ToSerial   string // empty = open ended
}

// BOMLine represents a single line in a Bill of Materials
type BOMLine struct {
	ParentPN     PartNumber
	ChildPN      PartNumber
	QtyPer       Quantity
	FindNumber   int
	Effectivity  SerialEffectivity
}

// DemandRequirement represents external demand for a part
type DemandRequirement struct {
	PartNumber   PartNumber
	Quantity     Quantity
	NeedDate     time.Time
	DemandSource string
	Location     string
	TargetSerial string // Serial this demand is for
}

// InventoryLot represents lot-controlled inventory
type InventoryLot struct {
	PartNumber   PartNumber
	LotNumber    string
	Location     string
	Quantity     Quantity
	ReceiptDate  time.Time
	Status       InventoryStatus
}

// SerializedInventory represents serialized inventory items
type SerializedInventory struct {
	PartNumber   PartNumber
	SerialNumber string
	Location     string
	Status       InventoryStatus
	ReceiptDate  time.Time
}

// GrossRequirement represents calculated gross requirements before inventory allocation
type GrossRequirement struct {
	PartNumber   PartNumber
	Quantity     Quantity
	NeedDate     time.Time
	DemandTrace  string
	Location     string
	TargetSerial string
}

// NetRequirement represents net requirements after inventory allocation
type NetRequirement struct {
	PartNumber   PartNumber
	Quantity     Quantity
	NeedDate     time.Time
	DemandTrace  string
	Location     string
	TargetSerial string
}

// AllocationResult represents the result of inventory allocation
type AllocationResult struct {
	PartNumber      PartNumber
	Location        string
	AllocatedQty    Quantity
	RemainingDemand Quantity
	AllocatedFrom   []InventoryAllocation
}

// InventoryAllocation represents a specific allocation from inventory
type InventoryAllocation struct {
	LotNumber    string
	SerialNumber string
	Quantity     Quantity
	Location     string
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

// MRPResult contains the complete output of an MRP run
type MRPResult struct {
	PlannedOrders   []PlannedOrder
	Allocations     []AllocationResult
	ShortageReport  []Shortage
	ExplosionCache  map[ExplosionCacheKey]*ExplosionResult
}

// ExplosionCacheKey is used for memoizing BOM explosion results
type ExplosionCacheKey struct {
	PartNumber        PartNumber
	SerialEffectivity SerialEffectivity
}

// ExplosionResult contains cached results of BOM explosion
type ExplosionResult struct {
	Requirements []GrossRequirement
	LeadTimeDays int
	ComputedAt   time.Time
}

// Shortage represents unfulfilled demand
type Shortage struct {
	PartNumber   PartNumber
	Location     string
	ShortQty     Quantity
	NeedDate     time.Time
	DemandTrace  string
	TargetSerial string
}

// InventoryStatus represents the status of inventory
type InventoryStatus int

const (
	Available InventoryStatus = iota
	Allocated
	Quarantine
)

// OrderType represents the type of planned order
type OrderType int

const (
	Make OrderType = iota
	Buy
	Transfer
)

// LotSizeRule represents the lot sizing rule for an item
type LotSizeRule int

const (
	LotForLot LotSizeRule = iota
	MinimumQty
	StandardPack
)

// String methods for enums
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