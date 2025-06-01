package services

import (
	"context"
	"testing"
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/infrastructure/repositories/memory"
)

func TestMRPService_CriticalPathAnalysis_SimpleCase(t *testing.T) {
	ctx := context.Background()

	// Build a simple test BOM with known critical path
	bomRepo, itemRepo, inventoryRepo := buildCriticalPathTestData()
	demandRepo := memory.NewDemandRepository()

	// Create MRP service
	mrpService := newTestMRPService(bomRepo, itemRepo, inventoryRepo, demandRepo)
	criticalPathService := NewCriticalPathService(bomRepo, itemRepo, inventoryRepo, nil)
	orchestrator := NewPlanningOrchestrator(mrpService, criticalPathService)

	// Analyze critical path for rocket engine
	analysis, err := orchestrator.AnalyzeCriticalPathForPart(
		ctx,
		"ROCKET_ENGINE",
		"AS507",
		"KENNEDY",
		5,
	)
	if err != nil {
		t.Fatalf("Critical path analysis failed: %v", err)
	}

	// Verify we found paths
	if len(analysis.TopPaths) == 0 {
		t.Fatal("Expected to find at least one critical path")
	}

	t.Logf("Critical Path Analysis Results:")
	t.Logf("  %s", analysis.GetCriticalPathSummary())
	t.Logf("  Total paths analyzed: %d", analysis.TotalPaths)
	t.Logf("  Inventory coverage: %.1f%%", analysis.GetInventoryCoverage())

	// Verify critical path
	criticalPath := analysis.CriticalPath
	if criticalPath.TotalLeadTime <= 0 {
		t.Error("Expected positive total lead time")
	}

	if len(criticalPath.Path) == 0 {
		t.Error("Expected non-empty critical path")
	}

	t.Logf("  Critical path details:")
	for _, node := range criticalPath.PathDetails {
		inventoryStatus := "No inventory"
		if node.HasInventory {
			inventoryStatus = "Has inventory"
		}
		t.Logf("    Level %d: %s (%d days) - %s",
			node.Level, node.PartNumber, node.LeadTimeDays, inventoryStatus)
	}

	// Show all top paths
	t.Logf("  Top %d paths:", len(analysis.TopPaths))
	for i, path := range analysis.TopPaths {
		t.Logf("    %d. %s", i+1, path.GetPathSummary())
	}
}

func TestMRPService_CriticalPathAnalysis_WithInventory(t *testing.T) {
	ctx := context.Background()

	bomRepo, itemRepo, inventoryRepo := buildCriticalPathTestData()
	demandRepo := memory.NewDemandRepository()

	// Add more inventory to test inventory impact
	baseDate := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	inventoryLot := &entities.InventoryLot{
		PartNumber:  "TURBOPUMP_V3",
		LotNumber:   "TURBO_LOT_001",
		Location:    "KENNEDY",
		Quantity:    entities.Quantity(5),
		ReceiptDate: baseDate,
		Status:      entities.Available,
	}
	err := inventoryRepo.SaveInventoryLot(inventoryLot)
	if err != nil {
		t.Fatalf("Failed to save inventory lot: %v", err)
	}

	mrpService := newTestMRPService(bomRepo, itemRepo, inventoryRepo, demandRepo)
	criticalPathService := NewCriticalPathService(bomRepo, itemRepo, inventoryRepo, nil)
	orchestrator := NewPlanningOrchestrator(mrpService, criticalPathService)

	// Analyze critical path
	analysis, err := orchestrator.AnalyzeCriticalPathForPart(
		ctx,
		"ROCKET_ENGINE",
		"AS507",
		"KENNEDY",
		3,
	)
	if err != nil {
		t.Fatalf("Critical path analysis failed: %v", err)
	}

	t.Logf("Critical Path Analysis with Inventory:")
	t.Logf("  %s", analysis.GetCriticalPathSummary())
	t.Logf("  Inventory coverage: %.1f%%", analysis.GetInventoryCoverage())

	// Verify inventory impact
	foundInventoryNode := false
	for _, node := range analysis.CriticalPath.PathDetails {
		if node.HasInventory {
			foundInventoryNode = true
			t.Logf("  Found inventory for %s: %d units available",
				node.PartNumber, node.InventoryQty)

			// Effective lead time should be less than or equal to base lead time
			if node.EffectiveLeadTime > node.LeadTimeDays {
				t.Errorf(
					"Expected effective lead time (%d) to be less than or equal to base lead time (%d) for part with inventory",
					node.EffectiveLeadTime,
					node.LeadTimeDays,
				)
			}
		}
	}

	if !foundInventoryNode {
		t.Error("Expected to find at least one node with inventory")
	}
}

