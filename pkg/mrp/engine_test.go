package mrp

import (
	"context"
	"testing"
	"time"
)

func TestEngine_ExplodeDemand_SingleLevel(t *testing.T) {
	ctx := context.Background()
	
	// Setup repositories
	bomRepo := NewTestBOMRepository()
	inventoryRepo := NewInMemoryInventoryRepository()
	
	// Add test item
	bomRepo.AddItem(Item{
		PartNumber:      "SIMPLE_ASSEMBLY",
		Description:     "Simple Assembly",
		LeadTimeDays:    30,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(1),
		SafetyStock:     Quantity(0),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddItem(Item{
		PartNumber:      "COMPONENT_A",
		Description:     "Component A",
		LeadTimeDays:    15,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(1),
		SafetyStock:     Quantity(0),
		UnitOfMeasure:   "EA",
	})
	
	// Add BOM line
	bomRepo.AddBOMLine(BOMLine{
		ParentPN:     "SIMPLE_ASSEMBLY",
		ChildPN:      "COMPONENT_A",
		QtyPer:       Quantity(2),
		FindNumber:   100,
		Effectivity:  SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
	})
	
	// Create engine
	engine := NewTestEngine(bomRepo, inventoryRepo)
	
	// Create demand
	needDate := time.Now().Add(30 * 24 * time.Hour)
	demands := []DemandRequirement{
		{
			PartNumber:   "SIMPLE_ASSEMBLY",
			Quantity:     Quantity(1),
			NeedDate:     needDate,
			DemandSource: "TEST_ORDER",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}
	
	// Execute MRP
	result, err := engine.ExplodeDemand(ctx, demands)
	if err != nil {
		t.Fatalf("ExplodeDemand failed: %v", err)
	}
	
	// Verify results
	if len(result.PlannedOrders) == 0 {
		t.Error("Expected planned orders but got none")
	}
	
	// Should have orders for both the assembly and the component
	foundAssembly := false
	foundComponent := false
	
	for _, order := range result.PlannedOrders {
		if order.PartNumber == "SIMPLE_ASSEMBLY" {
			foundAssembly = true
			if order.Quantity != 1 {
				t.Errorf("Expected assembly quantity 1, got %d", order.Quantity)
			}
		}
		if order.PartNumber == "COMPONENT_A" {
			foundComponent = true
			if order.Quantity != 2 {
				t.Errorf("Expected component quantity 2, got %d", order.Quantity)
			}
		}
	}
	
	if !foundAssembly {
		t.Error("Expected planned order for assembly")
	}
	if !foundComponent {
		t.Error("Expected planned order for component")
	}
}

func TestEngine_ExplodeDemand_SerialEffectivity(t *testing.T) {
	ctx := context.Background()
	
	// Use aerospace test data
	bomRepo, inventoryRepo := buildAerospaceTestData()
	
	engine := NewTestEngine(bomRepo, inventoryRepo)
	
	needDate := time.Date(2025, 8, 15, 0, 0, 0, 0, time.UTC)
	
	tests := []struct {
		name         string
		targetSerial string
		expectedVac  PartNumber
	}{
		{
			name:         "early_serial_uses_v1",
			targetSerial: "AS505",
			expectedVac:  "J2_ENGINE_V1",
		},
		{
			name:         "late_serial_uses_v2",
			targetSerial: "AS507",
			expectedVac:  "J2_ENGINE_V2",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			demands := []DemandRequirement{
				{
					PartNumber:   "SATURN_V",
					Quantity:     Quantity(1),
					NeedDate:     needDate,
					DemandSource: "TEST_MISSION",
					Location:     "KENNEDY",
					TargetSerial: tt.targetSerial,
				},
			}
			
			result, err := engine.ExplodeDemand(ctx, demands)
			if err != nil {
				t.Fatalf("ExplodeDemand failed: %v", err)
			}
			
			// Check that the correct vacuum engine variant is planned
			foundCorrectVac := false
			for _, order := range result.PlannedOrders {
				if order.PartNumber == tt.expectedVac {
					foundCorrectVac = true
					break
				}
			}
			
			if !foundCorrectVac {
				t.Errorf("Expected planned order for %s but didn't find it", tt.expectedVac)
			}
		})
	}
}

func TestEngine_ExplodeDemand_InventoryAllocation(t *testing.T) {
	ctx := context.Background()
	
	// Setup repositories
	bomRepo := NewTestBOMRepository()
	inventoryRepo := NewInMemoryInventoryRepository()
	
	// Add test item
	bomRepo.AddItem(Item{
		PartNumber:      "TEST_PART",
		Description:     "Test Part",
		LeadTimeDays:    15,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(1),
		SafetyStock:     Quantity(0),
		UnitOfMeasure:   "EA",
	})
	
	// Add inventory
	inventoryRepo.AddLotInventory(InventoryLot{
		PartNumber:   "TEST_PART",
		LotNumber:    "LOT001",
		Location:     "WAREHOUSE",
		Quantity:     Quantity(5),
		ReceiptDate:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Status:       Available,
	})
	
	engine := NewTestEngine(bomRepo, inventoryRepo)
	
	// Create demand for 3 units (should partially allocate from inventory)
	demands := []DemandRequirement{
		{
			PartNumber:   "TEST_PART",
			Quantity:     Quantity(3),
			NeedDate:     time.Now().Add(30 * 24 * time.Hour),
			DemandSource: "TEST_ORDER",
			Location:     "WAREHOUSE",
			TargetSerial: "SN001",
		},
	}
	
	result, err := engine.ExplodeDemand(ctx, demands)
	if err != nil {
		t.Fatalf("ExplodeDemand failed: %v", err)
	}
	
	
	// Should have allocation results
	if len(result.Allocations) == 0 {
		t.Error("Expected allocations but got none")
	}
	
	allocation := result.Allocations[0]
	if allocation.PartNumber != "TEST_PART" {
		t.Errorf("Expected allocation for TEST_PART, got %s", allocation.PartNumber)
	}
	
	if allocation.AllocatedQty != 3 {
		t.Errorf("Expected allocated quantity 3, got %d", allocation.AllocatedQty)
	}
	
	if allocation.RemainingDemand != 0 {
		t.Errorf("Expected no remaining demand, got %d", allocation.RemainingDemand)
	}
}

func TestEngine_ExplodeDemand_Memoization(t *testing.T) {
	ctx := context.Background()
	
	// Setup repositories with multi-level BOM
	bomRepo := NewTestBOMRepository()
	inventoryRepo := NewInMemoryInventoryRepository()
	
	// Add items
	bomRepo.AddItem(Item{
		PartNumber:      "LEVEL_0",
		Description:     "Level 0 Assembly",
		LeadTimeDays:    30,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(1),
		SafetyStock:     Quantity(0),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddItem(Item{
		PartNumber:      "LEVEL_1",
		Description:     "Level 1 Subassembly",
		LeadTimeDays:    20,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(1),
		SafetyStock:     Quantity(0),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddItem(Item{
		PartNumber:      "LEVEL_2",
		Description:     "Level 2 Component",
		LeadTimeDays:    10,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(1),
		SafetyStock:     Quantity(0),
		UnitOfMeasure:   "EA",
	})
	
	// Add BOM lines - LEVEL_1 is used twice in LEVEL_0
	bomRepo.AddBOMLine(BOMLine{
		ParentPN:     "LEVEL_0",
		ChildPN:      "LEVEL_1",
		QtyPer:       Quantity(2),
		FindNumber:   100,
		Effectivity:  SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
	})
	
	bomRepo.AddBOMLine(BOMLine{
		ParentPN:     "LEVEL_1",
		ChildPN:      "LEVEL_2",
		QtyPer:       Quantity(3),
		FindNumber:   200,
		Effectivity:  SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
	})
	
	engine := NewTestEngine(bomRepo, inventoryRepo)
	
	// Create demand
	demands := []DemandRequirement{
		{
			PartNumber:   "LEVEL_0",
			Quantity:     Quantity(1),
			NeedDate:     time.Now().Add(60 * 24 * time.Hour),
			DemandSource: "TEST_ORDER",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}
	
	result, err := engine.ExplodeDemand(ctx, demands)
	if err != nil {
		t.Fatalf("ExplodeDemand failed: %v", err)
	}
	
	// Verify cache was populated
	if len(result.ExplosionCache) == 0 {
		t.Error("Expected explosion cache to be populated")
	}
	
	// Should have total quantity of 6 for LEVEL_2 (2 * 3)
	foundLevel2 := false
	for _, order := range result.PlannedOrders {
		if order.PartNumber == "LEVEL_2" {
			foundLevel2 = true
			if order.Quantity != 6 {
				t.Errorf("Expected LEVEL_2 quantity 6, got %d", order.Quantity)
			}
			break
		}
	}
	
	if !foundLevel2 {
		t.Error("Expected planned order for LEVEL_2")
	}
}

func TestEngine_ExplodeDemand_MultipleTargetSerials(t *testing.T) {
	ctx := context.Background()
	
	// Use aerospace test data
	bomRepo, inventoryRepo := buildAerospaceTestData()
	
	engine := NewTestEngine(bomRepo, inventoryRepo)
	
	needDate := time.Date(2025, 8, 15, 0, 0, 0, 0, time.UTC)
	
	// Create demands for different target serials
	demands := []DemandRequirement{
		{
			PartNumber:   "SATURN_V",
			Quantity:     Quantity(1),
			NeedDate:     needDate,
			DemandSource: "APOLLO_OLD",
			Location:     "KENNEDY",
			TargetSerial: "AS505", // Should use J2_ENGINE_V1
		},
		{
			PartNumber:   "SATURN_V",
			Quantity:     Quantity(1),
			NeedDate:     needDate,
			DemandSource: "APOLLO_NEW",
			Location:     "KENNEDY",
			TargetSerial: "AS507", // Should use J2_ENGINE_V2
		},
	}
	
	result, err := engine.ExplodeDemand(ctx, demands)
	if err != nil {
		t.Fatalf("ExplodeDemand failed: %v", err)
	}
	
	// Should have orders for both vacuum engine variants
	foundV1 := false
	foundV2 := false
	
	for _, order := range result.PlannedOrders {
		if order.PartNumber == "J2_ENGINE_V1" {
			foundV1 = true
		}
		if order.PartNumber == "J2_ENGINE_V2" {
			foundV2 = true
		}
	}
	
	if !foundV1 {
		t.Error("Expected planned order for J2_ENGINE_V1")
	}
	if !foundV2 {
		t.Error("Expected planned order for J2_ENGINE_V2")
	}
}