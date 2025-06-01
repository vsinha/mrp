package services

import (
	"context"
	"testing"
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
	testinghelpers "github.com/vsinha/mrp/pkg/infrastructure/testing"
)

func TestPlanningOrchestrator_AnalyzeCriticalPathWithMRPResults(t *testing.T) {
	bomRepo, itemRepo, inventoryRepo, demandRepo := testinghelpers.BuildAerospaceTestData()

	mrpService := NewMRPService()
	criticalPathService := NewCriticalPathService(bomRepo, itemRepo, inventoryRepo, nil)
	orchestrator := NewPlanningOrchestrator(
		mrpService,
		criticalPathService,
		bomRepo,
		itemRepo,
		inventoryRepo,
		demandRepo,
	)

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
	planningResult, err := orchestrator.RunCompletePlanning(
		context.Background(),
		[]*entities.DemandRequirement{demand},
		5,
	)
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

	bomTraverser := NewBOMTraverser(bomRepo, itemRepo, inventoryRepo)

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

func TestAllocationMap(t *testing.T) {
	// Test creating empty allocation map
	allocMap := NewAllocationMap()
	if allocMap.Size() != 0 {
		t.Errorf("Expected empty map, got size %d", allocMap.Size())
	}

	// Test creating from allocation results
	allocations := []entities.AllocationResult{
		{
			PartNumber:      "ENGINE_A",
			Location:        "PLANT_1",
			AllocatedQty:    5,
			RemainingDemand: 2,
		},
		{
			PartNumber:      "ENGINE_B",
			Location:        "PLANT_2",
			AllocatedQty:    0,
			RemainingDemand: 10,
		},
	}

	allocMap = NewAllocationMapFromResults(allocations)

	// Test basic operations
	if allocMap.Size() != 2 {
		t.Errorf("Expected map size 2, got %d", allocMap.Size())
	}

	// Test Get method
	context := allocMap.Get("ENGINE_A", "PLANT_1")
	if context == nil {
		t.Error("Expected to find allocation context for ENGINE_A")
	} else {
		if context.AllocatedQty != 5 {
			t.Errorf("Expected allocated qty 5, got %d", context.AllocatedQty)
		}
		if context.RemainingDemand != 2 {
			t.Errorf("Expected remaining demand 2, got %d", context.RemainingDemand)
		}
		if !context.HasAllocation {
			t.Error("Expected HasAllocation to be true")
		}
	}

	// Test Has method
	if !allocMap.Has("ENGINE_A", "PLANT_1") {
		t.Error("Expected Has to return true for ENGINE_A")
	}

	if allocMap.Has("ENGINE_C", "PLANT_3") {
		t.Error("Expected Has to return false for non-existent ENGINE_C")
	}

	// Test aggregate methods
	totalAllocated := allocMap.GetTotalAllocated()
	if totalAllocated != 5 {
		t.Errorf("Expected total allocated 5, got %d", totalAllocated)
	}

	totalDemand := allocMap.GetTotalDemand()
	if totalDemand != 17 { // (5+2) + (0+10) = 17
		t.Errorf("Expected total demand 17, got %d", totalDemand)
	}

	coverageRatio := allocMap.GetCoverageRatio()
	expectedRatio := 5.0 / 17.0
	if coverageRatio < expectedRatio-0.001 || coverageRatio > expectedRatio+0.001 {
		t.Errorf("Expected coverage ratio ~%.3f, got %.3f", expectedRatio, coverageRatio)
	}

	// Test GetAllParts
	parts := allocMap.GetAllParts()
	if len(parts) != 2 {
		t.Errorf("Expected 2 parts, got %d", len(parts))
	}

	partSet := make(map[entities.PartNumber]bool)
	for _, part := range parts {
		partSet[part] = true
	}
	if !partSet["ENGINE_A"] || !partSet["ENGINE_B"] {
		t.Error("Expected to find both ENGINE_A and ENGINE_B in parts list")
	}

	// Test Clear
	allocMap.Clear()
	if allocMap.Size() != 0 {
		t.Errorf("Expected map to be empty after clear, got size %d", allocMap.Size())
	}

	// Test String method (for debugging)
	allocMap.Set("TEST_PART", "TEST_LOC", &AllocationContext{
		AllocatedQty:    1,
		RemainingDemand: 2,
		HasAllocation:   true,
	})

	str := allocMap.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}
	t.Logf("AllocationMap string representation: %s", str)
}

