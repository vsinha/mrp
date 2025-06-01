package entities

import "time"

// DemandRequirement represents external demand for a part
type DemandRequirement struct {
	PartNumber   PartNumber
	Quantity     Quantity
	NeedDate     time.Time
	DemandSource string
	Location     string
	TargetSerial string // Serial this demand is for
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

// Shortage represents unfulfilled demand
type Shortage struct {
	PartNumber   PartNumber `json:"part_number"`
	Location     string     `json:"location"`
	ShortQty     Quantity   `json:"short_qty"`
	NeedDate     time.Time  `json:"need_date"`
	DemandTrace  string     `json:"demand_trace"`
	TargetSerial string     `json:"target_serial"`
}
