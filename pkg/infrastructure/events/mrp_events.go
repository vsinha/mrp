package events

import (
	"github.com/vsinha/mrp/pkg/domain/entities"
)

const (
	DemandCreatedEvent = "demand.created"
	DemandUpdatedEvent = "demand.updated"
	DemandDeletedEvent = "demand.deleted"

	InventoryReceivedEvent    = "inventory.received"
	InventoryAllocatedEvent   = "inventory.allocated"
	InventoryDeAllocatedEvent = "inventory.deallocated"

	BOMLineCreatedEvent = "bom.line.created"
	BOMLineUpdatedEvent = "bom.line.updated"
	BOMLineDeletedEvent = "bom.line.deleted"

	RequirementCalculatedEvent = "requirement.calculated"
	RequirementUpdatedEvent    = "requirement.updated"

	OrderPlannedEvent   = "order.planned"
	OrderUpdatedEvent   = "order.updated"
	OrderCancelledEvent = "order.cancelled"

	ShortageIdentifiedEvent = "shortage.identified"
	ShortageResolvedEvent   = "shortage.resolved"
)

type DemandCreated struct {
	Demand entities.DemandRequirement `json:"demand"`
}

type DemandUpdated struct {
	OldDemand entities.DemandRequirement `json:"old_demand"`
	NewDemand entities.DemandRequirement `json:"new_demand"`
}

type DemandDeleted struct {
	Demand entities.DemandRequirement `json:"demand"`
}

type InventoryReceived struct {
	InventoryLot        *entities.InventoryLot        `json:"inventory_lot,omitempty"`
	SerializedInventory *entities.SerializedInventory `json:"serialized_inventory,omitempty"`
}

type InventoryAllocated struct {
	AllocationResult entities.AllocationResult `json:"allocation_result"`
	ForDemand        entities.GrossRequirement `json:"for_demand"`
}

type InventoryDeAllocated struct {
	AllocationResult entities.AllocationResult `json:"allocation_result"`
	Reason           string                    `json:"reason"`
}

type BOMLineCreated struct {
	BOMLine entities.BOMLine `json:"bom_line"`
}

type BOMLineUpdated struct {
	OldBOMLine entities.BOMLine `json:"old_bom_line"`
	NewBOMLine entities.BOMLine `json:"new_bom_line"`
}

type BOMLineDeleted struct {
	BOMLine entities.BOMLine `json:"bom_line"`
}

type RequirementCalculated struct {
	GrossRequirement entities.GrossRequirement  `json:"gross_requirement"`
	SourceDemand     entities.DemandRequirement `json:"source_demand"`
	BOMPath          []entities.BOMLine         `json:"bom_path"`
}

type RequirementUpdated struct {
	OldRequirement entities.GrossRequirement `json:"old_requirement"`
	NewRequirement entities.GrossRequirement `json:"new_requirement"`
}

type OrderPlanned struct {
	PlannedOrder   entities.PlannedOrder   `json:"planned_order"`
	ForRequirement entities.NetRequirement `json:"for_requirement"`
}

type OrderUpdated struct {
	OldOrder entities.PlannedOrder `json:"old_order"`
	NewOrder entities.PlannedOrder `json:"new_order"`
}

type OrderCancelled struct {
	PlannedOrder entities.PlannedOrder `json:"planned_order"`
	Reason       string                `json:"reason"`
}

type ShortageIdentified struct {
	Shortage entities.Shortage `json:"shortage"`
}

type ShortageResolved struct {
	Shortage   entities.Shortage `json:"shortage"`
	ResolvedBy string            `json:"resolved_by"`
}

func NewDemandCreatedEvent(demand entities.DemandRequirement) Event {
	return NewEvent(DemandCreatedEvent, string(demand.PartNumber), DemandCreated{Demand: demand})
}

func NewInventoryReceivedEvent(inventory interface{}) Event {
	var streamID string
	var eventData InventoryReceived

	switch inv := inventory.(type) {
	case *entities.InventoryLot:
		streamID = string(inv.PartNumber)
		eventData = InventoryReceived{InventoryLot: inv}
	case *entities.SerializedInventory:
		streamID = string(inv.PartNumber)
		eventData = InventoryReceived{SerializedInventory: inv}
	}

	return NewEvent(InventoryReceivedEvent, streamID, eventData)
}

func NewBOMLineCreatedEvent(bomLine entities.BOMLine) Event {
	return NewEvent(BOMLineCreatedEvent, string(bomLine.ParentPN), BOMLineCreated{BOMLine: bomLine})
}

func NewRequirementCalculatedEvent(
	requirement entities.GrossRequirement,
	sourceDemand entities.DemandRequirement,
	bomPath []entities.BOMLine,
) Event {
	return NewEvent(
		RequirementCalculatedEvent,
		string(requirement.PartNumber),
		RequirementCalculated{
			GrossRequirement: requirement,
			SourceDemand:     sourceDemand,
			BOMPath:          bomPath,
		},
	)
}

func NewInventoryAllocatedEvent(
	allocation entities.AllocationResult,
	forDemand entities.GrossRequirement,
) Event {
	return NewEvent(InventoryAllocatedEvent, string(allocation.PartNumber), InventoryAllocated{
		AllocationResult: allocation,
		ForDemand:        forDemand,
	})
}

func NewOrderPlannedEvent(
	order entities.PlannedOrder,
	forRequirement entities.NetRequirement,
) Event {
	return NewEvent(OrderPlannedEvent, string(order.PartNumber), OrderPlanned{
		PlannedOrder:   order,
		ForRequirement: forRequirement,
	})
}

func NewShortageIdentifiedEvent(shortage entities.Shortage) Event {
	return NewEvent(
		ShortageIdentifiedEvent,
		string(shortage.PartNumber),
		ShortageIdentified{Shortage: shortage},
	)
}
