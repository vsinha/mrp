package incremental

import (
	"fmt"
	"sync"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
	"github.com/vsinha/mrp/pkg/infrastructure/events"
)

type AllocationProcessor struct {
	inventoryRepo repositories.InventoryRepository
	eventStore    events.EventStore

	// Track current allocations for incremental updates
	currentAllocations map[string]*entities.AllocationResult
	allocationMutex    sync.RWMutex
}

func NewAllocationProcessor(
	inventoryRepo repositories.InventoryRepository,
	eventStore events.EventStore,
) *AllocationProcessor {
	return &AllocationProcessor{
		inventoryRepo:      inventoryRepo,
		eventStore:         eventStore,
		currentAllocations: make(map[string]*entities.AllocationResult),
	}
}

func (p *AllocationProcessor) Handle(event events.Event) error {
	switch event.Type() {
	case events.RequirementCalculatedEvent:
		return p.handleRequirementCalculated(event)
	case events.InventoryReceivedEvent:
		return p.handleInventoryReceived(event)
	case events.InventoryDeAllocatedEvent:
		return p.handleInventoryDeAllocated(event)
	default:
		return nil // Ignore events we don't handle
	}
}

func (p *AllocationProcessor) CanHandle(eventType string) bool {
	switch eventType {
	case events.RequirementCalculatedEvent,
		events.InventoryReceivedEvent,
		events.InventoryDeAllocatedEvent:
		return true
	default:
		return false
	}
}

func (p *AllocationProcessor) handleRequirementCalculated(event events.Event) error {
	reqData, ok := event.Data().(events.RequirementCalculated)
	if !ok {
		return fmt.Errorf("invalid event data for requirement calculated")
	}

	grossReq := reqData.GrossRequirement

	// Try to allocate inventory for this specific requirement
	allocation, err := p.inventoryRepo.AllocateInventory(
		grossReq.PartNumber,
		grossReq.Location,
		grossReq.Quantity,
	)
	if err != nil {
		return fmt.Errorf("failed to allocate inventory for %s: %w", grossReq.PartNumber, err)
	}

	// Track the allocation
	allocationKey := fmt.Sprintf("%s|%s", grossReq.PartNumber, grossReq.Location)
	p.allocationMutex.Lock()
	p.currentAllocations[allocationKey] = allocation
	p.allocationMutex.Unlock()

	// Publish allocation event
	allocationEvent := events.NewInventoryAllocatedEvent(*allocation, grossReq)
	if err := p.eventStore.AppendEvent(string(grossReq.PartNumber), allocationEvent); err != nil {
		fmt.Printf("Warning: failed to publish inventory allocated event: %v\n", err)
	}

	// If there's remaining demand, it becomes a net requirement
	if allocation.RemainingDemand > 0 {
		netReq := entities.NetRequirement{
			PartNumber:   grossReq.PartNumber,
			Quantity:     allocation.RemainingDemand,
			NeedDate:     grossReq.NeedDate,
			DemandTrace:  grossReq.DemandTrace,
			Location:     grossReq.Location,
			TargetSerial: grossReq.TargetSerial,
		}

		// Publish net requirement as an order planning event
		orderEvent := events.NewEvent(
			"net.requirement.created",
			string(netReq.PartNumber),
			map[string]interface{}{
				"net_requirement": netReq,
			},
		)
		if err := p.eventStore.AppendEvent(string(netReq.PartNumber), orderEvent); err != nil {
			fmt.Printf("Warning: failed to publish net requirement event: %v\n", err)
		}
	}

	return nil
}

func (p *AllocationProcessor) handleInventoryReceived(event events.Event) error {
	invData, ok := event.Data().(events.InventoryReceived)
	if !ok {
		return fmt.Errorf("invalid event data for inventory received")
	}

	var partNumber entities.PartNumber
	var location string

	// Extract part number and location from inventory data
	if invData.InventoryLot != nil {
		partNumber = invData.InventoryLot.PartNumber
		location = invData.InventoryLot.Location
	} else if invData.SerializedInventory != nil {
		partNumber = invData.SerializedInventory.PartNumber
		location = invData.SerializedInventory.Location
	} else {
		return fmt.Errorf("no inventory data found in event")
	}

	// Check if there are pending allocations that could benefit from this new inventory
	allocationKey := fmt.Sprintf("%s|%s", partNumber, location)
	p.allocationMutex.RLock()
	existingAllocation, exists := p.currentAllocations[allocationKey]
	p.allocationMutex.RUnlock()

	if exists && existingAllocation.RemainingDemand > 0 {
		// Try to allocate additional inventory against remaining demand
		additionalAllocation, err := p.inventoryRepo.AllocateInventory(
			partNumber,
			location,
			existingAllocation.RemainingDemand,
		)
		if err != nil {
			fmt.Printf("Warning: failed to allocate additional inventory: %v\n", err)
			return nil // Don't fail the entire event processing
		}

		if additionalAllocation.AllocatedQty > 0 {
			// Update tracked allocation
			p.allocationMutex.Lock()
			existingAllocation.AllocatedQty += additionalAllocation.AllocatedQty
			existingAllocation.RemainingDemand = additionalAllocation.RemainingDemand
			existingAllocation.AllocatedFrom = append(
				existingAllocation.AllocatedFrom,
				additionalAllocation.AllocatedFrom...)
			p.allocationMutex.Unlock()

			// Publish updated allocation event
			grossReq := entities.GrossRequirement{
				PartNumber: partNumber,
				Location:   location,
				Quantity:   additionalAllocation.AllocatedQty,
			}

			allocationEvent := events.NewInventoryAllocatedEvent(*additionalAllocation, grossReq)
			if err := p.eventStore.AppendEvent(string(partNumber), allocationEvent); err != nil {
				fmt.Printf("Warning: failed to publish additional allocation event: %v\n", err)
			}
		}
	}

	return nil
}

func (p *AllocationProcessor) handleInventoryDeAllocated(event events.Event) error {
	deAllocData, ok := event.Data().(events.InventoryDeAllocated)
	if !ok {
		return fmt.Errorf("invalid event data for inventory deallocated")
	}

	allocation := deAllocData.AllocationResult
	allocationKey := fmt.Sprintf("%s|%s", allocation.PartNumber, allocation.Location)

	// Update tracked allocation
	p.allocationMutex.Lock()
	if existing, exists := p.currentAllocations[allocationKey]; exists {
		existing.AllocatedQty -= allocation.AllocatedQty
		existing.RemainingDemand += allocation.AllocatedQty

		// Remove deallocated items from allocation list
		// This is simplified - in practice would need more sophisticated tracking
		if existing.AllocatedQty <= 0 {
			delete(p.currentAllocations, allocationKey)
		}
	}
	p.allocationMutex.Unlock()

	return nil
}

func (p *AllocationProcessor) GetCurrentAllocations() map[string]*entities.AllocationResult {
	p.allocationMutex.RLock()
	defer p.allocationMutex.RUnlock()

	// Return a copy to avoid concurrent access issues
	result := make(map[string]*entities.AllocationResult)
	for key, allocation := range p.currentAllocations {
		result[key] = allocation
	}
	return result
}
