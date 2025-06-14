package mrp

import (
	"context"
	"fmt"

	"github.com/vsinha/mrp/pkg/application/dto"
	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
	"github.com/vsinha/mrp/pkg/infrastructure/events"
)

type EventDrivenMRPService struct {
	mrpService *MRPService
	eventStore events.EventStore
}

func NewEventDrivenMRPService(eventStore events.EventStore) *EventDrivenMRPService {
	return &EventDrivenMRPService{
		mrpService: NewMRPService(),
		eventStore: eventStore,
	}
}

func NewEventDrivenMRPServiceWithConfig(
	config EngineConfig,
	eventStore events.EventStore,
) *EventDrivenMRPService {
	return &EventDrivenMRPService{
		mrpService: NewMRPServiceWithConfig(config),
		eventStore: eventStore,
	}
}

func (s *EventDrivenMRPService) ExplodeDemand(
	ctx context.Context,
	demands []*entities.DemandRequirement,
	bomRepo repositories.BOMRepository,
	itemRepo repositories.ItemRepository,
	inventoryRepo repositories.InventoryRepository,
	demandRepo repositories.DemandRepository,
) (*dto.MRPResult, error) {

	// Publish demand events at start
	for _, demand := range demands {
		event := events.NewDemandCreatedEvent(*demand)
		if err := s.eventStore.AppendEvent(string(demand.PartNumber), event); err != nil {
			fmt.Printf("Warning: failed to publish demand created event: %v\n", err)
		}
	}

	// Execute the original MRP logic
	result, err := s.mrpService.ExplodeDemand(
		ctx,
		demands,
		bomRepo,
		itemRepo,
		inventoryRepo,
		demandRepo,
	)
	if err != nil {
		return nil, err
	}

	// Publish events for all results
	s.publishMRPResultEvents(result, demands)

	return result, nil
}

func (s *EventDrivenMRPService) publishMRPResultEvents(
	result *dto.MRPResult,
	originalDemands []*entities.DemandRequirement,
) {
	// Create a demand lookup for tracing requirements back to original demands
	demandMap := make(map[string]*entities.DemandRequirement)
	for _, demand := range originalDemands {
		demandMap[string(demand.PartNumber)] = demand
	}

	// Publish gross requirement events (from cache)
	for _, explosionResult := range result.ExplosionCache {
		for _, req := range explosionResult.Requirements {
			// Find source demand by parsing demand trace or part number
			sourceDemand := s.findSourceDemand(req.DemandTrace, demandMap)
			if sourceDemand != nil {
				event := events.NewRequirementCalculatedEvent(
					req,
					*sourceDemand,
					[]entities.BOMLine{},
				)
				if err := s.eventStore.AppendEvent(string(req.PartNumber), event); err != nil {
					fmt.Printf("Warning: failed to publish requirement calculated event: %v\n", err)
				}
			}
		}
	}

	// Publish inventory allocation events
	for _, allocation := range result.Allocations {
		// Create a dummy gross requirement for the allocation event
		grossReq := entities.GrossRequirement{
			PartNumber:  allocation.PartNumber,
			Quantity:    allocation.AllocatedQty + allocation.RemainingDemand,
			Location:    allocation.Location,
			DemandTrace: fmt.Sprintf("Allocation for %s", allocation.PartNumber),
		}

		event := events.NewInventoryAllocatedEvent(allocation, grossReq)
		if err := s.eventStore.AppendEvent(string(allocation.PartNumber), event); err != nil {
			fmt.Printf("Warning: failed to publish inventory allocated event: %v\n", err)
		}
	}

	// Publish planned order events
	for _, order := range result.PlannedOrders {
		// Create a dummy net requirement for the order event
		netReq := entities.NetRequirement{
			PartNumber:   order.PartNumber,
			Quantity:     order.Quantity,
			NeedDate:     order.DueDate,
			DemandTrace:  order.DemandTrace,
			Location:     order.Location,
			TargetSerial: order.TargetSerial,
		}

		event := events.NewOrderPlannedEvent(order, netReq)
		if err := s.eventStore.AppendEvent(string(order.PartNumber), event); err != nil {
			fmt.Printf("Warning: failed to publish order planned event: %v\n", err)
		}
	}

	// Publish shortage events
	for _, shortage := range result.ShortageReport {
		event := events.NewShortageIdentifiedEvent(shortage)
		if err := s.eventStore.AppendEvent(string(shortage.PartNumber), event); err != nil {
			fmt.Printf("Warning: failed to publish shortage identified event: %v\n", err)
		}
	}
}

func (s *EventDrivenMRPService) findSourceDemand(
	demandTrace string,
	demandMap map[string]*entities.DemandRequirement,
) *entities.DemandRequirement {
	// Simple heuristic: look for part numbers in demand trace
	for partNumber, demand := range demandMap {
		if len(demandTrace) > 0 && demandTrace[0:len(partNumber)] == partNumber {
			return demand
		}
	}

	// If no match found, return the first demand as fallback
	for _, demand := range demandMap {
		return demand
	}

	return nil
}
