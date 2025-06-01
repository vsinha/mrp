package testing

import (
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/infrastructure/repositories/memory"
)

// mustCreateItem is a helper for tests - panics on validation error
func mustCreateItem(
	partNumber, description string,
	leadTime int,
	lotRule entities.LotSizeRule,
	minOrderQty, maxOrderQty, safetyStock entities.Quantity,
	uom string,
) *entities.Item {
	item, err := entities.NewItem(
		entities.PartNumber(partNumber),
		description,
		leadTime,
		lotRule,
		minOrderQty,
		maxOrderQty,
		safetyStock,
		uom,
	)
	if err != nil {
		panic(err)
	}
	return item
}

// mustCreateBOMLine is a helper for tests - panics on validation error
func mustCreateBOMLine(
	parentPN, childPN string,
	qtyPer entities.Quantity,
	findNumber int,
	fromSerial, toSerial string,
) *entities.BOMLine {
	effectivity, err := entities.NewSerialEffectivity(fromSerial, toSerial)
	if err != nil {
		panic(err)
	}
	bomLine, err := entities.NewBOMLine(
		entities.PartNumber(parentPN),
		entities.PartNumber(childPN),
		qtyPer,
		findNumber,
		*effectivity,
		0,
	)
	if err != nil {
		panic(err)
	}
	return bomLine
}

// mustCreateAlternateBOMLine is a helper for tests with alternate support - panics on validation error
func mustCreateAlternateBOMLine(
	parentPN, childPN string,
	qtyPer entities.Quantity,
	findNumber int,
	fromSerial, toSerial string,
	priority int,
) *entities.BOMLine {
	effectivity, err := entities.NewSerialEffectivity(fromSerial, toSerial)
	if err != nil {
		panic(err)
	}
	bomLine, err := entities.NewBOMLine(
		entities.PartNumber(parentPN),
		entities.PartNumber(childPN),
		qtyPer,
		findNumber,
		*effectivity,
		priority,
	)
	if err != nil {
		panic(err)
	}
	return bomLine
}

// mustCreateInventoryLot is a helper for tests - panics on validation error
func mustCreateInventoryLot(
	partNumber, lotNumber, location string,
	quantity entities.Quantity,
	receiptDate time.Time,
	status entities.InventoryStatus,
) *entities.InventoryLot {
	lot, err := entities.NewInventoryLot(
		entities.PartNumber(partNumber),
		lotNumber,
		location,
		quantity,
		receiptDate,
		status,
	)
	if err != nil {
		panic(err)
	}
	return lot
}

// BuildAerospaceTestData builds the aerospace test scenario from the specification
func BuildAerospaceTestData() (*memory.BOMRepository, *memory.ItemRepository, *memory.InventoryRepository, *memory.DemandRepository) {
	bomRepo := memory.NewBOMRepository(20) // 20 BOM lines
	itemRepo := memory.NewItemRepository(10)
	inventoryRepo := memory.NewInventoryRepository()
	demandRepo := memory.NewDemandRepository()

	// Add items
	items := []*entities.Item{
		mustCreateItem(
			"SATURN_V",
			"Saturn V Launch Vehicle",
			180,
			entities.LotForLot,
			entities.Quantity(1),
			entities.Quantity(10), // Max 10 rockets at once
			entities.Quantity(0),
			"EA",
		),
		mustCreateItem(
			"F1_ENGINE",
			"F-1 Engine Assembly",
			120,
			entities.MinimumQty,
			entities.Quantity(10),
			entities.Quantity(50), // Max 50 engines at once
			entities.Quantity(2),
			"EA",
		),
		mustCreateItem(
			"J2_ENGINE_V1",
			"J-2 Engine V1",
			90,
			entities.LotForLot,
			entities.Quantity(1),
			entities.Quantity(30), // Max 30 engines at once
			entities.Quantity(0),
			"EA",
		),
		mustCreateItem(
			"J2_ENGINE_V2",
			"J-2 Engine V2",
			90,
			entities.LotForLot,
			entities.Quantity(1),
			entities.Quantity(30), // Max 30 engines at once
			entities.Quantity(0),
			"EA",
		),
		mustCreateItem(
			"F1_TURBOPUMP_V1",
			"F-1 Turbopump Assembly V1",
			60,
			entities.LotForLot,
			entities.Quantity(1),
			entities.Quantity(100), // Max 100 turbopumps at once
			entities.Quantity(0),
			"EA",
		),
		mustCreateItem(
			"F1_TURBOPUMP_V2",
			"F-1 Turbopump Assembly V2",
			60,
			entities.LotForLot,
			entities.Quantity(1),
			entities.Quantity(100), // Max 100 turbopumps at once
			entities.Quantity(0),
			"EA",
		),
	}

	for _, item := range items {
		err := itemRepo.SaveItem(item)
		if err != nil {
			panic(err)
		}
	}

	// Add BOM lines with serial effectivity
	bomLines := []*entities.BOMLine{
		mustCreateBOMLine("SATURN_V", "F1_ENGINE", entities.Quantity(5), 100, "AS501", ""),
		mustCreateBOMLine("SATURN_V", "J2_ENGINE_V1", entities.Quantity(6), 200, "AS501", "AS506"),
		mustCreateBOMLine("SATURN_V", "J2_ENGINE_V2", entities.Quantity(6), 200, "AS507", ""),
		mustCreateBOMLine(
			"F1_ENGINE",
			"F1_TURBOPUMP_V1",
			entities.Quantity(1),
			300,
			"AS501",
			"AS505",
		),
		mustCreateBOMLine("F1_ENGINE", "F1_TURBOPUMP_V2", entities.Quantity(1), 300, "AS506", ""),
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
		mustCreateInventoryLot(
			"F1_ENGINE",
			"F1_LOT_001",
			"MICHOUD",
			entities.Quantity(2),
			baseDate,
			entities.Available,
		),
		mustCreateInventoryLot(
			"F1_ENGINE",
			"F1_LOT_002",
			"STENNIS",
			entities.Quantity(1),
			baseDate.Add(4*24*time.Hour),
			entities.Available,
		),
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
	bomRepo := memory.NewBOMRepository(5) // 5 BOM lines
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
			MaxOrderQty:   entities.Quantity(100),
			SafetyStock:   entities.Quantity(0),
			UnitOfMeasure: "EA",
		},
		{
			PartNumber:    "COMPONENT_A",
			Description:   "Test Component A",
			LeadTimeDays:  15,
			LotSizeRule:   entities.LotForLot,
			MinOrderQty:   entities.Quantity(1),
			MaxOrderQty:   entities.Quantity(200),
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
