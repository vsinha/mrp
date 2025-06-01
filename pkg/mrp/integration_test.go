package mrp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestMRPIntegration_AerospaceScenario(t *testing.T) {
	ctx := context.Background()
	
	// Build aerospace test data
	bomRepo, inventoryRepo := buildAerospaceTestData()
	
	// Create MRP engine
	engine := NewEngine(bomRepo, inventoryRepo)
	
	// Define complex multi-vehicle demand scenario
	needDate := time.Date(2025, 8, 15, 0, 0, 0, 0, time.UTC)
	demands := []DemandRequirement{
		// New vehicle build - uses newer BOM effectivity
		{
			PartNumber:   "FALCON_9_BLOCK5",
			Quantity:     Quantity(decimal.NewFromInt(1)),
			NeedDate:     needDate,
			DemandSource: "MISSION_STARLINK_001",
			Location:     "HAWTHORNE",
			TargetSerial: "SN075", // Uses MERLIN_VAC_V2, TURBOPUMP_V2
		},
		// Refurbishment of older vehicle - uses older BOM effectivity  
		{
			PartNumber:   "MERLIN_ENGINE_1D",
			Quantity:     Quantity(decimal.NewFromInt(9)),
			NeedDate:     time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC), 
			DemandSource: "REFURB_SN020",
			Location:     "MCGREGOR",
			TargetSerial: "SN020", // Uses TURBOPUMP_V1 for refurb compatibility
		},
		// Spare parts for test campaign
		{
			PartNumber:   "TURBOPUMP_ASSEMBLY_V2",
			Quantity:     Quantity(decimal.NewFromInt(4)),
			NeedDate:     time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC),
			DemandSource: "TEST_CAMPAIGN_Q3",
			Location:     "MCGREGOR", 
			TargetSerial: "SN999", // Special test serial for latest config
		},
	}
	
	// Execute MRP
	result, err := engine.ExplodeDemand(ctx, demands)
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
	foundMerlinVacV2 := false
	foundTurbopumpV1 := false
	foundTurbopumpV2 := false
	
	for _, order := range result.PlannedOrders {
		switch order.PartNumber {
		case "MERLIN_VAC_V2":
			foundMerlinVacV2 = true
			t.Logf("  Found MERLIN_VAC_V2 order for target serial %s", order.TargetSerial)
		case "TURBOPUMP_ASSEMBLY_V1":
			foundTurbopumpV1 = true
			t.Logf("  Found TURBOPUMP_V1 order qty=%s for target serial %s", 
				order.Quantity.Decimal(), order.TargetSerial)
		case "TURBOPUMP_ASSEMBLY_V2":
			foundTurbopumpV2 = true
			t.Logf("  Found TURBOPUMP_V2 order qty=%s for target serial %s", 
				order.Quantity.Decimal(), order.TargetSerial)
		}
	}
	
	// Verify correct effectivity resolution
	if !foundMerlinVacV2 {
		t.Error("Expected MERLIN_VAC_V2 for SN075 (late serial)")
	}
	if !foundTurbopumpV1 {
		t.Error("Expected TURBOPUMP_V1 for SN020 refurb (early serial)")
	}
	if !foundTurbopumpV2 {
		t.Error("Expected TURBOPUMP_V2 for SN075 and SN999 (late serials)")
	}
	
	// Verify inventory allocation
	engineAllocation := false
	for _, alloc := range result.Allocations {
		if alloc.PartNumber == "MERLIN_ENGINE_1D" {
			engineAllocation = true
			t.Logf("  Engine allocation: %s units allocated from inventory", 
				alloc.AllocatedQty.Decimal())
			break
		}
	}
	
	if !engineAllocation {
		t.Error("Expected inventory allocation for engines")
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
	
	// Verify realistic quantities
	totalTurbopumpOrders := decimal.Zero
	for _, order := range result.PlannedOrders {
		if order.PartNumber == "TURBOPUMP_ASSEMBLY_V1" || order.PartNumber == "TURBOPUMP_ASSEMBLY_V2" {
			totalTurbopumpOrders = totalTurbopumpOrders.Add(order.Quantity.Decimal())
		}
	}
	
	// Should have turbopumps for:
	// - 9 engines (SN075 vehicle) * 2 pumps = 18 V2 pumps
	// - 9 engines (SN020 refurb) * 2 pumps = 18 V1 pumps  
	// - 4 spares (direct demand) = 4 V2 pumps
	// Total expected: 40 turbopumps
	expectedTurbopumps := decimal.NewFromInt(40)
	if totalTurbopumpOrders.Cmp(expectedTurbopumps) != 0 {
		t.Errorf("Expected %s total turbopump orders, got %s", 
			expectedTurbopumps, totalTurbopumpOrders)
	}
}

