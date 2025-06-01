package memory

import (
	"testing"
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

func TestInventoryRepository_SaveAndGetInventoryLot(t *testing.T) {
	repo := NewInventoryRepository()

	lot := &entities.InventoryLot{
		PartNumber:  "TEST_PART",
		LotNumber:   "LOT001",
		Location:    "WAREHOUSE_A",
		Quantity:    entities.Quantity(100),
		ReceiptDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Status:      entities.Available,
	}

	// Save inventory lot
	err := repo.SaveInventoryLot(lot)
	if err != nil {
		t.Fatalf("Failed to save inventory lot: %v", err)
	}

	// Get inventory for part
	lots, err := repo.GetInventoryLots("TEST_PART", "WAREHOUSE_A")
	if err != nil {
		t.Fatalf("Failed to get inventory lots: %v", err)
	}

	if len(lots) != 1 {
		t.Fatalf("Expected 1 inventory lot, got %d", len(lots))
	}

	retrieved := lots[0]
	if retrieved.PartNumber != lot.PartNumber {
		t.Errorf("Expected part number %s, got %s", lot.PartNumber, retrieved.PartNumber)
	}

	if retrieved.LotNumber != lot.LotNumber {
		t.Errorf("Expected lot number %s, got %s", lot.LotNumber, retrieved.LotNumber)
	}

	if retrieved.Quantity != lot.Quantity {
		t.Errorf("Expected quantity %d, got %d", lot.Quantity, retrieved.Quantity)
	}

	if retrieved.Status != lot.Status {
		t.Errorf("Expected status %v, got %v", lot.Status, retrieved.Status)
	}
}

func TestInventoryRepository_AllocateInventory(t *testing.T) {
	tests := []struct {
		name              string
		requestedQty      entities.Quantity
		expectedAllocated entities.Quantity
		expectedRemaining entities.Quantity
	}{
		{
			name:              "partial_allocation",
			requestedQty:      entities.Quantity(30),
			expectedAllocated: entities.Quantity(30),
			expectedRemaining: entities.Quantity(0),
		},
		{
			name:              "full_lot_allocation",
			requestedQty:      entities.Quantity(50),
			expectedAllocated: entities.Quantity(50),
			expectedRemaining: entities.Quantity(0),
		},
		{
			name:              "multi_lot_allocation",
			requestedQty:      entities.Quantity(70),
			expectedAllocated: entities.Quantity(70), // 50 from LOT001 + 20 from LOT002
			expectedRemaining: entities.Quantity(0),
		},
		{
			name:              "insufficient_inventory",
			requestedQty:      entities.Quantity(200),
			expectedAllocated: entities.Quantity(80), // 50 + 30 = 80 total available
			expectedRemaining: entities.Quantity(120),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh repository for each test
			repo := NewInventoryRepository()

			// Add inventory lots
			lots := []*entities.InventoryLot{
				{
					PartNumber:  "TEST_PART",
					LotNumber:   "LOT001",
					Location:    "WAREHOUSE_A",
					Quantity:    entities.Quantity(50),
					ReceiptDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					Status:      entities.Available,
				},
				{
					PartNumber:  "TEST_PART",
					LotNumber:   "LOT002",
					Location:    "WAREHOUSE_A",
					Quantity:    entities.Quantity(30),
					ReceiptDate: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
					Status:      entities.Available,
				},
				{
					PartNumber:  "TEST_PART",
					LotNumber:   "LOT003",
					Location:    "WAREHOUSE_A",
					Quantity:    entities.Quantity(20),
					ReceiptDate: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC),
					Status:      entities.Quarantine, // Should not be allocated
				},
			}

			for _, lot := range lots {
				err := repo.SaveInventoryLot(lot)
				if err != nil {
					t.Fatalf("Failed to save inventory lot: %v", err)
				}
			}

			allocation, err := repo.AllocateInventory("TEST_PART", "WAREHOUSE_A", tt.requestedQty)
			if err != nil {
				t.Fatalf("Failed to allocate inventory: %v", err)
			}

			if allocation.AllocatedQty != tt.expectedAllocated {
				t.Errorf("Expected allocated quantity %d, got %d", tt.expectedAllocated, allocation.AllocatedQty)
			}

			if allocation.RemainingDemand != tt.expectedRemaining {
				t.Errorf("Expected remaining demand %d, got %d", tt.expectedRemaining, allocation.RemainingDemand)
			}

			if allocation.PartNumber != "TEST_PART" {
				t.Errorf("Expected part number TEST_PART, got %s", allocation.PartNumber)
			}

			if allocation.Location != "WAREHOUSE_A" {
				t.Errorf("Expected location WAREHOUSE_A, got %s", allocation.Location)
			}
		})
	}
}

