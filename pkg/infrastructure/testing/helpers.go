package testing

import (
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/infrastructure/repositories/memory"
)

// BuildAerospaceTestData builds the aerospace test scenario from the specification
func BuildAerospaceTestData() (*memory.BOMRepository, *memory.ItemRepository, *memory.InventoryRepository, *memory.DemandRepository) {
	bomRepo := memory.NewBOMRepository(10, 20) // 10 items, 20 BOM lines
	itemRepo := memory.NewItemRepository(10)
	inventoryRepo := memory.NewInventoryRepository()
	demandRepo := memory.NewDemandRepository()

	// Add items
	items := []*entities.Item{
		{
			PartNumber:    "SATURN_V",
			Description:   "Saturn V Launch Vehicle",
			LeadTimeDays:  180,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "F1_ENGINE",
			Description:   "F-1 Engine Assembly",
			LeadTimeDays:  120,
			LotSizeRule:   entities.MinimumQty,
			MinOrderQty:   entities.Quantity(10),
			SafetyStock:   entities.Quantity(2),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "J2_ENGINE_V1",
			Description:   "J-2 Engine V1",
			LeadTimeDays:  90,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "J2_ENGINE_V2",
			Description:   "J-2 Engine V2",
			LeadTimeDays:  90,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "F1_TURBOPUMP_V1",
			Description:   "F-1 Turbopump Assembly V1",
			LeadTimeDays:  60,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "F1_TURBOPUMP_V2",
			Description:   "F-1 Turbopump Assembly V2",
			LeadTimeDays:  60,
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

	// Add BOM lines with serial effectivity
	bomLines := []*entities.BOMLine{
		{
			ParentPN:    "SATURN_V",
			ChildPN:     "F1_ENGINE",
			QtyPer:      entities.Quantity(5),
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "AS501", ToSerial: ""},
		},
		{
			ParentPN:    "SATURN_V",
			ChildPN:     "J2_ENGINE_V1",
			QtyPer:      entities.Quantity(6),
			FindNumber:  200,
			Effectivity: entities.SerialEffectivity{FromSerial: "AS501", ToSerial: "AS506"},
		},
		{
			ParentPN:    "SATURN_V",
			ChildPN:     "J2_ENGINE_V2",
			QtyPer:      entities.Quantity(6),
			FindNumber:  200,
			Effectivity: entities.SerialEffectivity{FromSerial: "AS507", ToSerial: ""},
		},
		{
			ParentPN:    "F1_ENGINE",
			ChildPN:     "F1_TURBOPUMP_V1",
			QtyPer:      entities.Quantity(1),
			FindNumber:  300,
			Effectivity: entities.SerialEffectivity{FromSerial: "AS501", ToSerial: "AS505"},
		},
		{
			ParentPN:    "F1_ENGINE",
			ChildPN:     "F1_TURBOPUMP_V2",
			QtyPer:      entities.Quantity(1),
			FindNumber:  300,
			Effectivity: entities.SerialEffectivity{FromSerial: "AS506", ToSerial: ""},
		},
	}

	for _, bomLine := range bomLines {
		err := bomRepo.SaveBOMLine(bomLine)
		if err != nil {
			panic(err)
		}
	}

	// Add inventory
	baseDate := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

	inventoryLots := []*entities.InventoryLot{
		{
			PartNumber:  "F1_ENGINE",
			LotNumber:   "F1_LOT_001",
			Location:    "MICHOUD",
			Quantity:    entities.Quantity(2),
			Status:      entities.Available,
			ReceiptDate: baseDate,
		},
		{
			PartNumber:  "F1_ENGINE",
			LotNumber:   "F1_LOT_002",
			Location:    "STENNIS",
			Quantity:    entities.Quantity(1),
			Status:      entities.Available,
			ReceiptDate: baseDate.Add(4 * 24 * time.Hour),
		},
	}

	for _, lot := range inventoryLots {
		err := inventoryRepo.SaveInventoryLot(lot)
		if err != nil {
			panic(err)
		}
	}

	return bomRepo, itemRepo, inventoryRepo, demandRepo
}

// BuildSimpleTestData creates simple test data for basic tests
func BuildSimpleTestData() (*memory.BOMRepository, *memory.ItemRepository, *memory.InventoryRepository, *memory.DemandRepository) {
	bomRepo := memory.NewBOMRepository(5, 5) // 5 items, 5 BOM lines
	itemRepo := memory.NewItemRepository(5)
	inventoryRepo := memory.NewInventoryRepository()
	demandRepo := memory.NewDemandRepository()

	// Add simple test items
	items := []*entities.Item{
		{
			PartNumber:    "ASSEMBLY_A",
			Description:   "Test Assembly A",
			LeadTimeDays:  30,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "COMPONENT_A",
			Description:   "Test Component A",
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

	// Add simple BOM relationship
	bomLine := &entities.BOMLine{
		ParentPN:    "ASSEMBLY_A",
		ChildPN:     "COMPONENT_A",
		QtyPer:      entities.Quantity(2),
		FindNumber:  100,
		Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
	}

	err := bomRepo.SaveBOMLine(bomLine)
	if err != nil {
		panic(err)
	}

	return bomRepo, itemRepo, inventoryRepo, demandRepo
}
