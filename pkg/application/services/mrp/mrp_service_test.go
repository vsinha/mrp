package mrp

import (
	"context"
	"fmt"
	"testing"
	"time"

	testhelpers "github.com/vsinha/mrp/pkg/application/services/testing"
	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/infrastructure/repositories/memory"
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

func TestMRPService_ExplodeDemand_OrderSplitting(t *testing.T) {
	ctx := context.Background()

	// Create test repositories
	bomRepo := memory.NewBOMRepository(5)
	itemRepo := memory.NewItemRepository(5)
	inventoryRepo := memory.NewInventoryRepository()
	demandRepo := memory.NewDemandRepository()

	// Create an item with a small max order quantity to force splitting
	item, err := entities.NewItem(
		"HIGH_DEMAND_PART",
		"Part with Limited Order Size",
		10, // 10 day lead time
		entities.LotForLot,
		entities.Quantity(1),  // min qty
		entities.Quantity(15), // max qty - this will force splitting
		entities.Quantity(0),  // safety stock
		"EA",
		entities.MakeBuyMake,
	)
	if err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	err = itemRepo.SaveItem(item)
	if err != nil {
		t.Fatalf("Failed to save item: %v", err)
	}

	service := newTestMRPService()

	// Create demand for 50 units (will be split into 4 orders: 15+15+15+5)
	needDate := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "HIGH_DEMAND_PART",
			Quantity:     entities.Quantity(50),
			NeedDate:     needDate,
			DemandSource: "LARGE_ORDER",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}

	result, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
	if err != nil {
		t.Fatalf("ExplodeDemand failed: %v", err)
	}

	// Should have exactly 4 planned orders (15+15+15+5 = 50)
	expectedOrders := 4
	actualOrders := len(result.PlannedOrders)

	if actualOrders != expectedOrders {
		t.Errorf("Expected %d planned orders, got %d", expectedOrders, actualOrders)
	}

	// Verify quantities and sequential scheduling
	var totalQuantity entities.Quantity = 0
	var previousDueDate time.Time
	splitOrderCount := 0

	for i, order := range result.PlannedOrders {
		if order.PartNumber != "HIGH_DEMAND_PART" {
			continue
		}

		totalQuantity += order.Quantity

		// Check quantity constraints
		if order.Quantity > 15 {
			t.Errorf("Order %d quantity %d exceeds max order qty of 15", i, order.Quantity)
		}

		// Check forward sequential scheduling (each order due after the previous one)
		if i > 0 && !order.DueDate.After(previousDueDate) {
			t.Errorf("Order %d due date %v should be after previous due date %v for forward sequential ordering",
				i, order.DueDate, previousDueDate)
		}
		previousDueDate = order.DueDate

		// Check demand trace for split orders
		if i > 0 {
			expectedTrace := fmt.Sprintf("LARGE_ORDER (Split %d)", i+1)
			if order.DemandTrace != expectedTrace {
				t.Errorf("Expected demand trace '%s', got '%s'", expectedTrace, order.DemandTrace)
			}
			splitOrderCount++
		} else {
			// First order should have original demand trace
			if order.DemandTrace != "LARGE_ORDER" {
				t.Errorf("Expected first order demand trace 'LARGE_ORDER', got '%s'", order.DemandTrace)
			}
		}

		// Verify lead time is respected (start + lead time = due date)
		expectedDueDate := order.StartDate.Add(10 * 24 * time.Hour) // 10 day lead time
		if !order.DueDate.Equal(expectedDueDate) {
			t.Errorf("Order %d due date %v doesn't match start date + lead time %v",
				i, order.DueDate, expectedDueDate)
		}
	}

	// Verify total quantity matches demand
	if totalQuantity != 50 {
		t.Errorf("Total planned quantity %d doesn't match demand quantity 50", totalQuantity)
	}

	// Verify correct number of split orders (should be 3: splits 2, 3, and 4)
	expectedSplitOrders := 3
	if splitOrderCount != expectedSplitOrders {
		t.Errorf("Expected %d split orders, got %d", expectedSplitOrders, splitOrderCount)
	}
}

