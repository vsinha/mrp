package criticalpath

import (
	"context"
	"testing"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/infrastructure/repositories/memory"
)

func TestCriticalPathService_Basic(t *testing.T) {
	ctx := context.Background()

	// Build a simple test BOM
	bomRepo, itemRepo, inventoryRepo := buildSimpleTestData()

	// Create critical path service
	service := NewCriticalPathService(bomRepo, itemRepo, inventoryRepo, nil)

	// Analyze critical path for a simple assembly
	analysis, err := service.AnalyzeCriticalPath(
		ctx,
		"SIMPLE_ASSEMBLY",
		"SN001",
		"FACTORY",
		3,
	)
	if err != nil {
		t.Fatalf("Critical path analysis failed: %v", err)
	}

	// Verify we found paths
	if len(analysis.TopPaths) == 0 {
		t.Fatal("Expected to find at least one critical path")
	}

	t.Logf("Critical Path Analysis Results:")
	t.Logf("  Total paths analyzed: %d", analysis.TotalPaths)
	t.Logf("  Critical path lead time: %d days", analysis.CriticalPath.TotalLeadTime)

	// Verify critical path
	criticalPath := analysis.CriticalPath
	if criticalPath.TotalLeadTime <= 0 {
		t.Error("Expected positive total lead time")
	}

	if len(criticalPath.Path) == 0 {
		t.Error("Expected non-empty critical path")
	}
}

// buildSimpleTestData creates minimal test data for unit tests
func buildSimpleTestData() (*memory.BOMRepository, *memory.ItemRepository, *memory.InventoryRepository) {
	bomRepo := memory.NewBOMRepository(2)
	itemRepo := memory.NewItemRepository(3)
	inventoryRepo := memory.NewInventoryRepository()

	// Add items
	items := []*entities.Item{
		{
			PartNumber:    "SIMPLE_ASSEMBLY",
			Description:   "Simple Assembly",
			LeadTimeDays:  30,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "COMPONENT_A",
			Description:   "Component A",
			LeadTimeDays:  20,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "COMPONENT_B",
			Description:   "Component B",
			LeadTimeDays:  15,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
	}

	for _, item := range items {
		err := itemRepo.SaveItem(item)
		if err != nil {
			panic(err)
		}
	}

	// Add BOM structure
	bomLines := []*entities.BOMLine{
		{
			ParentPN:    "SIMPLE_ASSEMBLY",
			ChildPN:     "COMPONENT_A",
			QtyPer:      entities.Quantity(1),
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
		{
			ParentPN:    "SIMPLE_ASSEMBLY",
			ChildPN:     "COMPONENT_B",
			QtyPer:      entities.Quantity(2),
			FindNumber:  200,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
	}

	for _, bomLine := range bomLines {
		err := bomRepo.SaveBOMLine(bomLine)
		if err != nil {
			panic(err)
		}
	}

	return bomRepo, itemRepo, inventoryRepo
}
