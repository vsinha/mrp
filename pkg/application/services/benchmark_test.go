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

// Helper to create test MRP service for benchmarks
func newTestMRPServiceForBenchmark() *MRPService {
	config := EngineConfig{
		EnableGCPacing:  false, // Disable GC tuning in tests for predictable performance
		MaxCacheEntries: 1000,  // Smaller cache for tests
	}
	return NewMRPServiceWithConfig(config)
}

func BenchmarkMRPService_SingleLevel(b *testing.B) {
	ctx := context.Background()
	bomRepo, itemRepo, inventoryRepo, demandRepo := testhelpers.BuildSimpleTestData()
	service := newTestMRPServiceForBenchmark()

	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "ASSEMBLY_A",
			Quantity:     entities.Quantity(1),
			NeedDate:     time.Now().Add(30 * 24 * time.Hour),
			DemandSource: "BENCHMARK",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
		if err != nil {
			b.Fatalf("ExplodeDemand failed: %v", err)
		}
	}
}

func BenchmarkMRPService_DeepBOM(b *testing.B) {
	ctx := context.Background()
	bomRepo, itemRepo, inventoryRepo, demandRepo := setupDeepBOM(10) // 10 levels deep
	service := newTestMRPServiceForBenchmark()

	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "LEVEL_0",
			Quantity:     entities.Quantity(1),
			NeedDate:     time.Now().Add(100 * 24 * time.Hour),
			DemandSource: "BENCHMARK",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
		if err != nil {
			b.Fatalf("ExplodeDemand failed: %v", err)
		}
	}
}

func BenchmarkMRPService_WideBOM(b *testing.B) {
	ctx := context.Background()
	bomRepo, itemRepo, inventoryRepo, demandRepo := setupWideBOM(50) // 50 children per parent
	service := newTestMRPServiceForBenchmark()

	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "TOP_ASSEMBLY",
			Quantity:     entities.Quantity(1),
			NeedDate:     time.Now().Add(60 * 24 * time.Hour),
			DemandSource: "BENCHMARK",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
		if err != nil {
			b.Fatalf("ExplodeDemand failed: %v", err)
		}
	}
}

func BenchmarkMRPService_SerialEffectivity(b *testing.B) {
	ctx := context.Background()
	bomRepo, itemRepo, inventoryRepo, demandRepo := testhelpers.BuildAerospaceTestData()
	service := newTestMRPServiceForBenchmark()

	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "SATURN_V",
			Quantity:     entities.Quantity(1),
			NeedDate:     time.Date(2025, 8, 15, 0, 0, 0, 0, time.UTC),
			DemandSource: "BENCHMARK",
			Location:     "KENNEDY",
			TargetSerial: "AS506",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
		if err != nil {
			b.Fatalf("ExplodeDemand failed: %v", err)
		}
	}
}

