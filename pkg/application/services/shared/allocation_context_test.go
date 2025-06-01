package shared

import (
	"testing"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

func TestAllocationMap_BasicOperations(t *testing.T) {
	// Test creating empty allocation map
	allocMap := NewAllocationMap()
	if allocMap.Size() != 0 {
		t.Errorf("Expected empty map, got size %d", allocMap.Size())
	}

	// Test Set and Get
	context := &AllocationContext{
		AllocatedQty:    5,
		RemainingDemand: 3,
		HasAllocation:   true,
	}

	allocMap.Set("TEST_PART", "TEST_LOC", context)

	retrieved := allocMap.Get("TEST_PART", "TEST_LOC")
	if retrieved == nil {
		t.Error("Expected to find allocation context")
	} else {
		if retrieved.AllocatedQty != 5 {
			t.Errorf("Expected allocated qty 5, got %d", retrieved.AllocatedQty)
		}
		if retrieved.RemainingDemand != 3 {
			t.Errorf("Expected remaining demand 3, got %d", retrieved.RemainingDemand)
		}
		if !retrieved.HasAllocation {
			t.Error("Expected HasAllocation to be true")
		}
	}

	// Test Has method
	if !allocMap.Has("TEST_PART", "TEST_LOC") {
		t.Error("Expected Has to return true")
	}

	if allocMap.Has("NONEXISTENT", "LOC") {
		t.Error("Expected Has to return false for non-existent part")
	}
}

func TestAllocationMap_FromResults(t *testing.T) {
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

	allocMap := NewAllocationMapFromResults(allocations)

	// Test basic operations
	if allocMap.Size() != 2 {
		t.Errorf("Expected map size 2, got %d", allocMap.Size())
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

	// Test Clear
	allocMap.Clear()
	if allocMap.Size() != 0 {
		t.Errorf("Expected map to be empty after clear, got size %d", allocMap.Size())
	}
}
