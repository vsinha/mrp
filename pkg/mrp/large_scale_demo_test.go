package mrp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// TestLargeScaleAerospaceBOM demonstrates the full capabilities of the MRP system with large BOMs
func TestLargeScaleAerospaceBOM(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large scale test in short mode")
	}
	
	ctx := context.Background()
	
	// Configuration for a realistic aerospace BOM
	config := LargeBOMConfig{
		TotalParts:         30000,  // 30,000 unique parts
		MaxLevels:          10,     // 10 levels of BOM nesting
		AvgChildrenPerPart: 8.0,    // Average 8 children per parent
		SerialRanges:       25,     // 25 different serial effectivity ranges
		InventoryRatio:     0.15,   // 15% of parts have inventory
		LeadTimeVariation:  45,     // ±45 days lead time variation
	}
	
	t.Logf("🚀 Testing Large-Scale Aerospace MRP System")
	t.Logf("Configuration: %d parts, %d levels, %.1f avg children per part", 
		config.TotalParts, config.MaxLevels, config.AvgChildrenPerPart)
	
	// Generate the BOM
	startTime := time.Now()
	synthesizer := NewLargeBOMSynthesizer(config)
	bomRepo, inventoryRepo := synthesizer.SynthesizeAerospaceBOM()
	synthesisTime := time.Since(startTime)
	
	t.Logf("✅ BOM synthesis completed in %v", synthesisTime)
	
	// Verify BOM structure
	allItems, err := bomRepo.GetAllItems(ctx)
	if err != nil {
		t.Fatalf("Failed to get all items: %v", err)
	}
	
	allBOMLines, err := bomRepo.GetAllBOMLines(ctx)
	if err != nil {
		t.Fatalf("Failed to get all BOM lines: %v", err)
	}
	
	t.Logf("📊 BOM Statistics:")
	t.Logf("  Total Items: %d", len(allItems))
	t.Logf("  Total BOM Lines: %d", len(allBOMLines))
	t.Logf("  Average BOM Lines per Part: %.2f", float64(len(allBOMLines))/float64(len(allItems)))
	
	// Test with standard engine
	t.Run("Standard_Engine_Performance", func(t *testing.T) {
		engine := NewTestEngine(bomRepo, inventoryRepo)
		
		demands := []DemandRequirement{
			{
				PartNumber:   "L0_ASM_000000", // Top-level assembly
				Quantity:     Quantity(decimal.NewFromInt(3)), // Multiple units
				NeedDate:     time.Now().Add(200 * 24 * time.Hour),
				DemandSource: "LARGE_SCALE_TEST",
				Location:     "HUNTSVILLE",
				TargetSerial: "SN150",
			},
			{
				PartNumber:   "L0_ASM_000001", // Another top-level assembly
				Quantity:     Quantity(decimal.NewFromInt(2)),
				NeedDate:     time.Now().Add(180 * 24 * time.Hour),
				DemandSource: "LARGE_SCALE_TEST_2",
				Location:     "KENNEDY",
				TargetSerial: "SN075", // Different serial for effectivity testing
			},
		}
		
		startTime := time.Now()
		result, err := engine.ExplodeDemand(ctx, demands)
		explosionTime := time.Since(startTime)
		
		if err != nil {
			t.Fatalf("MRP explosion failed: %v", err)
		}
		
		t.Logf("🎯 Standard Engine Results (in %v):", explosionTime)
		t.Logf("  Planned Orders: %d", len(result.PlannedOrders))
		t.Logf("  Inventory Allocations: %d", len(result.Allocations))
		t.Logf("  Material Shortages: %d", len(result.ShortageReport))
		t.Logf("  Cache Entries: %d", len(result.ExplosionCache))
		
		// Verify explosion reached deep levels
		maxLevel := 0
		for _, order := range result.PlannedOrders {
			// Extract level from part number (format: L{level}_{type}_{number})
			partStr := string(order.PartNumber)
			if len(partStr) > 2 && partStr[0] == 'L' {
				level := int(partStr[1] - '0')
				if level > maxLevel {
					maxLevel = level
				}
			}
		}
		t.Logf("  Maximum BOM Level Reached: %d", maxLevel)
		
		// Verify serial effectivity worked
		serialEffectivityWorked := false
		for _, order := range result.PlannedOrders {
			if order.TargetSerial == "SN075" || order.TargetSerial == "SN150" {
				serialEffectivityWorked = true
				break
			}
		}
		
		if !serialEffectivityWorked {
			t.Error("Serial effectivity tracking not working correctly")
		}
		
		// Performance assertions
		if explosionTime > 10*time.Second {
			t.Errorf("MRP explosion took too long: %v (expected < 10s)", explosionTime)
		}
		
		if len(result.PlannedOrders) == 0 {
			t.Error("Expected planned orders to be generated")
		}
		
		if maxLevel < 7 {
			t.Errorf("Expected BOM explosion to reach at least level 7, got %d", maxLevel)
		}
	})
	
	// Test with optimized engine
	t.Run("Optimized_Engine_Performance", func(t *testing.T) {
		bomRepo := NewBOMRepository(len(allItems), len(allBOMLines))
		
		for _, item := range allItems {
			bomRepo.AddItem(item)
		}
		for _, line := range allBOMLines {
			bomRepo.AddBOMLine(line)
		}
		
		engineConfig := EngineConfig{
			EnableGCPacing:  true,
			MaxCacheEntries: 8000,
		}
		
		engine := NewEngineWithConfig(bomRepo, inventoryRepo, engineConfig)
		
		demands := []DemandRequirement{
			{
				PartNumber:   "L0_ASM_000000",
				Quantity:     Quantity(decimal.NewFromInt(5)), // Even more units
				NeedDate:     time.Now().Add(200 * 24 * time.Hour),
				DemandSource: "OPTIMIZED_LARGE_SCALE_TEST",
				Location:     "HUNTSVILLE",
				TargetSerial: "SN200",
			},
		}
		
		startTime := time.Now()
		result, err := engine.ExplodeDemand(ctx, demands)
		explosionTime := time.Since(startTime)
		
		if err != nil {
			t.Fatalf("Optimized MRP explosion failed: %v", err)
		}
		
		t.Logf("⚡ Optimized Engine Results (in %v):", explosionTime)
		t.Logf("  Planned Orders: %d", len(result.PlannedOrders))
		t.Logf("  Inventory Allocations: %d", len(result.Allocations))
		t.Logf("  Material Shortages: %d", len(result.ShortageReport))
		t.Logf("  Cache Entries: %d", len(result.ExplosionCache))
		
		// Performance assertions for optimized engine
		if explosionTime > 8*time.Second {
			t.Errorf("Optimized MRP explosion took too long: %v (expected < 8s)", explosionTime)
		}
	})
	
	// Test memory usage
	t.Run("Memory_Usage_Analysis", func(t *testing.T) {
		// Get initial memory stats
		initialStats := GetMemoryStats()
		
		engine := NewTestEngine(bomRepo, inventoryRepo)
		demands := []DemandRequirement{
			{
				PartNumber:   "L0_ASM_000002",
				Quantity:     Quantity(decimal.NewFromInt(1)),
				NeedDate:     time.Now().Add(150 * 24 * time.Hour),
				DemandSource: "MEMORY_TEST",
				Location:     "WALLOPS",
				TargetSerial: "SN100",
			},
		}
		
		_, err := engine.ExplodeDemand(ctx, demands)
		if err != nil {
			t.Fatalf("Memory test MRP explosion failed: %v", err)
		}
		
		finalStats := GetMemoryStats()
		
		t.Logf("💾 Memory Usage Analysis:")
		t.Logf("  Initial Memory: %s", FormatBytes(initialStats.AllocBytes))
		t.Logf("  Final Memory: %s", FormatBytes(finalStats.AllocBytes))
		var memoryChange string
		if finalStats.AllocBytes >= initialStats.AllocBytes {
			memoryChange = "+" + FormatBytes(finalStats.AllocBytes-initialStats.AllocBytes)
		} else {
			memoryChange = "-" + FormatBytes(initialStats.AllocBytes-finalStats.AllocBytes)
		}
		t.Logf("  Memory Change: %s", memoryChange)
		t.Logf("  Total Allocations: %s", FormatBytes(finalStats.TotalAllocBytes-initialStats.TotalAllocBytes))
		
		// Memory should be reasonable for 30K parts
		// Use total allocations instead of current memory to avoid GC effects
		totalAllocations := finalStats.TotalAllocBytes - initialStats.TotalAllocBytes
		if totalAllocations > 1024*1024*1024 { // 1GB total allocations is reasonable
			t.Errorf("Total allocations too high: %s (expected < 1GB)", FormatBytes(totalAllocations))
		}
	})
	
	// Test concurrent access
	t.Run("Concurrent_Access", func(t *testing.T) {
		engine := NewTestEngine(bomRepo, inventoryRepo)
		
		// Run multiple MRP explosions concurrently
		type result struct {
			orders int
			err    error
		}
		
		results := make(chan result, 3)
		
		for i := 0; i < 3; i++ {
			go func(id int) {
				demands := []DemandRequirement{
					{
						PartNumber:   PartNumber(fmt.Sprintf("L0_ASM_%06d", id)),
						Quantity:     Quantity(decimal.NewFromInt(1)),
						NeedDate:     time.Now().Add(time.Duration(120+id*10) * 24 * time.Hour),
						DemandSource: fmt.Sprintf("CONCURRENT_TEST_%d", id),
						Location:     "KENNEDY",
						TargetSerial: fmt.Sprintf("SN%03d", 100+id),
					},
				}
				
				mrpResult, err := engine.ExplodeDemand(ctx, demands)
				if err != nil {
					results <- result{0, err}
					return
				}
				
				results <- result{len(mrpResult.PlannedOrders), nil}
			}(i)
		}
		
		// Collect results
		for i := 0; i < 3; i++ {
			res := <-results
			if res.err != nil {
				t.Errorf("Concurrent MRP run %d failed: %v", i, res.err)
			} else {
				t.Logf("Concurrent run %d generated %d planned orders", i, res.orders)
			}
		}
	})
	
	t.Logf("🏁 Large-Scale Test Suite Completed Successfully")
}