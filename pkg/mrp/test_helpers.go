package mrp

import (
	"time"

	"github.com/shopspring/decimal"
)

// Creates a BOM repository for testing with reasonable defaults
func NewTestBOMRepository() *BOMRepository {
	return NewBOMRepository(20, 20) // Default size for most tests
}

// NewTestEngine creates an MRP engine for testing with reasonable defaults
func NewTestEngine(bomRepo *BOMRepository, inventoryRepo *InventoryRepository) *Engine {
	config := EngineConfig{
		EnableGCPacing:  false, // Disable GC tuning in tests for predictable performance
		MaxCacheEntries: 1000,  // Smaller cache for tests
	}
	return NewEngineWithConfig(bomRepo, inventoryRepo, config)
}


// buildAerospaceTestData builds the aerospace test scenario from the specification  
func buildAerospaceTestData() (*BOMRepository, *InventoryRepository) {
	bomRepo := NewBOMRepository(6, 5) // 6 items, 5 BOM lines
	inventoryRepo := NewInMemoryInventoryRepository()
	
	// Add items
	bomRepo.AddItem(Item{
		PartNumber:      "SATURN_V",
		Description:     "Saturn V Launch Vehicle",
		LeadTimeDays:    180,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(decimal.NewFromInt(1)),
		SafetyStock:     Quantity(decimal.Zero),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddItem(Item{
		PartNumber:      "F1_ENGINE",
		Description:     "F-1 Engine Assembly",
		LeadTimeDays:    120,
		LotSizeRule:     MinimumQty,
		MinOrderQty:     Quantity(decimal.NewFromInt(10)),
		SafetyStock:     Quantity(decimal.NewFromInt(2)),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddItem(Item{
		PartNumber:      "J2_ENGINE_V1",
		Description:     "J-2 Engine V1",
		LeadTimeDays:    90,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(decimal.NewFromInt(1)),
		SafetyStock:     Quantity(decimal.Zero),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddItem(Item{
		PartNumber:      "J2_ENGINE_V2",
		Description:     "J-2 Engine V2",
		LeadTimeDays:    90,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(decimal.NewFromInt(1)),
		SafetyStock:     Quantity(decimal.Zero),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddItem(Item{
		PartNumber:      "F1_TURBOPUMP_V1",
		Description:     "F-1 Turbopump Assembly V1",
		LeadTimeDays:    60,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(decimal.NewFromInt(1)),
		SafetyStock:     Quantity(decimal.Zero),
		UnitOfMeasure:   "EA",
	})
	
	bomRepo.AddItem(Item{
		PartNumber:      "F1_TURBOPUMP_V2",
		Description:     "F-1 Turbopump Assembly V2",
		LeadTimeDays:    60,
		LotSizeRule:     LotForLot,
		MinOrderQty:     Quantity(decimal.NewFromInt(1)),
		SafetyStock:     Quantity(decimal.Zero),
		UnitOfMeasure:   "EA",
	})
	
	// Add BOM lines with serial effectivity
	bomRepo.AddBOMLine(BOMLine{
		ParentPN:     "SATURN_V",
		ChildPN:      "F1_ENGINE",
		QtyPer:       Quantity(decimal.NewFromInt(5)),
		FindNumber:   100,
		Effectivity:  SerialEffectivity{FromSerial: "AS501", ToSerial: ""},
	})
	
	bomRepo.AddBOMLine(BOMLine{
		ParentPN:     "SATURN_V",
		ChildPN:      "J2_ENGINE_V1",
		QtyPer:       Quantity(decimal.NewFromInt(6)),
		FindNumber:   200,
		Effectivity:  SerialEffectivity{FromSerial: "AS501", ToSerial: "AS506"},
	})
	
	bomRepo.AddBOMLine(BOMLine{
		ParentPN:     "SATURN_V",
		ChildPN:      "J2_ENGINE_V2",
		QtyPer:       Quantity(decimal.NewFromInt(6)),
		FindNumber:   200,
		Effectivity:  SerialEffectivity{FromSerial: "AS507", ToSerial: ""},
	})
	
	bomRepo.AddBOMLine(BOMLine{
		ParentPN:     "F1_ENGINE",
		ChildPN:      "F1_TURBOPUMP_V1",
		QtyPer:       Quantity(decimal.NewFromInt(1)),
		FindNumber:   300,
		Effectivity:  SerialEffectivity{FromSerial: "AS501", ToSerial: "AS505"},
	})
	
	bomRepo.AddBOMLine(BOMLine{
		ParentPN:     "F1_ENGINE",
		ChildPN:      "F1_TURBOPUMP_V2",
		QtyPer:       Quantity(decimal.NewFromInt(1)),
		FindNumber:   300,
		Effectivity:  SerialEffectivity{FromSerial: "AS506", ToSerial: ""},
	})
	
	// Add inventory
	baseDate := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	
	inventoryRepo.AddSerializedInventory(SerializedInventory{
		PartNumber:   "F1_ENGINE",
		SerialNumber: "F1_001",
		Location:     "MICHOUD",
		Status:       Available,
		ReceiptDate:  baseDate,
	})
	
	inventoryRepo.AddSerializedInventory(SerializedInventory{
		PartNumber:   "F1_ENGINE",
		SerialNumber: "F1_002",
		Location:     "STENNIS",
		Status:       Allocated,
		ReceiptDate:  baseDate.Add(4 * 24 * time.Hour),
	})
	
	return bomRepo, inventoryRepo
}