func TestMRPService_ForwardScheduling_BasicDependencyTiming(t *testing.T) {
	ctx := context.Background()

	// Create test repositories
	bomRepo := memory.NewBOMRepository(5)
	itemRepo := memory.NewItemRepository(5)
	inventoryRepo := memory.NewInventoryRepository()
	demandRepo := memory.NewDemandRepository()

	// Create items with specific lead times for predictable scheduling
	items := []*entities.Item{
		{
			PartNumber:    "PARENT_ASSY",
			Description:   "Parent Assembly",
			LeadTimeDays:  10,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			MaxOrderQty:   entities.Quantity(100),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "CHILD_COMP",
			Description:   "Child Component",
			LeadTimeDays:  5,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			MaxOrderQty:   entities.Quantity(100),
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

	// Create BOM relationship: PARENT_ASSY uses CHILD_COMP
	bomLine := &entities.BOMLine{
		ParentPN:    "PARENT_ASSY",
		ChildPN:     "CHILD_COMP",
		QtyPer:      entities.Quantity(1),
		FindNumber:  100,
		Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
	}

	err := bomRepo.SaveBOMLine(bomLine)
	if err != nil {
		t.Fatalf("Failed to save BOM line: %v", err)
	}

	service := newTestMRPService()

	// Create demand for parent assembly
	fixedTime := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "PARENT_ASSY",
			Quantity:     entities.Quantity(1),
			NeedDate:     fixedTime,
			DemandSource: "TEST_ORDER",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}

	result, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
	if err != nil {
		t.Fatalf("ExplodeDemand failed: %v", err)
	}

	// Should have orders for both parent and child
	if len(result.PlannedOrders) != 2 {
		t.Fatalf("Expected 2 planned orders, got %d", len(result.PlannedOrders))
	}

	var childOrder, parentOrder *entities.PlannedOrder
	for i := range result.PlannedOrders {
		if result.PlannedOrders[i].PartNumber == "CHILD_COMP" {
			childOrder = &result.PlannedOrders[i]
		}
		if result.PlannedOrders[i].PartNumber == "PARENT_ASSY" {
			parentOrder = &result.PlannedOrders[i]
		}
	}

	if childOrder == nil {
		t.Fatal("Child component order not found")
	}
	if parentOrder == nil {
		t.Fatal("Parent assembly order not found")
	}

	// FORWARD SCHEDULING ASSERTIONS:
	// 1. Child component should start first (no dependencies)
	// 2. Parent assembly should start AFTER child component completes
	// 3. Parent start time should equal child completion time

	if !parentOrder.StartDate.After(childOrder.StartDate) {
		t.Errorf("Parent assembly start date %v should be after child component start date %v",
			parentOrder.StartDate, childOrder.StartDate)
	}

	if !parentOrder.StartDate.Equal(childOrder.DueDate) {
		t.Errorf("Parent assembly start date %v should equal child component due date %v (forward scheduling)",
			parentOrder.StartDate, childOrder.DueDate)
	}

	// Verify lead times are respected
	expectedChildDue := childOrder.StartDate.Add(5 * 24 * time.Hour)
	if !childOrder.DueDate.Equal(expectedChildDue) {
		t.Errorf("Child due date %v doesn't match start + 5 day lead time %v",
			childOrder.DueDate, expectedChildDue)
	}

	expectedParentDue := parentOrder.StartDate.Add(10 * 24 * time.Hour)
	if !parentOrder.DueDate.Equal(expectedParentDue) {
		t.Errorf("Parent due date %v doesn't match start + 10 day lead time %v",
			parentOrder.DueDate, expectedParentDue)
	}
}

func TestMRPService_ForwardScheduling_InventoryImpact(t *testing.T) {
	ctx := context.Background()

	// Create test repositories
	bomRepo := memory.NewBOMRepository(5)
	itemRepo := memory.NewItemRepository(5)
	inventoryRepo := memory.NewInventoryRepository()
	demandRepo := memory.NewDemandRepository()

	// Create items
	items := []*entities.Item{
		{
			PartNumber:    "ASSY_WITH_INVENTORY",
			Description:   "Assembly with Inventory Available",
			LeadTimeDays:  15,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			MaxOrderQty:   entities.Quantity(100),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "COMP_WITH_INVENTORY",
			Description:   "Component with Full Inventory",
			LeadTimeDays:  7,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			MaxOrderQty:   entities.Quantity(100),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "COMP_NO_INVENTORY",
			Description:   "Component with No Inventory",
			LeadTimeDays:  10,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			MaxOrderQty:   entities.Quantity(100),
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

	// Create BOM relationships
	bomLines := []*entities.BOMLine{
		{
			ParentPN:    "ASSY_WITH_INVENTORY",
			ChildPN:     "COMP_WITH_INVENTORY",
			QtyPer:      entities.Quantity(1),
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
		{
			ParentPN:    "ASSY_WITH_INVENTORY",
			ChildPN:     "COMP_NO_INVENTORY",
			QtyPer:      entities.Quantity(1),
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

	// Add inventory for one component (full coverage)
	inventoryLot := &entities.InventoryLot{
		PartNumber:  "COMP_WITH_INVENTORY",
		LotNumber:   "LOT001",
		Location:    "FACTORY",
		Quantity:    entities.Quantity(10), // More than needed
		ReceiptDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Status:      entities.Available,
	}
	err := inventoryRepo.SaveInventoryLot(inventoryLot)
	if err != nil {
		t.Fatalf("Failed to save inventory: %v", err)
	}

	service := newTestMRPService()

	// Create demand
	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "ASSY_WITH_INVENTORY",
			Quantity:     entities.Quantity(1),
			NeedDate:     time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			DemandSource: "INVENTORY_TEST",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}

	result, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
	if err != nil {
		t.Fatalf("ExplodeDemand failed: %v", err)
	}

	// Should have orders only for parts that need production
	// COMP_WITH_INVENTORY should be satisfied by inventory (no order)
	// COMP_NO_INVENTORY should have a production order
	// ASSY_WITH_INVENTORY should have a production order

	expectedOrders := 2 // Assembly + Component without inventory
	if len(result.PlannedOrders) != expectedOrders {
		t.Errorf("Expected %d planned orders, got %d", expectedOrders, len(result.PlannedOrders))
	}

	var assemblyOrder, componentOrder *entities.PlannedOrder
	for i := range result.PlannedOrders {
		if result.PlannedOrders[i].PartNumber == "ASSY_WITH_INVENTORY" {
			assemblyOrder = &result.PlannedOrders[i]
		}
		if result.PlannedOrders[i].PartNumber == "COMP_NO_INVENTORY" {
			componentOrder = &result.PlannedOrders[i]
		}
		// Should NOT have order for COMP_WITH_INVENTORY (covered by inventory)
		if result.PlannedOrders[i].PartNumber == "COMP_WITH_INVENTORY" {
			t.Errorf("Should not have production order for COMP_WITH_INVENTORY (covered by inventory)")
		}
	}

	if componentOrder == nil {
		t.Fatal("Component without inventory order not found")
	}
	if assemblyOrder == nil {
		t.Fatal("Assembly order not found")
	}

	// FORWARD SCHEDULING WITH INVENTORY ASSERTIONS:
	// Assembly should start when component without inventory completes
	// (Component with inventory is available immediately, so doesn't delay assembly)

	if !assemblyOrder.StartDate.Equal(componentOrder.DueDate) {
		t.Errorf("Assembly start date %v should equal component due date %v (forward scheduling with inventory)",
			assemblyOrder.StartDate, componentOrder.DueDate)
	}

	// Verify inventory allocation
	if len(result.Allocations) == 0 {
		t.Error("Expected inventory allocations")
	}

	foundInventoryAllocation := false
	for _, allocation := range result.Allocations {
		if allocation.PartNumber == "COMP_WITH_INVENTORY" && allocation.AllocatedQty > 0 {
			foundInventoryAllocation = true
			if allocation.RemainingDemand != 0 {
				t.Errorf("Expected full allocation for COMP_WITH_INVENTORY, got remaining demand %d",
					allocation.RemainingDemand)
			}
		}
	}

	if !foundInventoryAllocation {
		t.Error("Expected inventory allocation for COMP_WITH_INVENTORY")
	}
}

func TestMRPService_ForwardScheduling_ParallelBranches(t *testing.T) {
	ctx := context.Background()

	// Create test repositories
	bomRepo := memory.NewBOMRepository(10)
	itemRepo := memory.NewItemRepository(10)
	inventoryRepo := memory.NewInventoryRepository()
	demandRepo := memory.NewDemandRepository()

	// Create items for parallel branch test
	items := []*entities.Item{
		{
			PartNumber:    "ROOT_ASSEMBLY",
			Description:   "Root Assembly",
			LeadTimeDays:  5,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			MaxOrderQty:   entities.Quantity(100),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "BRANCH_A_SUBASSY",
			Description:   "Branch A Sub Assembly",
			LeadTimeDays:  8,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			MaxOrderQty:   entities.Quantity(100),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "BRANCH_B_SUBASSY",
			Description:   "Branch B Sub Assembly",
			LeadTimeDays:  12,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			MaxOrderQty:   entities.Quantity(100),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "COMP_A",
			Description:   "Component A (fast)",
			LeadTimeDays:  3,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			MaxOrderQty:   entities.Quantity(100),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "COMP_B",
			Description:   "Component B (slow)",
			LeadTimeDays:  15,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			MaxOrderQty:   entities.Quantity(100),
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

	// Create BOM structure with parallel branches:
	// ROOT_ASSEMBLY
	// ├── BRANCH_A_SUBASSY (8 days)
	// │   └── COMP_A (3 days) - Total: 11 days
	// └── BRANCH_B_SUBASSY (12 days)
	//     └── COMP_B (15 days) - Total: 27 days (critical path)
	bomLines := []*entities.BOMLine{
		{
			ParentPN:    "ROOT_ASSEMBLY",
			ChildPN:     "BRANCH_A_SUBASSY",
			QtyPer:      entities.Quantity(1),
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
		{
			ParentPN:    "ROOT_ASSEMBLY",
			ChildPN:     "BRANCH_B_SUBASSY",
			QtyPer:      entities.Quantity(1),
			FindNumber:  200,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
		{
			ParentPN:    "BRANCH_A_SUBASSY",
			ChildPN:     "COMP_A",
			QtyPer:      entities.Quantity(1),
			FindNumber:  300,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
		{
			ParentPN:    "BRANCH_B_SUBASSY",
			ChildPN:     "COMP_B",
			QtyPer:      entities.Quantity(1),
			FindNumber:  400,
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
			PartNumber:   "ROOT_ASSEMBLY",
			Quantity:     entities.Quantity(1),
			NeedDate:     time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			DemandSource: "PARALLEL_TEST",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}

	result, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
	if err != nil {
		t.Fatalf("ExplodeDemand failed: %v", err)
	}

	// Should have 5 orders (one for each part)
	if len(result.PlannedOrders) != 5 {
		t.Fatalf("Expected 5 planned orders, got %d", len(result.PlannedOrders))
	}

	// Find all orders
	orders := make(map[string]*entities.PlannedOrder)
	for i := range result.PlannedOrders {
		orders[string(result.PlannedOrders[i].PartNumber)] = &result.PlannedOrders[i]
	}

	compA := orders["COMP_A"]
	compB := orders["COMP_B"]
	branchA := orders["BRANCH_A_SUBASSY"]
	branchB := orders["BRANCH_B_SUBASSY"]
	root := orders["ROOT_ASSEMBLY"]

	// PARALLEL BRANCH ASSERTIONS:
	// 1. Both components can start simultaneously (no interdependence)
	// 2. Branch A waits only for Comp A (not Comp B)
	// 3. Branch B waits only for Comp B (not Comp A)
	// 4. Root assembly waits for BOTH branches to complete
	// 5. Branch B determines the critical path (longer duration)

	// Branch A should start when Comp A completes (independent of Comp B)
	if !branchA.StartDate.Equal(compA.DueDate) {
		t.Errorf("Branch A start %v should equal Comp A due %v (independent scheduling)",
			branchA.StartDate, compA.DueDate)
	}

	// Branch B should start when Comp B completes (independent of Comp A)
	if !branchB.StartDate.Equal(compB.DueDate) {
		t.Errorf("Branch B start %v should equal Comp B due %v (independent scheduling)",
			branchB.StartDate, compB.DueDate)
	}

	// Root assembly should wait for BOTH branches (latest completion)
	latestBranchCompletion := branchA.DueDate
	if branchB.DueDate.After(latestBranchCompletion) {
		latestBranchCompletion = branchB.DueDate
	}

	if !root.StartDate.Equal(latestBranchCompletion) {
		t.Errorf("Root assembly start %v should equal latest branch completion %v",
			root.StartDate, latestBranchCompletion)
	}

	// Verify that Branch B's path is indeed the critical path (takes longer)
	branchATotal := compA.DueDate.Sub(compA.StartDate) + branchA.DueDate.Sub(branchA.StartDate)
	branchBTotal := compB.DueDate.Sub(compB.StartDate) + branchB.DueDate.Sub(branchB.StartDate)

	if branchBTotal <= branchATotal {
		t.Error("Branch B should be the critical path (longer duration)")
	}

	// Root assembly should start based on Branch B completion (critical path)
	if !root.StartDate.Equal(branchB.DueDate) {
		t.Errorf("Root assembly should start when critical path (Branch B) completes")
	}
}
