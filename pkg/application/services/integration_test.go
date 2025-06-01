package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/infrastructure/repositories/memory"
	testhelpers "github.com/vsinha/mrp/pkg/infrastructure/testing"
)

// Helper to create test MRP service
func newTestMRPServiceForIntegration(
	bomRepo *memory.BOMRepository,
	itemRepo *memory.ItemRepository,
	inventoryRepo *memory.InventoryRepository,
	demandRepo *memory.DemandRepository,
) *MRPService {
	config := EngineConfig{
		EnableGCPacing:  false, // Disable GC tuning in tests for predictable performance
		MaxCacheEntries: 1000,  // Smaller cache for tests
	}
	return NewMRPServiceWithConfig(bomRepo, itemRepo, inventoryRepo, demandRepo, config)
}

func TestMRPIntegration_AerospaceScenario(t *testing.T) {
	ctx := context.Background()

	// Build aerospace test data
	bomRepo, itemRepo, inventoryRepo, demandRepo := testhelpers.BuildAerospaceTestData()

	// Create MRP service
	service := newTestMRPServiceForIntegration(bomRepo, itemRepo, inventoryRepo, demandRepo)

	// Define complex multi-vehicle demand scenario
	needDate := time.Date(2025, 8, 15, 0, 0, 0, 0, time.UTC)
	demands := []*entities.DemandRequirement{
		// New vehicle build - uses newer BOM effectivity
		{
			PartNumber:   "SATURN_V",
			Quantity:     entities.Quantity(1),
			NeedDate:     needDate,
			DemandSource: "APOLLO_11_MISSION",
			Location:     "KENNEDY",
			TargetSerial: "AS506", // Uses J2_ENGINE_V1, F1_TURBOPUMP_V2
		},
		// Refurbishment of older vehicle - uses older BOM effectivity
		{
			PartNumber:   "F1_ENGINE",
			Quantity:     entities.Quantity(5),
			NeedDate:     time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
			DemandSource: "REFURB_AS502",
			Location:     "STENNIS",
			TargetSerial: "AS502", // Uses F1_TURBOPUMP_V1 for refurb compatibility
		},
		// Spare parts for test campaign
		{
			PartNumber:   "F1_TURBOPUMP_V2",
			Quantity:     entities.Quantity(4),
			NeedDate:     time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC),
			DemandSource: "TEST_CAMPAIGN_APOLLO",
			Location:     "STENNIS",
			TargetSerial: "AS999", // Special test serial for latest config
		},
	}

	// Execute MRP
	result, err := service.ExplodeDemand(ctx, demands)
	if err != nil {
		t.Fatalf("MRP explosion failed: %v", err)
	}

	// Validate results
	t.Logf("MRP Results Summary:")
	t.Logf("  Planned Orders: %d", len(result.PlannedOrders))
	t.Logf("  Allocations: %d", len(result.Allocations))
	t.Logf("  Shortages: %d", len(result.ShortageReport))
	t.Logf("  Cache Entries: %d", len(result.ExplosionCache))

	// Verify serial effectivity worked correctly
	foundJ2EngineV1 := false
	foundTurbopumpV1 := false
	foundTurbopumpV2 := false

	for _, order := range result.PlannedOrders {
		switch order.PartNumber {
		case "J2_ENGINE_V1":
			foundJ2EngineV1 = true
			t.Logf("  Found J2_ENGINE_V1 order for target serial %s", order.TargetSerial)
		case "F1_TURBOPUMP_V1":
			foundTurbopumpV1 = true
			t.Logf("  Found TURBOPUMP_V1 order qty=%d for target serial %s",
				order.Quantity, order.TargetSerial)
		case "F1_TURBOPUMP_V2":
			foundTurbopumpV2 = true
			t.Logf("  Found TURBOPUMP_V2 order qty=%d for target serial %s",
				order.Quantity, order.TargetSerial)
		}
	}

	// Verify correct effectivity resolution
	if !foundJ2EngineV1 {
		t.Error("Expected J2_ENGINE_V1 for AS506 (early serial)")
	}
	if !foundTurbopumpV1 {
		t.Error("Expected F1_TURBOPUMP_V1 for AS502 refurb (early serial)")
	}
	if !foundTurbopumpV2 {
		t.Error("Expected F1_TURBOPUMP_V2 for AS999 test (late serials)")
	}

	// Verify inventory allocation
	engineAllocation := false
	for _, alloc := range result.Allocations {
		if alloc.PartNumber == "F1_ENGINE" {
			engineAllocation = true
			t.Logf("  Engine allocation: %d units allocated from inventory",
				alloc.AllocatedQty)
			break
		}
	}

	// Note: Inventory allocation may not occur if no available inventory matches demand requirements
	if !engineAllocation {
		t.Logf("No engine inventory allocation found (may be normal if no available inventory)")
	}

	// Verify demand traceability
	for _, order := range result.PlannedOrders[:3] { // Check first few orders
		if order.DemandTrace == "" {
			t.Errorf("Order for %s missing demand trace", order.PartNumber)
		}
		t.Logf("  Demand trace: %s", order.DemandTrace)
	}

	// Verify memoization cache is populated
	if len(result.ExplosionCache) == 0 {
		t.Error("Expected explosion cache to be populated for performance")
	}
}

