package mrp

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

// InMemoryBOMRepository provides an in-memory implementation for testing
type InMemoryBOMRepository struct {
	items      map[PartNumber]*Item
	bomLines   []BOMLine
	serialComp *SerialComparator
}

// NewInMemoryBOMRepository creates a new in-memory BOM repository
func NewInMemoryBOMRepository() *InMemoryBOMRepository {
	return &InMemoryBOMRepository{
		items:      make(map[PartNumber]*Item),
		bomLines:   []BOMLine{},
		serialComp: NewSerialComparator(),
	}
}

// NewTestBOMRepository creates a compact BOM repository for testing with reasonable defaults
func NewTestBOMRepository() *CompactBOMRepository {
	return NewCompactBOMRepository(20, 20) // Default size for most tests
}

// NewTestEngine creates an optimized MRP engine for testing
func NewTestEngine(bomRepo BOMRepository, inventoryRepo InventoryRepository) MRPEngine {
	config := OptimizationConfig{
		EnableGCPacing:       true,
		CacheCleanupInterval: 5 * time.Minute,
		MaxCacheEntries:      1000, // Smaller for tests
		BatchSize:           100,   // Smaller for tests
	}
	return NewOptimizedEngine(bomRepo, inventoryRepo, config)
}

// AddItem adds an item to the repository
func (r *InMemoryBOMRepository) AddItem(item Item) {
	r.items[item.PartNumber] = &item
}

// AddBOMLine adds a BOM line to the repository
func (r *InMemoryBOMRepository) AddBOMLine(line BOMLine) {
	r.bomLines = append(r.bomLines, line)
}

// GetEffectiveBOM returns the effective BOM lines for a part and target serial
func (r *InMemoryBOMRepository) GetEffectiveBOM(ctx context.Context, pn PartNumber, serial string) ([]BOMLine, error) {
	var effectiveLines []BOMLine
	
	for _, line := range r.bomLines {
		if line.ParentPN == pn && r.serialComp.IsSerialInRange(serial, line.Effectivity) {
			effectiveLines = append(effectiveLines, line)
		}
	}
	
	return effectiveLines, nil
}

// GetItem returns item master data for a part number
func (r *InMemoryBOMRepository) GetItem(ctx context.Context, pn PartNumber) (*Item, error) {
	item, exists := r.items[pn]
	if !exists {
		return nil, fmt.Errorf("item not found: %s", pn)
	}
	return item, nil
}

// GetAllBOMLines returns all BOM lines
func (r *InMemoryBOMRepository) GetAllBOMLines(ctx context.Context) ([]BOMLine, error) {
	return r.bomLines, nil
}

// GetAllItems returns all items
func (r *InMemoryBOMRepository) GetAllItems(ctx context.Context) ([]Item, error) {
	var items []Item
	for _, item := range r.items {
		items = append(items, *item)
	}
	return items, nil
}

// InMemoryInventoryRepository provides an in-memory implementation for testing
type InMemoryInventoryRepository struct {
	lotInventory        []InventoryLot
	serializedInventory []SerializedInventory
}

// NewInMemoryInventoryRepository creates a new in-memory inventory repository
func NewInMemoryInventoryRepository() *InMemoryInventoryRepository {
	return &InMemoryInventoryRepository{
		lotInventory:        []InventoryLot{},
		serializedInventory: []SerializedInventory{},
	}
}

// AddLotInventory adds lot inventory to the repository
func (r *InMemoryInventoryRepository) AddLotInventory(lot InventoryLot) {
	r.lotInventory = append(r.lotInventory, lot)
}

// AddSerializedInventory adds serialized inventory to the repository
func (r *InMemoryInventoryRepository) AddSerializedInventory(inv SerializedInventory) {
	r.serializedInventory = append(r.serializedInventory, inv)
}

// GetAvailableInventory returns available lot and serialized inventory for a part at a location
func (r *InMemoryInventoryRepository) GetAvailableInventory(ctx context.Context, pn PartNumber, location string) ([]InventoryLot, []SerializedInventory, error) {
	var availableLots []InventoryLot
	var availableSerials []SerializedInventory
	
	// Filter and sort lot inventory by receipt date (FIFO)
	for _, lot := range r.lotInventory {
		if lot.PartNumber == pn && lot.Location == location && lot.Status == Available {
			availableLots = append(availableLots, lot)
		}
	}
	sort.Slice(availableLots, func(i, j int) bool {
		return availableLots[i].ReceiptDate.Before(availableLots[j].ReceiptDate)
	})
	
	// Filter and sort serialized inventory by receipt date (FIFO)
	for _, inv := range r.serializedInventory {
		if inv.PartNumber == pn && inv.Location == location && inv.Status == Available {
			availableSerials = append(availableSerials, inv)
		}
	}
	sort.Slice(availableSerials, func(i, j int) bool {
		return availableSerials[i].ReceiptDate.Before(availableSerials[j].ReceiptDate)
	})
	
	return availableLots, availableSerials, nil
}

// GetInventoryByLot returns inventory for a specific lot
func (r *InMemoryInventoryRepository) GetInventoryByLot(ctx context.Context, pn PartNumber, lotNumber string) (*InventoryLot, error) {
	for _, lot := range r.lotInventory {
		if lot.PartNumber == pn && lot.LotNumber == lotNumber {
			return &lot, nil
		}
	}
	return nil, fmt.Errorf("lot not found: %s for part %s", lotNumber, pn)
}

// GetInventoryBySerial returns inventory for a specific serial number
func (r *InMemoryInventoryRepository) GetInventoryBySerial(ctx context.Context, pn PartNumber, serialNumber string) (*SerializedInventory, error) {
	for _, inv := range r.serializedInventory {
		if inv.PartNumber == pn && inv.SerialNumber == serialNumber {
			return &inv, nil
		}
	}
	return nil, fmt.Errorf("serial not found: %s for part %s", serialNumber, pn)
}

// UpdateInventoryAllocation updates inventory status after allocation
func (r *InMemoryInventoryRepository) UpdateInventoryAllocation(ctx context.Context, allocations []InventoryAllocation) error {
	for _, alloc := range allocations {
		if alloc.LotNumber != "" {
			// Update lot inventory
			for i := range r.lotInventory {
				if r.lotInventory[i].LotNumber == alloc.LotNumber && r.lotInventory[i].Location == alloc.Location {
					newQty := decimal.Decimal(r.lotInventory[i].Quantity).Sub(decimal.Decimal(alloc.Quantity))
					r.lotInventory[i].Quantity = Quantity(newQty)
					if newQty.IsZero() {
						r.lotInventory[i].Status = Allocated
					}
					break
				}
			}
		}
		
		if alloc.SerialNumber != "" {
			// Update serialized inventory
			for i := range r.serializedInventory {
				if r.serializedInventory[i].SerialNumber == alloc.SerialNumber && r.serializedInventory[i].Location == alloc.Location {
					r.serializedInventory[i].Status = Allocated
					break
				}
			}
		}
	}
	return nil
}

// buildAerospaceTestData builds the aerospace test scenario from the specification  
func buildAerospaceTestData() (*CompactBOMRepository, *InMemoryInventoryRepository) {
	bomRepo := NewCompactBOMRepository(6, 5) // 6 items, 5 BOM lines
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