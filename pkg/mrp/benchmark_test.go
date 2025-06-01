package mrp

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func BenchmarkMRPEngine_SingleLevel(b *testing.B) {
	ctx := context.Background()
	bomRepo, inventoryRepo := setupSimpleBOM()
	engine := NewTestEngine(bomRepo, inventoryRepo)
	
	demands := []DemandRequirement{
		{
			PartNumber:   "ASSEMBLY_A",
			Quantity:     Quantity(1),
			NeedDate:     time.Now().Add(30 * 24 * time.Hour),
			DemandSource: "BENCHMARK",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.ExplodeDemand(ctx, demands)
		if err != nil {
			b.Fatalf("ExplodeDemand failed: %v", err)
		}
	}
}

func BenchmarkMRPEngine_DeepBOM(b *testing.B) {
	ctx := context.Background()
	bomRepo, inventoryRepo := setupDeepBOM(10) // 10 levels deep
	engine := NewTestEngine(bomRepo, inventoryRepo)
	
	demands := []DemandRequirement{
		{
			PartNumber:   "LEVEL_0",
			Quantity:     Quantity(1),
			NeedDate:     time.Now().Add(100 * 24 * time.Hour),
			DemandSource: "BENCHMARK",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.ExplodeDemand(ctx, demands)
		if err != nil {
			b.Fatalf("ExplodeDemand failed: %v", err)
		}
	}
}

func BenchmarkMRPEngine_WideBOM(b *testing.B) {
	ctx := context.Background()
	bomRepo, inventoryRepo := setupWideBOM(50) // 50 children per parent
	engine := NewTestEngine(bomRepo, inventoryRepo)
	
	demands := []DemandRequirement{
		{
			PartNumber:   "TOP_ASSEMBLY",
			Quantity:     Quantity(1),
			NeedDate:     time.Now().Add(60 * 24 * time.Hour),
			DemandSource: "BENCHMARK",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.ExplodeDemand(ctx, demands)
		if err != nil {
			b.Fatalf("ExplodeDemand failed: %v", err)
		}
	}
}

func BenchmarkMRPEngine_SerialEffectivity(b *testing.B) {
	ctx := context.Background()
	bomRepo, inventoryRepo := buildAerospaceTestData()
	engine := NewTestEngine(bomRepo, inventoryRepo)
	
	demands := []DemandRequirement{
		{
			PartNumber:   "SATURN_V",
			Quantity:     Quantity(1),
			NeedDate:     time.Date(2025, 8, 15, 0, 0, 0, 0, time.UTC),
			DemandSource: "BENCHMARK",
			Location:     "KENNEDY",
			TargetSerial: "AS506",
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.ExplodeDemand(ctx, demands)
		if err != nil {
			b.Fatalf("ExplodeDemand failed: %v", err)
		}
	}
}

// Helper functions for benchmark setups

func setupSimpleBOM() (*BOMRepository, *InventoryRepository) {
	bomRepo := NewTestBOMRepository()
	inventoryRepo := NewInMemoryInventoryRepository()
	
	bomRepo.AddItem(Item{
		PartNumber:      "ASSEMBLY_A",
		Description:     "Simple Assembly",
		LeadTimeDays:    30,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(1),
		SafetyStock:     Quantity(0),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddItem(Item{
		PartNumber:      "PART_B",
		Description:     "Component B",
		LeadTimeDays:    15,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(1),
		SafetyStock:     Quantity(0),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddBOMLine(BOMLine{
		ParentPN:     "ASSEMBLY_A",
		ChildPN:      "PART_B",
		QtyPer:       Quantity(2),
		FindNumber:   100,
		Effectivity:  SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
	})
	
	return bomRepo, inventoryRepo
}

func setupDeepBOM(levels int) (*BOMRepository, *InventoryRepository) {
	bomRepo := NewBOMRepository(levels, levels-1) // levels items, levels-1 BOM lines
	inventoryRepo := NewInMemoryInventoryRepository()
	
	// Create items for each level
	for level := 0; level < levels; level++ {
		partNum := PartNumber(fmt.Sprintf("LEVEL_%d", level))
		bomRepo.AddItem(Item{
			PartNumber:      partNum,
			Description:     fmt.Sprintf("Level %d Part", level),
			LeadTimeDays:    (level + 1) * 5,
			LotSizeRule:     LotForLot,
			MinOrderQty:     Quantity(1),
			SafetyStock:     Quantity(0),
			UnitOfMeasure:   "EA",
		})
		
		// Create BOM relationship to next level
		if level < levels-1 {
			childPartNum := PartNumber(fmt.Sprintf("LEVEL_%d", level+1))
			bomRepo.AddBOMLine(BOMLine{
				ParentPN:     partNum,
				ChildPN:      childPartNum,
				QtyPer:       Quantity(2),
				FindNumber:   100,
				Effectivity:  SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
			})
		}
	}
	
	return bomRepo, inventoryRepo
}

func setupWideBOM(width int) (*BOMRepository, *InventoryRepository) {
	bomRepo := NewBOMRepository(width+1, width) // width+1 items (including top assembly), width BOM lines
	inventoryRepo := NewInMemoryInventoryRepository()
	
	// Create top assembly
	bomRepo.AddItem(Item{
		PartNumber:      "TOP_ASSEMBLY",
		Description:     "Top Level Assembly",
		LeadTimeDays:    60,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(1),
		SafetyStock:     Quantity(0),
		UnitOfMeasure:   "EA",
	})
	
	// Create many child parts
	for i := 0; i < width; i++ {
		partNum := PartNumber(fmt.Sprintf("CHILD_PART_%d", i))
		bomRepo.AddItem(Item{
			PartNumber:      partNum,
			Description:     fmt.Sprintf("Child Part %d", i),
			LeadTimeDays:    30,
			LotSizeRule:     LotForLot,
			MinOrderQty:     Quantity(1),
			SafetyStock:     Quantity(0),
			UnitOfMeasure:   "EA",
		})
		
		bomRepo.AddBOMLine(BOMLine{
			ParentPN:     "TOP_ASSEMBLY",
			ChildPN:      partNum,
			QtyPer:       Quantity(1),
			FindNumber:   i + 1,
			Effectivity:  SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		})
	}
	
	return bomRepo, inventoryRepo
}