func TestInventoryRepository_GetAvailableQuantity(t *testing.T) {
	repo := NewInventoryRepository()

	// Add inventory lots with different statuses
	lots := []*entities.InventoryLot{
		{
			PartNumber:  "TEST_PART",
			LotNumber:   "LOT001",
			Location:    "WAREHOUSE_A",
			Quantity:    entities.Quantity(100),
			ReceiptDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			Status:      entities.Available,
		},
		{
			PartNumber:  "TEST_PART",
			LotNumber:   "LOT002",
			Location:    "WAREHOUSE_A",
			Quantity:    entities.Quantity(50),
			ReceiptDate: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
			Status:      entities.Quarantine, // Should not count
		},
		{
			PartNumber:  "TEST_PART",
			LotNumber:   "LOT003",
			Location:    "WAREHOUSE_B", // Different location
			Quantity:    entities.Quantity(75),
			ReceiptDate: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC),
			Status:      entities.Available,
		},
		{
			PartNumber:  "OTHER_PART",
			LotNumber:   "LOT004",
			Location:    "WAREHOUSE_A",
			Quantity:    entities.Quantity(25),
			ReceiptDate: time.Date(2025, 1, 4, 0, 0, 0, 0, time.UTC),
			Status:      entities.Available,
		},
	}

	for _, lot := range lots {
		err := repo.SaveInventoryLot(lot)
		if err != nil {
			t.Fatalf("Failed to save inventory lot: %v", err)
		}
	}

	tests := []struct {
		name        string
		partNumber  entities.PartNumber
		location    string
		expectedQty entities.Quantity
	}{
		{
			name:        "available_inventory_warehouse_a",
			partNumber:  "TEST_PART",
			location:    "WAREHOUSE_A",
			expectedQty: entities.Quantity(100), // Only LOT001 is available
		},
		{
			name:        "available_inventory_warehouse_b",
			partNumber:  "TEST_PART",
			location:    "WAREHOUSE_B",
			expectedQty: entities.Quantity(75),
		},
		{
			name:        "different_part",
			partNumber:  "OTHER_PART",
			location:    "WAREHOUSE_A",
			expectedQty: entities.Quantity(25),
		},
		{
			name:        "no_inventory",
			partNumber:  "NONEXISTENT_PART",
			location:    "WAREHOUSE_A",
			expectedQty: entities.Quantity(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			availableQty, err := repo.GetAvailableQuantity(tt.partNumber, tt.location)
			if err != nil {
				t.Fatalf("Failed to get available quantity: %v", err)
			}

			if availableQty != tt.expectedQty {
				t.Errorf("Expected available quantity %d, got %d", tt.expectedQty, availableQty)
			}
		})
	}
}

func TestInventoryRepository_UpdateInventoryStatus(t *testing.T) {
	repo := NewInventoryRepository()

	lot := &entities.InventoryLot{
		PartNumber:  "TEST_PART",
		LotNumber:   "LOT001",
		Location:    "WAREHOUSE_A",
		Quantity:    entities.Quantity(100),
		ReceiptDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Status:      entities.Available,
	}

	// Save inventory lot
	err := repo.SaveInventoryLot(lot)
	if err != nil {
		t.Fatalf("Failed to save inventory lot: %v", err)
	}

	// Update status to allocated
	err = repo.UpdateInventoryStatus("TEST_PART", "LOT001", "WAREHOUSE_A", entities.Allocated)
	if err != nil {
		t.Fatalf("Failed to update inventory status: %v", err)
	}

	// Verify status was updated - use GetAllInventoryLots since GetInventoryLots only returns Available lots
	allLots, err := repo.GetAllInventoryLots()
	if err != nil {
		t.Fatalf("Failed to get all inventory lots: %v", err)
	}

	var foundLot *entities.InventoryLot
	for _, lot := range allLots {
		if lot.PartNumber == "TEST_PART" && lot.LotNumber == "LOT001" && lot.Location == "WAREHOUSE_A" {
			foundLot = lot
			break
		}
	}

	if foundLot == nil {
		t.Fatal("Could not find the updated lot")
	}

	if foundLot.Status != entities.Allocated {
		t.Errorf("Expected status %v, got %v", entities.Allocated, foundLot.Status)
	}

	// Verify available quantity is now zero
	availableQty, err := repo.GetAvailableQuantity("TEST_PART", "WAREHOUSE_A")
	if err != nil {
		t.Fatalf("Failed to get available quantity: %v", err)
	}

	if availableQty != 0 {
		t.Errorf("Expected available quantity 0 after allocating, got %d", availableQty)
	}
}
