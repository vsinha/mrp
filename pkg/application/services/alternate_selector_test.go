package services

import (
	"testing"

	"github.com/vsinha/mrp/pkg/domain/entities"
	testinghelpers "github.com/vsinha/mrp/pkg/infrastructure/testing"
)

func TestSelectBestAlternateByPriority(t *testing.T) {
	_, _, _, _ = testinghelpers.BuildAerospaceTestData()

	// Create test alternates with different priorities
	effectivity, err := entities.NewSerialEffectivity("AS501", "AS505")
	if err != nil {
		t.Fatalf("Failed to create serial effectivity: %v", err)
	}

	// Primary turbopump (priority 0 = standard)
	primary, err := entities.NewBOMLine("F1_ENGINE", "F1_TURBOPUMP_V1", 1, 300, *effectivity, 0)
	if err != nil {
		t.Fatalf("Failed to create primary BOM line: %v", err)
	}

	// Alternate turbopump (priority 1 = first alternate)
	alternate1, err := entities.NewBOMLine("F1_ENGINE", "F1_TURBOPUMP_V2", 1, 300, *effectivity, 1)
	if err != nil {
		t.Fatalf("Failed to create alternate BOM line: %v", err)
	}

	// Another alternate (priority 2 = second alternate)
	alternate2, err := entities.NewBOMLine("F1_ENGINE", "F1_TURBOPUMP_V3", 1, 300, *effectivity, 2)
	if err != nil {
		t.Fatalf("Failed to create second alternate BOM line: %v", err)
	}

	alternates := []*entities.BOMLine{alternate1, alternate2, primary}

	// Test: Should select the primary (priority 0)
	selected := SelectBestAlternateByPriority(alternates)
	if selected == nil {
		t.Fatal("Expected to select an alternate, got nil")
	}

	if selected.Priority != 0 {
		t.Errorf("Expected to select primary (priority 0), got priority %d", selected.Priority)
	}

	if selected.ChildPN != "F1_TURBOPUMP_V1" {
		t.Errorf("Expected to select F1_TURBOPUMP_V1, got %s", selected.ChildPN)
	}
}

func TestSelectBestAlternateByPriority_EmptyList(t *testing.T) {
	// Test: Empty list should return nil
	selected := SelectBestAlternateByPriority([]*entities.BOMLine{})
	if selected != nil {
		t.Error("Expected nil for empty alternates list")
	}
}

func TestSelectBestAlternateWithInventory(t *testing.T) {
	_, _, inventoryRepo, _ := testinghelpers.BuildAerospaceTestData()

	// Create test alternates
	effectivity, err := entities.NewSerialEffectivity("AS501", "AS505")
	if err != nil {
		t.Fatalf("Failed to create serial effectivity: %v", err)
	}

	// Primary turbopump (priority 0)
	primary, err := entities.NewBOMLine("F1_ENGINE", "F1_TURBOPUMP_V1", 1, 300, *effectivity, 0)
	if err != nil {
		t.Fatalf("Failed to create primary BOM line: %v", err)
	}

	alternates := []*entities.BOMLine{primary}

	// Test: Should select based on priority since we have basic inventory setup
	selected := SelectBestAlternateWithInventory(alternates, 1, inventoryRepo)
	if selected == nil {
		t.Fatal("Expected to select an alternate, got nil")
	}

	if selected.Priority != 0 {
		t.Errorf("Expected to select primary (priority 0), got priority %d", selected.Priority)
	}
}