// TestAllocationContext_RealWorldUsage shows a more realistic usage pattern
func TestAllocationContext_RealWorldUsage(t *testing.T) {
	// Simulate MRP results for a complex assembly
	mrpResults := []entities.AllocationResult{
		{PartNumber: "AIRCRAFT_FRAME", Location: "SEATTLE", AllocatedQty: 1, RemainingDemand: 2},
		{PartNumber: "ENGINE_LEFT", Location: "SEATTLE", AllocatedQty: 0, RemainingDemand: 1},
		{PartNumber: "ENGINE_RIGHT", Location: "SEATTLE", AllocatedQty: 1, RemainingDemand: 0},
		{PartNumber: "AVIONICS_SUITE", Location: "PHOENIX", AllocatedQty: 2, RemainingDemand: 1},
		{PartNumber: "LANDING_GEAR", Location: "WICHITA", AllocatedQty: 3, RemainingDemand: 0},
	}

	// Create allocation map
	allocMap := NewAllocationMapFromResults(mrpResults)

	// Check overall allocation coverage
	totalDemand := allocMap.GetTotalDemand()
	totalAllocated := allocMap.GetTotalAllocated()
	overallCoverage := allocMap.GetCoverageRatio()

	t.Logf("Production Planning Summary:")
	t.Logf("  Total Demand: %d units", totalDemand)
	t.Logf("  Total Allocated: %d units", totalAllocated)
	t.Logf("  Overall Coverage: %.1f%%", overallCoverage*100)

	// Analyze allocation by location
	locationStats := make(map[string]struct {
		allocated entities.Quantity
		demand    entities.Quantity
	})

	for _, result := range mrpResults {
		stats := locationStats[result.Location]
		stats.allocated += result.AllocatedQty
		stats.demand += result.AllocatedQty + result.RemainingDemand
		locationStats[result.Location] = stats
	}

	t.Logf("  By Location:")
	for location, stats := range locationStats {
		coverage := float64(stats.allocated) / float64(stats.demand) * 100
		t.Logf("    %s: %.1f%% coverage (%d/%d)", location, coverage, stats.allocated, stats.demand)
	}

	// Check specific parts for critical path analysis
	criticalParts := []entities.PartNumber{"AIRCRAFT_FRAME", "ENGINE_LEFT", "ENGINE_RIGHT"}

	t.Logf("  Critical Parts Analysis:")
	for _, part := range criticalParts {
		for _, result := range mrpResults {
			if result.PartNumber == part {
				context := allocMap.Get(part, result.Location)
				if context != nil {
					partCoverage := float64(context.AllocatedQty) /
						float64(context.AllocatedQty+context.RemainingDemand) * 100
					status := "⚠️  SHORTAGE"
					if context.HasAllocation && partCoverage >= 100 {
						status = "✅ COVERED"
					} else if context.HasAllocation {
						status = "🔶 PARTIAL"
					}

					t.Logf("    %s: %s (%.1f%% coverage)", part, status, partCoverage)
				}
			}
		}
	}

	// Verify expected results
	expectedTotal := entities.Quantity(11) // (1+2) + (0+1) + (1+0) + (2+1) + (3+0) = 11
	if totalDemand != expectedTotal {
		t.Errorf("Expected total demand %d, got %d", expectedTotal, totalDemand)
	}

	if totalAllocated != 7 { // 1 + 0 + 1 + 2 + 3 = 7
		t.Errorf("Expected total allocated 7, got %d", totalAllocated)
	}

	if overallCoverage < 0.5 || overallCoverage > 1.0 {
		t.Errorf("Unexpected overall coverage: %.3f", overallCoverage)
	}
}
