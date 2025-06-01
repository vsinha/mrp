package mrp

import (
	"context"
	"testing"
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
	testhelpers "github.com/vsinha/mrp/pkg/infrastructure/testing"
)

// Helper to create test MRP service
func newTestMRPService() *MRPService {
	config := EngineConfig{
		EnableGCPacing:  false, // Disable GC tuning in tests for predictable performance
		MaxCacheEntries: 1000,  // Smaller cache for tests
	}
	return NewMRPServiceWithConfig(config)
}

func TestMRPService_ExplodeDemand_SingleLevel(t *testing.T) {
	ctx := context.Background()

	// Setup repositories
	bomRepo, itemRepo, inventoryRepo, demandRepo := testhelpers.BuildSimpleTestData()

	// Create service
	service := newTestMRPService()

	// Create demand
	needDate := time.Now().Add(30 * 24 * time.Hour)
	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "ASSEMBLY_A",
			Quantity:     entities.Quantity(1),
			NeedDate:     needDate,
			DemandSource: "TEST_ORDER",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}

	// Execute MRP
	result, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
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
		if order.PartNumber == "ASSEMBLY_A" {
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

func TestMRPService_ExplodeDemand_SerialEffectivity(t *testing.T) {
	ctx := context.Background()

	// Use aerospace test data
	bomRepo, itemRepo, inventoryRepo, demandRepo := testhelpers.BuildAerospaceTestData()

	service := newTestMRPService()

	needDate := time.Date(2025, 8, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		targetSerial string
		expectedVac  string
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
			demands := []*entities.DemandRequirement{
				{
					PartNumber:   "SATURN_V",
					Quantity:     entities.Quantity(1),
					NeedDate:     needDate,
					DemandSource: "TEST_MISSION",
					Location:     "KENNEDY",
					TargetSerial: tt.targetSerial,
				},
			}

			result, err := service.ExplodeDemand(
				ctx,
				demands,
				bomRepo,
				itemRepo,
				inventoryRepo,
				demandRepo,
			)
			if err != nil {
				t.Fatalf("ExplodeDemand failed: %v", err)
			}

			// Check that the correct vacuum engine variant is planned
			foundCorrectVac := false
			for _, order := range result.PlannedOrders {
				if string(order.PartNumber) == tt.expectedVac {
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

func TestMRPService_ExplodeDemand_InventoryAllocation(t *testing.T) {
	ctx := context.Background()

	// Setup repositories
	bomRepo, itemRepo, inventoryRepo, demandRepo := testhelpers.BuildSimpleTestData()

	// Add inventory
	inventoryLot := &entities.InventoryLot{
		PartNumber:  "COMPONENT_A",
		LotNumber:   "LOT001",
		Location:    "FACTORY",
		Quantity:    entities.Quantity(5),
		ReceiptDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Status:      entities.Available,
	}
	err := inventoryRepo.SaveInventoryLot(inventoryLot)
	if err != nil {
		t.Fatalf("Failed to save inventory: %v", err)
	}

	service := newTestMRPService()

	// Create demand for 3 units (should partially allocate from inventory)
	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "COMPONENT_A",
			Quantity:     entities.Quantity(3),
			NeedDate:     time.Now().Add(30 * 24 * time.Hour),
			DemandSource: "TEST_ORDER",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}

	result, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
	if err != nil {
		t.Fatalf("ExplodeDemand failed: %v", err)
	}

	// Should have allocation results
	if len(result.Allocations) == 0 {
		t.Error("Expected allocations but got none")
	}

	allocation := result.Allocations[0]
	if allocation.PartNumber != "COMPONENT_A" {
		t.Errorf("Expected allocation for COMPONENT_A, got %s", allocation.PartNumber)
	}

	if allocation.AllocatedQty != 3 {
		t.Errorf("Expected allocated quantity 3, got %d", allocation.AllocatedQty)
	}

	if allocation.RemainingDemand != 0 {
		t.Errorf("Expected no remaining demand, got %d", allocation.RemainingDemand)
	}
}

func TestMRPService_ExplodeDemand_Memoization(t *testing.T) {
	ctx := context.Background()

	// Setup repositories with multi-level BOM
	bomRepo, itemRepo, inventoryRepo, demandRepo := testhelpers.BuildSimpleTestData()

	// Add more complex structure
	items := []*entities.Item{
		{
			PartNumber:    "LEVEL_0",
			Description:   "Level 0 Assembly",
			LeadTimeDays:  30,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "LEVEL_1",
			Description:   "Level 1 Subassembly",
			LeadTimeDays:  20,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "LEVEL_2",
			Description:   "Level 2 Component",
			LeadTimeDays:  10,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
	}

	for _, item := range items {
		err := itemRepo.SaveItem(item)
		if err != nil {
			t.Fatalf("Failed to save item: %v", err)
		}
	}

	// Add BOM lines - LEVEL_1 is used twice in LEVEL_0
	bomLines := []*entities.BOMLine{
		{
			ParentPN:    "LEVEL_0",
			ChildPN:     "LEVEL_1",
			QtyPer:      entities.Quantity(2),
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
		{
			ParentPN:    "LEVEL_1",
			ChildPN:     "LEVEL_2",
			QtyPer:      entities.Quantity(3),
			FindNumber:  200,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
	}

	for _, bomLine := range bomLines {
		err := bomRepo.SaveBOMLine(bomLine)
		if err != nil {
			t.Fatalf("Failed to save BOM line: %v", err)
		}
	}

	service := newTestMRPService()

	// Create demand
	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "LEVEL_0",
			Quantity:     entities.Quantity(1),
			NeedDate:     time.Now().Add(60 * 24 * time.Hour),
			DemandSource: "TEST_ORDER",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}

	result, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
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

func TestMRPService_ExplodeDemand_MultipleTargetSerials(t *testing.T) {
	ctx := context.Background()

	// Use aerospace test data
	bomRepo, itemRepo, inventoryRepo, demandRepo := testhelpers.BuildAerospaceTestData()

	service := newTestMRPService()

	needDate := time.Date(2025, 8, 15, 0, 0, 0, 0, time.UTC)

	// Create demands for different target serials
	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "SATURN_V",
			Quantity:     entities.Quantity(1),
			NeedDate:     needDate,
			DemandSource: "APOLLO_OLD",
			Location:     "KENNEDY",
			TargetSerial: "AS505", // Should use J2_ENGINE_V1
		},
		{
			PartNumber:   "SATURN_V",
			Quantity:     entities.Quantity(1),
			NeedDate:     needDate,
			DemandSource: "APOLLO_NEW",
			Location:     "KENNEDY",
			TargetSerial: "AS507", // Should use J2_ENGINE_V2
		},
	}

	result, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
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
