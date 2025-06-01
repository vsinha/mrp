package memory

import (
	"strings"
	"testing"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

func TestItemRepository_SaveItem(t *testing.T) {
	repo := NewItemRepository(10)

	item := &entities.Item{
		PartNumber:    "TEST_PART",
		Description:   "Test Part",
		LeadTimeDays:  20,
		LotSizeRule:   entities.MinimumQty,
		MinOrderQty:   entities.Quantity(5),
		SafetyStock:   entities.Quantity(2),
		UnitOfMeasure: "EA",
	}

	// Save item
	err := repo.SaveItem(item)
	if err != nil {
		t.Fatalf("Failed to save item: %v", err)
	}

	// Retrieve item
	retrieved, err := repo.GetItem("TEST_PART")
	if err != nil {
		t.Fatalf("Failed to get item: %v", err)
	}

	if retrieved.PartNumber != item.PartNumber {
		t.Errorf("Expected part number %s, got %s", item.PartNumber, retrieved.PartNumber)
	}

	if retrieved.Description != item.Description {
		t.Errorf("Expected description %s, got %s", item.Description, retrieved.Description)
	}

	if retrieved.LeadTimeDays != item.LeadTimeDays {
		t.Errorf("Expected lead time %d, got %d", item.LeadTimeDays, retrieved.LeadTimeDays)
	}

	if retrieved.LotSizeRule != item.LotSizeRule {
		t.Errorf("Expected lot size rule %v, got %v", item.LotSizeRule, retrieved.LotSizeRule)
	}
}

func TestItemRepository_SaveItem_Duplicate(t *testing.T) {
	repo := NewItemRepository(10)

	item := &entities.Item{
		PartNumber:    "DUPLICATE_PART",
		Description:   "First Item",
		LeadTimeDays:  10,
		LotSizeRule:   entities.LotForLot,
		MinOrderQty:   entities.Quantity(1),
		SafetyStock:   entities.Quantity(0),
		UnitOfMeasure: "EA",
	}

	// Save item first time - should succeed
	err := repo.SaveItem(item)
	if err != nil {
		t.Fatalf("Failed to save item first time: %v", err)
	}

	// Try to save same part number again - should fail
	duplicateItem := &entities.Item{
		PartNumber:    "DUPLICATE_PART",
		Description:   "Second Item",
		LeadTimeDays:  20,
		LotSizeRule:   entities.MinimumQty,
		MinOrderQty:   entities.Quantity(5),
		SafetyStock:   entities.Quantity(1),
		UnitOfMeasure: "EA",
	}

	err = repo.SaveItem(duplicateItem)
	if err == nil {
		t.Error("Expected error when saving duplicate part number, got none")
	}

	if !strings.Contains(err.Error(), "duplicate part number") {
		t.Errorf("Expected error message to contain 'duplicate part number', got: %v", err)
	}

	// Verify original item is still there and unchanged
	retrieved, err := repo.GetItem("DUPLICATE_PART")
	if err != nil {
		t.Fatalf("Failed to get original item: %v", err)
	}

	if retrieved.Description != "First Item" {
		t.Errorf("Expected original description 'First Item', got %s", retrieved.Description)
	}
}

func TestItemRepository_LoadItems_WithDuplicates(t *testing.T) {
	repo := NewItemRepository(10)

	items := []*entities.Item{
		{
			PartNumber:    "PART_1",
			Description:   "Part One",
			LeadTimeDays:  10,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "PART_2",
			Description:   "Part Two",
			LeadTimeDays:  15,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "PART_1", // Duplicate
			Description:   "Part One Duplicate",
			LeadTimeDays:  20,
			LotSizeRule:   entities.MinimumQty,
			MinOrderQty:   entities.Quantity(5),
			SafetyStock:   entities.Quantity(1),
			UnitOfMeasure: "EA",
		},
	}

	// Load items should fail due to duplicate
	err := repo.LoadItems(items)
	if err == nil {
		t.Error("Expected error when loading items with duplicates, got none")
	}

	if !strings.Contains(err.Error(), "Duplicate part numbers found") {
		t.Errorf("Expected error message to contain 'Duplicate part numbers found', got: %v", err)
	}

	if !strings.Contains(err.Error(), "PART_1") {
		t.Errorf("Expected error message to contain 'PART_1', got: %v", err)
	}
}

func TestItemRepository_LoadItems_Success(t *testing.T) {
	repo := NewItemRepository(10)

	items := []*entities.Item{
		{
			PartNumber:    "PART_A",
			Description:   "Part A",
			LeadTimeDays:  10,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "PART_B",
			Description:   "Part B",
			LeadTimeDays:  15,
			LotSizeRule:   entities.MinimumQty,
			MinOrderQty:   entities.Quantity(5),
			SafetyStock:   entities.Quantity(2),
			UnitOfMeasure: "EA",
		},
	}

	// Load items should succeed
	err := repo.LoadItems(items)
	if err != nil {
		t.Fatalf("Failed to load items: %v", err)
	}

	// Verify both items were loaded
	partA, err := repo.GetItem("PART_A")
	if err != nil {
		t.Fatalf("Failed to get PART_A: %v", err)
	}
	if partA.Description != "Part A" {
		t.Errorf("Expected description 'Part A', got %s", partA.Description)
	}

	partB, err := repo.GetItem("PART_B")
	if err != nil {
		t.Fatalf("Failed to get PART_B: %v", err)
	}
	if partB.LeadTimeDays != 15 {
		t.Errorf("Expected lead time 15, got %d", partB.LeadTimeDays)
	}
}

func TestItemRepository_GetItem_NotFound(t *testing.T) {
	repo := NewItemRepository(10)

	_, err := repo.GetItem("NONEXISTENT")
	if err == nil {
		t.Error("Expected error for nonexistent item, got none")
	}

	if !strings.Contains(err.Error(), "item not found") {
		t.Errorf("Expected error message to contain 'item not found', got: %v", err)
	}
}