func BenchmarkMRPService_LargeScale(b *testing.B) {
	ctx := context.Background()
	bomRepo, itemRepo, inventoryRepo, demandRepo := setupLargeScaleBOM(1000) // 1000 parts
	service := newTestMRPServiceForBenchmark()

	demands := []*entities.DemandRequirement{
		{
			PartNumber:   "VEHICLE_SYSTEM",
			Quantity:     entities.Quantity(1),
			NeedDate:     time.Now().Add(180 * 24 * time.Hour),
			DemandSource: "BENCHMARK",
			Location:     "FACTORY",
			TargetSerial: "SN001",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ExplodeDemand(ctx, demands, bomRepo, itemRepo, inventoryRepo, demandRepo)
		if err != nil {
			b.Fatalf("ExplodeDemand failed: %v", err)
		}
	}
}

// Helper functions for benchmark setup

func setupDeepBOM(
	levels int,
) (*memory.BOMRepository, *memory.ItemRepository, *memory.InventoryRepository, *memory.DemandRepository) {
	bomRepo := memory.NewBOMRepository(levels*2, levels*2)
	itemRepo := memory.NewItemRepository(levels * 2)
	inventoryRepo := memory.NewInventoryRepository()
	demandRepo := memory.NewDemandRepository()

	// Create items for each level
	for level := 0; level < levels; level++ {
		partNum := entities.PartNumber(fmt.Sprintf("LEVEL_%d", level))
		item := &entities.Item{
			PartNumber:    partNum,
			Description:   fmt.Sprintf("Level %d Part", level),
			LeadTimeDays:  level + 1,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		}

		err := itemRepo.SaveItem(item)
		if err != nil {
			panic(err)
		}

		// Create BOM relationship to next level
		if level < levels-1 {
			childPartNum := entities.PartNumber(fmt.Sprintf("LEVEL_%d", level+1))
			bomLine := &entities.BOMLine{
				ParentPN:    partNum,
				ChildPN:     childPartNum,
				QtyPer:      entities.Quantity(2),
				FindNumber:  100,
				Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
			}

			err := bomRepo.SaveBOMLine(bomLine)
			if err != nil {
				panic(err)
			}
		}
	}

	return bomRepo, itemRepo, inventoryRepo, demandRepo
}

func setupWideBOM(
	childrenCount int,
) (*memory.BOMRepository, *memory.ItemRepository, *memory.InventoryRepository, *memory.DemandRepository) {
	bomRepo := memory.NewBOMRepository(childrenCount+1, childrenCount)
	itemRepo := memory.NewItemRepository(childrenCount + 1)
	inventoryRepo := memory.NewInventoryRepository()
	demandRepo := memory.NewDemandRepository()

	// Create top assembly
	topItem := &entities.Item{
		PartNumber:    "TOP_ASSEMBLY",
		Description:   "Top Level Assembly",
		LeadTimeDays:  30,
		LotSizeRule:   entities.LotForLot,
		MinOrderQty:   entities.Quantity(1),
		SafetyStock:   entities.Quantity(0),
		UnitOfMeasure: "EA",
	}

	err := itemRepo.SaveItem(topItem)
	if err != nil {
		panic(err)
	}

	// Create many child components
	for i := 0; i < childrenCount; i++ {
		childPartNum := entities.PartNumber(fmt.Sprintf("COMPONENT_%d", i))
		childItem := &entities.Item{
			PartNumber:    childPartNum,
			Description:   fmt.Sprintf("Component %d", i),
			LeadTimeDays:  10,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		}

		err := itemRepo.SaveItem(childItem)
		if err != nil {
			panic(err)
		}

		// Create BOM relationship
		bomLine := &entities.BOMLine{
			ParentPN:    "TOP_ASSEMBLY",
			ChildPN:     childPartNum,
			QtyPer:      entities.Quantity(1),
			FindNumber:  i + 100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		}

		err = bomRepo.SaveBOMLine(bomLine)
		if err != nil {
			panic(err)
		}
	}

	return bomRepo, itemRepo, inventoryRepo, demandRepo
}

func setupLargeScaleBOM(
	totalParts int,
) (*memory.BOMRepository, *memory.ItemRepository, *memory.InventoryRepository, *memory.DemandRepository) {
	bomRepo := memory.NewBOMRepository(totalParts, totalParts*2)
	itemRepo := memory.NewItemRepository(totalParts)
	inventoryRepo := memory.NewInventoryRepository()
	demandRepo := memory.NewDemandRepository()

	// Create a realistic hierarchy: 1 vehicle, 10 systems, 100 assemblies, rest components
	levels := []struct {
		prefix string
		count  int
	}{
		{"VEHICLE", 1},
		{"SYSTEM", 10},
		{"ASSEMBLY", 100},
		{"COMPONENT", totalParts - 111},
	}

	var prevLevelParts []entities.PartNumber

	for levelIdx, level := range levels {
		var currentLevelParts []entities.PartNumber

		for i := 0; i < level.count; i++ {
			partNum := entities.PartNumber(fmt.Sprintf("%s_%d", level.prefix, i))
			currentLevelParts = append(currentLevelParts, partNum)

			item := &entities.Item{
				PartNumber:    partNum,
				Description:   fmt.Sprintf("%s %d", level.prefix, i),
				LeadTimeDays:  (levelIdx + 1) * 15,
				LotSizeRule:   entities.LotForLot,
				MinOrderQty:   entities.Quantity(1),
				SafetyStock:   entities.Quantity(0),
				UnitOfMeasure: "EA",
			}

			err := itemRepo.SaveItem(item)
			if err != nil {
				panic(err)
			}
		}

		// Create BOM relationships from previous level
		if levelIdx > 0 {
			childrenPerParent := len(currentLevelParts) / len(prevLevelParts)
			if childrenPerParent == 0 {
				childrenPerParent = 1
			}

			for i, parentPart := range prevLevelParts {
				startIdx := i * childrenPerParent
				endIdx := startIdx + childrenPerParent
				if endIdx > len(currentLevelParts) {
					endIdx = len(currentLevelParts)
				}

				for j := startIdx; j < endIdx; j++ {
					bomLine := &entities.BOMLine{
						ParentPN:    parentPart,
						ChildPN:     currentLevelParts[j],
						QtyPer:      entities.Quantity(1),
						FindNumber:  j + 100,
						Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
					}

					err := bomRepo.SaveBOMLine(bomLine)
					if err != nil {
						panic(err)
					}
				}
			}
		}

		prevLevelParts = currentLevelParts
	}

	return bomRepo, itemRepo, inventoryRepo, demandRepo
}
