package incremental

import (
	"testing"
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/infrastructure/events"
)

func TestIncrementalMRP_DetailedEventFlow(t *testing.T) {
	bomRepo, itemRepo, inventoryRepo := setupTestRepositories(t)
	orchestrator := NewIncrementalMRPOrchestrator(bomRepo, itemRepo, inventoryRepo)

	// Give async processors time to subscribe
	time.Sleep(20 * time.Millisecond)

	// Step 1: Add first demand
	demand1 := entities.DemandRequirement{
		PartNumber:   "ENGINE_ASSEMBLY",
		Quantity:     entities.Quantity(1),
		NeedDate:     time.Now().Add(30 * 24 * time.Hour),
		DemandSource: "ORDER_1",
		Location:     "FACTORY",
		TargetSerial: "SN001",
	}

	t.Logf("Publishing first demand for %s qty %d", demand1.PartNumber, demand1.Quantity)

	demandEvent1 := events.NewDemandCreatedEvent(demand1)
	err := orchestrator.PublishEvent(string(demand1.PartNumber), demandEvent1)
	if err != nil {
		t.Fatalf("Failed to publish first demand: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Check events after first demand
	eventStore := orchestrator.GetEventStore()
	events1, _ := eventStore.ReadAllEvents(0)
	t.Logf("After first demand: %d events", len(events1))

	eventTypes1 := countEventTypes(events1)
	for eventType, count := range eventTypes1 {
		t.Logf("  %s: %d", eventType, count)
	}

	// Verify we have the expected events
	if eventTypes1[events.DemandCreatedEvent] != 1 {
		t.Errorf("Expected 1 demand created event, got %d", eventTypes1[events.DemandCreatedEvent])
	}

	if eventTypes1[events.RequirementCalculatedEvent] == 0 {
		t.Error("Expected requirement calculated events after demand processing")
	}

	// Step 2: Add second demand (should be incremental)
	demand2 := entities.DemandRequirement{
		PartNumber:   "ENGINE_ASSEMBLY",
		Quantity:     entities.Quantity(2),
		NeedDate:     time.Now().Add(45 * 24 * time.Hour),
		DemandSource: "ORDER_2",
		Location:     "FACTORY",
		TargetSerial: "SN001",
	}

	t.Logf("Publishing second demand for %s qty %d", demand2.PartNumber, demand2.Quantity)

	demandEvent2 := events.NewDemandCreatedEvent(demand2)
	err = orchestrator.PublishEvent(string(demand2.PartNumber), demandEvent2)
	if err != nil {
		t.Fatalf("Failed to publish second demand: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Check events after second demand
	events2, _ := eventStore.ReadAllEvents(0)
	t.Logf("After second demand: %d events (was %d)", len(events2), len(events1))

	eventTypes2 := countEventTypes(events2)
	for eventType, count := range eventTypes2 {
		t.Logf("  %s: %d", eventType, count)
	}

	// Should now have 2 demand created events
	if eventTypes2[events.DemandCreatedEvent] != 2 {
		t.Errorf("Expected 2 demand created events, got %d", eventTypes2[events.DemandCreatedEvent])
	}

	// Should have more requirement events (incremental processing)
	if eventTypes2[events.RequirementCalculatedEvent] <= eventTypes1[events.RequirementCalculatedEvent] {
		t.Errorf("Expected more requirement events after second demand: %d vs %d initially",
			eventTypes2[events.RequirementCalculatedEvent], eventTypes1[events.RequirementCalculatedEvent])
	}

	// Step 3: Check dependency graph state
	depGraph := orchestrator.GetDependencyGraph()
	engineInfo := depGraph.GetNodeInfo("ENGINE_ASSEMBLY")

	if engineInfo == nil {
		t.Fatal("ENGINE_ASSEMBLY should be in dependency graph")
	}

	t.Logf("ENGINE_ASSEMBLY in dependency graph:")
	t.Logf("  Level: %d", engineInfo.Level)
	t.Logf("  Requirements: %d", len(engineInfo.Requirements))
	t.Logf("  Children: %d", len(engineInfo.DirectChildren))
	t.Logf("  Parents: %d", len(engineInfo.DirectParents))

	// Should have requirements from both demands
	if len(engineInfo.Requirements) == 0 {
		t.Error("Expected ENGINE_ASSEMBLY to have requirements")
	}

	// Check children
	expectedChildren := []string{"TURBOPUMP", "NOZZLE"}
	for _, expectedChild := range expectedChildren {
		if !engineInfo.DirectChildren[entities.PartNumber(expectedChild)] {
			t.Errorf("Expected ENGINE_ASSEMBLY to have child %s", expectedChild)
		}
	}

	// Step 4: Add inventory incrementally and verify reprocessing
	newInventory, err := entities.NewInventoryLot(
		"TURBOPUMP",
		"LOT_TURBO_NEW",
		"FACTORY",
		entities.Quantity(10),
		time.Now(),
		entities.Available,
	)
	if err != nil {
		t.Fatalf("Failed to create inventory: %v", err)
	}

	t.Logf("Adding inventory for %s qty %d", newInventory.PartNumber, newInventory.Quantity)

	inventoryEvent := events.NewInventoryReceivedEvent(newInventory)
	err = orchestrator.PublishEvent(string(newInventory.PartNumber), inventoryEvent)
	if err != nil {
		t.Fatalf("Failed to publish inventory: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Check final events
	events3, _ := eventStore.ReadAllEvents(0)
	t.Logf("After inventory addition: %d events (was %d)", len(events3), len(events2))

	eventTypes3 := countEventTypes(events3)
	for eventType, count := range eventTypes3 {
		t.Logf("  %s: %d", eventType, count)
	}

	// Should have inventory received event
	if eventTypes3[events.InventoryReceivedEvent] == 0 {
		t.Error("Expected inventory received event")
	}

	// Should have new processing events
	if len(events3) <= len(events2) {
		t.Errorf("Expected more events after inventory addition: %d vs %d", len(events3), len(events2))
	}
}

func countEventTypes(events []events.Event) map[string]int {
	counts := make(map[string]int)
	for _, event := range events {
		counts[event.Type()]++
	}
	return counts
}

func TestIncrementalMRP_VerifyAsyncProcessing(t *testing.T) {
	bomRepo, itemRepo, inventoryRepo := setupTestRepositories(t)
	orchestrator := NewIncrementalMRPOrchestrator(bomRepo, itemRepo, inventoryRepo)

	// Wait for subscription setup
	time.Sleep(20 * time.Millisecond)

	// Check that processors are properly subscribed
	eventStore := orchestrator.GetEventStore()

	// Publish a test event and verify it gets processed
	demand := entities.DemandRequirement{
		PartNumber:   "ENGINE_ASSEMBLY",
		Quantity:     entities.Quantity(1),
		NeedDate:     time.Now().Add(30 * 24 * time.Hour),
		DemandSource: "ASYNC_TEST",
		Location:     "FACTORY",
		TargetSerial: "SN001",
	}

	// Record event count before
	eventsBefore, _ := eventStore.ReadAllEvents(0)
	beforeCount := len(eventsBefore)

	// Publish event
	demandEvent := events.NewDemandCreatedEvent(demand)
	err := orchestrator.PublishEvent(string(demand.PartNumber), demandEvent)
	if err != nil {
		t.Fatalf("Failed to publish demand: %v", err)
	}

	// Should process quickly but asynchronously
	time.Sleep(50 * time.Millisecond)

	eventsAfter, _ := eventStore.ReadAllEvents(0)
	afterCount := len(eventsAfter)

	if afterCount <= beforeCount {
		t.Errorf("Expected more events after async processing: %d vs %d before", afterCount, beforeCount)

		// Debug: show what events we got
		for i, event := range eventsAfter {
			t.Logf("Event %d: %s -> %s", i, event.Type(), event.StreamID())
		}
	}

	// Verify that we got the expected types of events
	eventTypes := countEventTypes(eventsAfter)

	if eventTypes[events.DemandCreatedEvent] == 0 {
		t.Error("Expected demand created event")
	}

	if eventTypes[events.RequirementCalculatedEvent] == 0 {
		t.Error("Expected requirement calculated events from async processing")
	}
}
