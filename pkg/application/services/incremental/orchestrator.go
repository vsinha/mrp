package incremental

import (
	"fmt"

	"github.com/vsinha/mrp/pkg/domain/repositories"
	"github.com/vsinha/mrp/pkg/infrastructure/events"
)

type IncrementalMRPOrchestrator struct {
	eventStore           *events.InMemoryEventStore
	requirementProcessor *RequirementProcessor
	allocationProcessor  *AllocationProcessor
	dependencyGraph      *IncrementalDependencyGraph
	schedulingProcessor  *SchedulingProcessor
}

func NewIncrementalMRPOrchestrator(
	bomRepo repositories.BOMRepository,
	itemRepo repositories.ItemRepository,
	inventoryRepo repositories.InventoryRepository,
) *IncrementalMRPOrchestrator {
	eventStore := events.NewInMemoryEventStore()

	dependencyGraph := NewIncrementalDependencyGraph(eventStore, bomRepo, itemRepo)

	requirementProcessor := NewRequirementProcessor(bomRepo, itemRepo, inventoryRepo, eventStore)
	allocationProcessor := NewAllocationProcessor(inventoryRepo, eventStore)
	schedulingProcessor := NewSchedulingProcessor(dependencyGraph, eventStore)

	orchestrator := &IncrementalMRPOrchestrator{
		eventStore:           eventStore,
		requirementProcessor: requirementProcessor,
		allocationProcessor:  allocationProcessor,
		dependencyGraph:      dependencyGraph,
		schedulingProcessor:  schedulingProcessor,
	}

	// Subscribe processors to relevant events
	orchestrator.setupEventSubscriptions()

	return orchestrator
}

func (o *IncrementalMRPOrchestrator) setupEventSubscriptions() {
	// Requirement processor subscriptions
	reqEvents := []string{
		events.DemandCreatedEvent,
		events.DemandUpdatedEvent,
		events.BOMLineCreatedEvent,
		events.BOMLineUpdatedEvent,
	}
	if err := o.eventStore.Subscribe(reqEvents, o.requirementProcessor); err != nil {
		fmt.Printf("Warning: failed to subscribe requirement processor: %v\n", err)
	}

	// Allocation processor subscriptions
	allocEvents := []string{
		events.RequirementCalculatedEvent,
		events.InventoryReceivedEvent,
		events.InventoryDeAllocatedEvent,
	}
	if err := o.eventStore.Subscribe(allocEvents, o.allocationProcessor); err != nil {
		fmt.Printf("Warning: failed to subscribe allocation processor: %v\n", err)
	}

	// Dependency graph subscriptions
	depEvents := []string{
		events.RequirementCalculatedEvent,
		events.BOMLineCreatedEvent,
		events.BOMLineUpdatedEvent,
		events.BOMLineDeletedEvent,
	}
	if err := o.eventStore.Subscribe(depEvents, o.dependencyGraph); err != nil {
		fmt.Printf("Warning: failed to subscribe dependency graph: %v\n", err)
	}

	// Scheduling processor subscriptions
	schedEvents := []string{
		"net.requirement.created",
		events.InventoryAllocatedEvent,
		events.OrderCancelledEvent,
	}
	if err := o.eventStore.Subscribe(schedEvents, o.schedulingProcessor); err != nil {
		fmt.Printf("Warning: failed to subscribe scheduling processor: %v\n", err)
	}
}

func (o *IncrementalMRPOrchestrator) PublishEvent(streamID string, event events.Event) error {
	return o.eventStore.AppendEvent(streamID, event)
}

func (o *IncrementalMRPOrchestrator) GetEventStore() *events.InMemoryEventStore {
	return o.eventStore
}

func (o *IncrementalMRPOrchestrator) GetDependencyGraph() *IncrementalDependencyGraph {
	return o.dependencyGraph
}

func (o *IncrementalMRPOrchestrator) GetSchedulingProcessor() *SchedulingProcessor {
	return o.schedulingProcessor
}

func (o *IncrementalMRPOrchestrator) GetAllocationProcessor() *AllocationProcessor {
	return o.allocationProcessor
}