func TestMRPService_CriticalPathAnalysis_MultiplePaths(t *testing.T) {
	ctx := context.Background()

	// Build a BOM with multiple paths of different lengths
	bomRepo := memory.NewBOMRepository(10, 20)
	itemRepo := memory.NewItemRepository(10)
	inventoryRepo := memory.NewInventoryRepository()
	demandRepo := memory.NewDemandRepository()

	// Create a part with multiple child paths
	items := []*entities.Item{
		{
			PartNumber:    "COMPLEX_ASSEMBLY",
			Description:   "Assembly with Multiple Paths",
			LeadTimeDays:  30,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		// Short path (1 level)
		{
			PartNumber:    "SIMPLE_PART",
			Description:   "Simple Part",
			LeadTimeDays:  10,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		// Medium path (2 levels)
		{
			PartNumber:    "MEDIUM_SUBASSY",
			Description:   "Medium Subassembly",
			LeadTimeDays:  45,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "MEDIUM_PART",
			Description:   "Medium Part",
			LeadTimeDays:  15,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		// Long path (3 levels)
		{
			PartNumber:    "COMPLEX_SUBASSY",
			Description:   "Complex Subassembly",
			LeadTimeDays:  60,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "COMPLEX_COMPONENT",
			Description:   "Complex Component",
			LeadTimeDays:  25,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "RAW_MATERIAL",
			Description:   "Raw Material",
			LeadTimeDays:  20,
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

	// Add BOM relationships
	bomLines := []*entities.BOMLine{
		{
			ParentPN:    "COMPLEX_ASSEMBLY",
			ChildPN:     "SIMPLE_PART",
			QtyPer:      entities.Quantity(1),
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
		{
			ParentPN:    "COMPLEX_ASSEMBLY",
			ChildPN:     "MEDIUM_SUBASSY",
			QtyPer:      entities.Quantity(1),
			FindNumber:  200,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
		{
			ParentPN:    "COMPLEX_ASSEMBLY",
			ChildPN:     "COMPLEX_SUBASSY",
			QtyPer:      entities.Quantity(1),
			FindNumber:  300,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
		{
			ParentPN:    "MEDIUM_SUBASSY",
			ChildPN:     "MEDIUM_PART",
			QtyPer:      entities.Quantity(1),
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
		{
			ParentPN:    "COMPLEX_SUBASSY",
			ChildPN:     "COMPLEX_COMPONENT",
			QtyPer:      entities.Quantity(1),
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
		{
			ParentPN:    "COMPLEX_COMPONENT",
			ChildPN:     "RAW_MATERIAL",
			QtyPer:      entities.Quantity(1),
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
	}

	for _, bomLine := range bomLines {
		err := bomRepo.SaveBOMLine(bomLine)
		if err != nil {
			t.Fatalf("Failed to save BOM line: %v", err)
		}
	}

	mrpService := newTestMRPService(bomRepo, itemRepo, inventoryRepo, demandRepo)
	criticalPathService := NewCriticalPathService(bomRepo, itemRepo, inventoryRepo, nil)
	orchestrator := NewPlanningOrchestrator(mrpService, criticalPathService)

	// Analyze critical path asking for top 5 paths
	analysis, err := orchestrator.AnalyzeCriticalPathForPart(
		ctx,
		"COMPLEX_ASSEMBLY",
		"SN001",
		"FACTORY",
		5,
	)
	if err != nil {
		t.Fatalf("Critical path analysis failed: %v", err)
	}

	t.Logf("Multiple Paths Analysis:")
	t.Logf("  Total paths found: %d", analysis.TotalPaths)
	t.Logf("  %s", analysis.GetCriticalPathSummary())

	// Should find 3 different paths
	if analysis.TotalPaths != 3 {
		t.Errorf("Expected 3 paths, got %d", analysis.TotalPaths)
	}

	// Verify paths are sorted by lead time (longest first)
	for i := 1; i < len(analysis.TopPaths); i++ {
		if analysis.TopPaths[i-1].EffectiveLeadTime < analysis.TopPaths[i].EffectiveLeadTime {
			t.Errorf("Paths not sorted correctly: path %d has shorter lead time than path %d",
				i-1, i)
		}
	}

	// Log all paths for verification
	for i, path := range analysis.TopPaths {
		t.Logf("  Path %d: %s", i+1, path.GetPathSummary())
		for _, node := range path.PathDetails {
			t.Logf("    %s (%d days)", node.PartNumber, node.LeadTimeDays)
		}
	}

	// Critical path should be the longest one
	expectedCriticalPath := 30 + 60 + 25 + 20 // COMPLEX_ASSEMBLY -> COMPLEX_SUBASSY -> COMPLEX_COMPONENT -> RAW_MATERIAL
	if analysis.CriticalPath.TotalLeadTime != expectedCriticalPath {
		t.Errorf("Expected critical path of %d days, got %d",
			expectedCriticalPath, analysis.CriticalPath.TotalLeadTime)
	}
}

// buildCriticalPathTestData creates test data for critical path analysis
func buildCriticalPathTestData() (*memory.BOMRepository, *memory.ItemRepository, *memory.InventoryRepository) {
	bomRepo := memory.NewBOMRepository(4, 3)
	itemRepo := memory.NewItemRepository(4)
	inventoryRepo := memory.NewInventoryRepository()

	// Add items
	items := []*entities.Item{
		{
			PartNumber:    "ROCKET_ENGINE",
			Description:   "Main Rocket Engine Assembly",
			LeadTimeDays:  120,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "TURBOPUMP_V3",
			Description:   "Turbopump Assembly V3 (Latest)",
			LeadTimeDays:  60,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "COMBUSTION_CHAMBER",
			Description:   "Main Combustion Chamber",
			LeadTimeDays:  90,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "VALVE_ASSEMBLY",
			Description:   "Main Valve Assembly",
			LeadTimeDays:  45,
			LotSizeRule:   entities.MinimumQty,
			MinOrderQty:   entities.Quantity(10),
			SafetyStock:   entities.Quantity(5),
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
			ParentPN:    "ROCKET_ENGINE",
			ChildPN:     "TURBOPUMP_V3",
			QtyPer:      entities.Quantity(2),
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN050", ToSerial: ""},
		},
		{
			ParentPN:    "ROCKET_ENGINE",
			ChildPN:     "COMBUSTION_CHAMBER",
			QtyPer:      entities.Quantity(1),
			FindNumber:  200,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
		{
			ParentPN:    "ROCKET_ENGINE",
			ChildPN:     "VALVE_ASSEMBLY",
			QtyPer:      entities.Quantity(4),
			FindNumber:  300,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
	}

	for _, bomLine := range bomLines {
		err := bomRepo.SaveBOMLine(bomLine)
		if err != nil {
			panic(err)
		}
	}

	// Add some inventory
	baseDate := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	serialInventory := &entities.SerializedInventory{
		PartNumber:   "ROCKET_ENGINE",
		SerialNumber: "F1_001",
		Location:     "KENNEDY",
		Status:       entities.Available,
		ReceiptDate:  baseDate,
	}
	err := inventoryRepo.SaveSerializedInventory(serialInventory)
	if err != nil {
		panic(err)
	}

	lotInventory := &entities.InventoryLot{
		PartNumber:  "VALVE_ASSEMBLY",
		LotNumber:   "VALVE_LOT_001",
		Location:    "KENNEDY",
		Quantity:    entities.Quantity(15),
		ReceiptDate: baseDate.Add(-30 * 24 * time.Hour),
		Status:      entities.Available,
	}
	err = inventoryRepo.SaveInventoryLot(lotInventory)
	if err != nil {
		panic(err)
	}

	return bomRepo, itemRepo, inventoryRepo
}
