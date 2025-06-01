package main

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/vsinha/mrp/pkg/mrp"
)

func main() {
	ctx := context.Background()
	
	// Create repositories
	bomRepo := mrp.NewInMemoryBOMRepository()
	inventoryRepo := mrp.NewInMemoryInventoryRepository()
	
	// Set up a simple rocket engine BOM
	setupRocketEngineBOM(bomRepo, inventoryRepo)
	
	// Create MRP engine
	engine := mrp.NewEngine(bomRepo, inventoryRepo)
	
	// Define demand for a rocket launch
	needDate := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	demands := []mrp.DemandRequirement{
		{
			PartNumber:   "ROCKET_ENGINE",
			Quantity:     mrp.Quantity(decimal.NewFromInt(9)), // 9 engines for first stage
			NeedDate:     needDate,
			DemandSource: "MISSION_MARS_001",
			Location:     "LAUNCH_PAD_39A",
			TargetSerial: "SN100", // Future serial using latest config
		},
	}
	
	fmt.Println("🚀 Running MRP for Mars Mission...")
	fmt.Printf("Demand: %d engines needed by %s\n", 
		demands[0].Quantity.Decimal().IntPart(), needDate.Format("2006-01-02"))
	fmt.Printf("Target Serial: %s\n", demands[0].TargetSerial)
	fmt.Println()
	
	// Execute MRP
	result, err := engine.ExplodeDemand(ctx, demands)
	if err != nil {
		fmt.Printf("❌ MRP failed: %v\n", err)
		return
	}
	
	// Display results
	fmt.Println("📊 MRP Results:")
	fmt.Printf("  Planned Orders: %d\n", len(result.PlannedOrders))
	fmt.Printf("  Inventory Allocations: %d\n", len(result.Allocations))
	fmt.Printf("  Shortages: %d\n", len(result.ShortageReport))
	fmt.Printf("  Cache Entries: %d\n", len(result.ExplosionCache))
	fmt.Println()
	
	// Show planned orders
	if len(result.PlannedOrders) > 0 {
		fmt.Println("📝 Planned Manufacturing Orders:")
		for _, order := range result.PlannedOrders {
			fmt.Printf("  %s: %s units (Due: %s)\n", 
				order.PartNumber, 
				order.Quantity.Decimal().String(),
				order.DueDate.Format("2006-01-02"))
			fmt.Printf("    Type: %s | Trace: %s\n", 
				order.OrderType.String(), order.DemandTrace)
		}
		fmt.Println()
	}
	
	// Show inventory allocations
	if len(result.Allocations) > 0 {
		fmt.Println("📦 Inventory Allocations:")
		for _, alloc := range result.Allocations {
			fmt.Printf("  %s: %s units allocated\n", 
				alloc.PartNumber, alloc.AllocatedQty.Decimal().String())
			if alloc.RemainingDemand.Decimal().IsPositive() {
				fmt.Printf("    ⚠️  Still need: %s units\n", 
					alloc.RemainingDemand.Decimal().String())
			}
		}
		fmt.Println()
	}
	
	// Show shortages
	if len(result.ShortageReport) > 0 {
		fmt.Println("🚨 Material Shortages:")
		for _, shortage := range result.ShortageReport {
			fmt.Printf("  %s: %s units short (Need by: %s)\n", 
				shortage.PartNumber, 
				shortage.ShortQty.Decimal().String(),
				shortage.NeedDate.Format("2006-01-02"))
		}
		fmt.Println()
	}
	
	fmt.Println("✅ MRP analysis complete!")
}

