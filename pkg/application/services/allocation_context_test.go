package services

import (
	"context"
	"testing"
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
	testinghelpers "github.com/vsinha/mrp/pkg/infrastructure/testing"
)

func TestPlanningOrchestrator_AnalyzeCriticalPathWithMRPResults(t *testing.T) {
	bomRepo, itemRepo, inventoryRepo, _ := testinghelpers.BuildAerospaceTestData()
	
	mrpService := NewMRPService(bomRepo, itemRepo, inventoryRepo, nil)
	criticalPathService := NewCriticalPathService(bomRepo, itemRepo, inventoryRepo, nil)
	orchestrator := NewPlanningOrchestrator(mrpService, criticalPathService)

	// Create a demand for F1_ENGINE
	demand := &entities.DemandRequirement{
		PartNumber:   "F1_ENGINE",
		Quantity:     1,
		NeedDate:     time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		DemandSource: "TEST_ALLOCATION_CONTEXT",
		Location:     "KSC",
		TargetSerial: "AS502",
	}

	// Run integrated planning (MRP + Critical Path)
	planningResult, err := orchestrator.RunCompletePlanning(context.Background(), []*entities.DemandRequirement{demand}, 5)
	if err != nil {
		t.Fatalf("Failed to run complete planning: %v", err)
	}

	t.Logf("Complete Planning Results:")
	t.Logf("  %s", planningResult.GetSummary())

	// Verify that the analysis used allocation context effectively
	if len(planningResult.CriticalPath.TopPaths) == 0 {
		t.Error("Expected at least one critical path")
	}

	// Check that some parts have inventory allocated
	foundAllocatedPart := false
	for _, path := range planningResult.CriticalPath.TopPaths {
		for _, node := range path.PathDetails {
			if node.HasInventory && node.InventoryQty > 0 {
				foundAllocatedPart = true
				t.Logf("  Part %s has allocation: %d units (required: %d)", 
					node.PartNumber, node.InventoryQty, node.RequiredQty)
			}
		}
	}

	if !foundAllocatedPart {
		t.Log("  Note: No parts had inventory allocations in this test scenario")
	}
}

func TestBOMTraverser_AllocationContext(t *testing.T) {
	bomRepo, itemRepo, inventoryRepo, _ := testinghelpers.BuildAerospaceTestData()
	
	alternateSelector := NewAlternateSelector(inventoryRepo, itemRepo)
	bomTraverser := NewBOMTraverser(bomRepo, itemRepo, alternateSelector)

	// Create some allocation results
	allocations := []entities.AllocationResult{
		{
			PartNumber:      "F1_ENGINE",
			Location:        "KSC",
			AllocatedQty:    2,
			RemainingDemand: 3,
		},
		{
			PartNumber:      "F1_TURBOPUMP_V1", 
			Location:        "KSC",
			AllocatedQty:    1,
			RemainingDemand: 0,
		},
	}

	// Set allocation context
	bomTraverser.SetAllocationContext(allocations)

	// Create a visitor to test allocation context
	visitor := NewCriticalPathVisitor(inventoryRepo, nil)

	// Test traversal
	result, err := bomTraverser.TraverseBOM(
		context.Background(),
		"F1_ENGINE",
		"AS502",
		"KSC",
		5,
		0,
		visitor,
	)
	if err != nil {
		t.Fatalf("Failed to traverse BOM with allocation context: %v", err)
	}

	paths := result.([]entities.CriticalPath)
	if len(paths) == 0 {
		t.Error("Expected at least one path")
	}

	// Verify that allocation context was used
	foundAllocationInfo := false
	for _, path := range paths {
		for _, node := range path.PathDetails {
			if node.PartNumber == "F1_ENGINE" && node.HasInventory && node.InventoryQty == 2 {
				foundAllocationInfo = true
				t.Logf("Allocation context correctly applied to %s: allocated=%d, required=%d", 
					node.PartNumber, node.InventoryQty, node.RequiredQty)
			}
		}
	}

	if !foundAllocationInfo {
		t.Error("Expected to find allocation context applied to F1_ENGINE")
	}

	// Clean up
	bomTraverser.ClearAllocationContext()
}