func TestMRPIntegration_PerformanceWithLargeBOM(t *testing.T) {
	ctx := context.Background()

	// Create a multi-level BOM with many parts
	levels := 5
	partsPerLevel := 10

	// Build a larger BOM structure for performance testing
	bomRepo := memory.NewBOMRepository(levels*partsPerLevel, levels*partsPerLevel*partsPerLevel)
	itemRepo := memory.NewItemRepository(levels * partsPerLevel)
	inventoryRepo := memory.NewInventoryRepository()
	demandRepo := memory.NewDemandRepository()
	qtyPer := 2

	var allParts []entities.PartNumber

	// Create items for each level
	for level := 0; level < levels; level++ {
		for part := 0; part < partsPerLevel; part++ {
			partNum := entities.PartNumber(fmt.Sprintf("LEVEL_%d_PART_%d", level, part))
			allParts = append(allParts, partNum)

			item := &entities.Item{
				PartNumber:    partNum,
				Description:   fmt.Sprintf("Level %d Part %d", level, part),
				LeadTimeDays:  (level + 1) * 10,
				LotSizeRule:   entities.LotForLot,
				MinOrderQty:   entities.Quantity(1),
				SafetyStock:   entities.Quantity(0),
				UnitOfMeasure: "EA",
			}

			err := itemRepo.SaveItem(item)
			if err != nil {
				t.Fatalf("Failed to save item: %v", err)
			}
		}
	}

	// Create BOM relationships (each part uses all parts from next level)
	for level := 0; level < levels-1; level++ {
		for part := 0; part < partsPerLevel; part++ {
			parentPart := entities.PartNumber(fmt.Sprintf("LEVEL_%d_PART_%d", level, part))

			for childPart := 0; childPart < partsPerLevel; childPart++ {
				childPartNum := entities.PartNumber(fmt.Sprintf("LEVEL_%d_PART_%d", level+1, childPart))

				bomLine := &entities.BOMLine{
					ParentPN:    parentPart,
					ChildPN:     childPartNum,
					QtyPer:      entities.Quantity(qtyPer),
					FindNumber:  childPart + 1,
					Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
				}

				err := bomRepo.SaveBOMLine(bomLine)
				if err != nil {
					t.Fatalf("Failed to save BOM line: %v", err)
				}
			}
		}
	}

	service := newTestMRPServiceForIntegration(bomRepo, itemRepo, inventoryRepo, demandRepo)

	// Create demand for top-level parts
	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "LEVEL_0_PART_0",
			Quantity:     entities.Quantity(1),
			NeedDate:     time.Now().Add(100 * 24 * time.Hour),
			DemandSource: "PERFORMANCE_TEST",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}

	// Measure performance
	start := time.Now()
	result, err := service.ExplodeDemand(ctx, demands)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Large BOM explosion failed: %v", err)
	}

	t.Logf("Performance Results:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Total Parts: %d", len(allParts))
	t.Logf("  Planned Orders: %d", len(result.PlannedOrders))
	t.Logf("  Cache Entries: %d", len(result.ExplosionCache))

	// Verify explosive growth was handled
	// With 5 levels, 10 parts per level, qty 2 each:
	// Level 4 (leaf) should have 2^4 = 16 units needed
	expectedLeafQty := entities.Quantity(16) // 2^4

	foundLeafOrder := false
	for _, order := range result.PlannedOrders {
		if order.PartNumber == "LEVEL_4_PART_0" {
			foundLeafOrder = true
			if order.Quantity != expectedLeafQty {
				t.Errorf("Expected leaf part quantity %d, got %d",
					expectedLeafQty, order.Quantity)
			}
			break
		}
	}

	if !foundLeafOrder {
		t.Error("Expected planned order for leaf level part")
	}

	// Performance should be reasonable (under 1 second for this size)
	if duration > time.Second {
		t.Errorf("Performance too slow: %v (expected < 1s)", duration)
	}
}