func setupRocketEngineBOM(bomRepo *mrp.InMemoryBOMRepository, inventoryRepo *mrp.InMemoryInventoryRepository) {
	// Add items
	bomRepo.AddItem(mrp.Item{
		PartNumber:      "ROCKET_ENGINE",
		Description:     "Main Rocket Engine Assembly",
		LeadTimeDays:    120,
		LotSizeRule:     mrp.LotForLot,
		MinOrderQty:     mrp.Quantity(decimal.NewFromInt(1)),
		SafetyStock:     mrp.Quantity(decimal.Zero),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddItem(mrp.Item{
		PartNumber:      "TURBOPUMP_V3",
		Description:     "Turbopump Assembly V3 (Latest)",
		LeadTimeDays:    60,
		LotSizeRule:     mrp.LotForLot,
		MinOrderQty:     mrp.Quantity(decimal.NewFromInt(1)),
		SafetyStock:     mrp.Quantity(decimal.Zero),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddItem(mrp.Item{
		PartNumber:      "COMBUSTION_CHAMBER",
		Description:     "Main Combustion Chamber",
		LeadTimeDays:    90,
		LotSizeRule:     mrp.LotForLot,
		MinOrderQty:     mrp.Quantity(decimal.NewFromInt(1)),
		SafetyStock:     mrp.Quantity(decimal.Zero),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddItem(mrp.Item{
		PartNumber:      "VALVE_ASSEMBLY",
		Description:     "Main Valve Assembly",
		LeadTimeDays:    45,
		LotSizeRule:     mrp.MinimumQty,
		MinOrderQty:     mrp.Quantity(decimal.NewFromInt(10)),
		SafetyStock:     mrp.Quantity(decimal.NewFromInt(5)),
		UnitOfMeasure:   "EA",
	})
	
	// Add BOM structure - Engine uses latest turbopump for SN100+
	bomRepo.AddBOMLine(mrp.BOMLine{
		ParentPN:     "ROCKET_ENGINE",
		ChildPN:      "TURBOPUMP_V3",
		QtyPer:       mrp.Quantity(decimal.NewFromInt(2)), // 2 turbopumps per engine
		FindNumber:   100,
		Effectivity:  mrp.SerialEffectivity{FromSerial: "SN050", ToSerial: ""}, // Latest config
	})
	
	bomRepo.AddBOMLine(mrp.BOMLine{
		ParentPN:     "ROCKET_ENGINE",
		ChildPN:      "COMBUSTION_CHAMBER",
		QtyPer:       mrp.Quantity(decimal.NewFromInt(1)),
		FindNumber:   200,
		Effectivity:  mrp.SerialEffectivity{FromSerial: "SN001", ToSerial: ""}, // All configs
	})
	
	bomRepo.AddBOMLine(mrp.BOMLine{
		ParentPN:     "ROCKET_ENGINE",
		ChildPN:      "VALVE_ASSEMBLY",
		QtyPer:       mrp.Quantity(decimal.NewFromInt(4)), // 4 valves per engine
		FindNumber:   300,
		Effectivity:  mrp.SerialEffectivity{FromSerial: "SN001", ToSerial: ""}, // All configs
	})
	
	// Add some inventory
	baseDate := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	
	// Have 2 engines in stock
	inventoryRepo.AddSerializedInventory(mrp.SerializedInventory{
		PartNumber:   "ROCKET_ENGINE",
		SerialNumber: "E9001",
		Location:     "LAUNCH_PAD_39A",
		Status:       mrp.Available,
		ReceiptDate:  baseDate,
	})
	
	inventoryRepo.AddSerializedInventory(mrp.SerializedInventory{
		PartNumber:   "ROCKET_ENGINE",
		SerialNumber: "E9002",
		Location:     "LAUNCH_PAD_39A",
		Status:       mrp.Available,
		ReceiptDate:  baseDate.Add(5 * 24 * time.Hour),
	})
	
	// Have some valves in stock
	inventoryRepo.AddLotInventory(mrp.InventoryLot{
		PartNumber:   "VALVE_ASSEMBLY",
		LotNumber:    "VALVE_LOT_001",
		Location:     "LAUNCH_PAD_39A",
		Quantity:     mrp.Quantity(decimal.NewFromInt(15)),
		ReceiptDate:  baseDate.Add(-30 * 24 * time.Hour),
		Status:       mrp.Available,
	})
}