func TestMRPIntegration_PerformanceWithLargeBOM(t *testing.T) {
	ctx := context.Background()
	
	// Build a larger BOM structure for performance testing
	bomRepo := NewInMemoryBOMRepository()
	inventoryRepo := NewInMemoryInventoryRepository()
	
	// Create a multi-level BOM with many parts
	levels := 5
	partsPerLevel := 10
	qtyPer := 2
	
	var allParts []PartNumber
	
	// Create items for each level
	for level := 0; level < levels; level++ {
		for part := 0; part < partsPerLevel; part++ {
			partNum := PartNumber(fmt.Sprintf("LEVEL_%d_PART_%d", level, part))
			allParts = append(allParts, partNum)
			
			bomRepo.AddItem(Item{
				PartNumber:      partNum,
				Description:     fmt.Sprintf("Level %d Part %d", level, part),
				LeadTimeDays:    (level + 1) * 10,
				LotSizeRule:     LotForLot,
				MinOrderQty:     Quantity(decimal.NewFromInt(1)),
				SafetyStock:     Quantity(decimal.Zero),
				UnitOfMeasure:   "EA",
			})
		}
	}
	
	// Create BOM relationships (each part uses all parts from next level)
	for level := 0; level < levels-1; level++ {
		for part := 0; part < partsPerLevel; part++ {
			parentPart := PartNumber(fmt.Sprintf("LEVEL_%d_PART_%d", level, part))
			
			for childPart := 0; childPart < partsPerLevel; childPart++ {
				childPartNum := PartNumber(fmt.Sprintf("LEVEL_%d_PART_%d", level+1, childPart))
				
				bomRepo.AddBOMLine(BOMLine{
					ParentPN:     parentPart,
					ChildPN:      childPartNum,
					QtyPer:       Quantity(decimal.NewFromInt(int64(qtyPer))),
					FindNumber:   childPart + 1,
					Effectivity:  SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
				})
			}
		}
	}
	
	engine := NewEngine(bomRepo, inventoryRepo)
	
	// Create demand for top-level parts
	demands := []DemandRequirement{
		{
			PartNumber:   "LEVEL_0_PART_0",
			Quantity:     Quantity(decimal.NewFromInt(1)),
			NeedDate:     time.Now().Add(100 * 24 * time.Hour),
			DemandSource: "PERFORMANCE_TEST",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}
	
	// Measure performance
	start := time.Now()
	result, err := engine.ExplodeDemand(ctx, demands)
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
	expectedLeafQty := decimal.NewFromInt(16) // 2^4
	
	foundLeafOrder := false
	for _, order := range result.PlannedOrders {
		if order.PartNumber == "LEVEL_4_PART_0" {
			foundLeafOrder = true
			if order.Quantity.Decimal().Cmp(expectedLeafQty) != 0 {
				t.Errorf("Expected leaf part quantity %s, got %s", 
					expectedLeafQty, order.Quantity.Decimal())
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

// Helper function to format test results
func formatDecimal(d decimal.Decimal) string {
	return d.String()
}