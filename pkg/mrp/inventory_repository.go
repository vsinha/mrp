package mrp

import (
	"context"
	"fmt"
	"sort"
)

type InventoryRepository struct {
	lotInventory        []InventoryLot
	serializedInventory []SerializedInventory
}

// NewInMemoryInventoryRepository creates a new in-memory inventory repository
func NewInMemoryInventoryRepository() *InventoryRepository {
	return &InventoryRepository{
		lotInventory:        []InventoryLot{},
		serializedInventory: []SerializedInventory{},
	}
}

// AddLotInventory adds lot inventory to the repository
func (r *InventoryRepository) AddLotInventory(lot InventoryLot) {
	r.lotInventory = append(r.lotInventory, lot)
}

// AddSerializedInventory adds serialized inventory to the repository
func (r *InventoryRepository) AddSerializedInventory(inv SerializedInventory) {
	r.serializedInventory = append(r.serializedInventory, inv)
}

// GetAvailableInventory returns available lot and serialized inventory for a part at a location
func (r *InventoryRepository) GetAvailableInventory(ctx context.Context, pn PartNumber, location string) ([]InventoryLot, []SerializedInventory, error) {
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
func (r *InventoryRepository) GetInventoryByLot(ctx context.Context, pn PartNumber, lotNumber string) (*InventoryLot, error) {
	for _, lot := range r.lotInventory {
		if lot.PartNumber == pn && lot.LotNumber == lotNumber {
			return &lot, nil
		}
	}
	return nil, fmt.Errorf("lot not found: %s for part %s", lotNumber, pn)
}

// GetInventoryBySerial returns inventory for a specific serial number
func (r *InventoryRepository) GetInventoryBySerial(ctx context.Context, pn PartNumber, serialNumber string) (*SerializedInventory, error) {
	for _, inv := range r.serializedInventory {
		if inv.PartNumber == pn && inv.SerialNumber == serialNumber {
			return &inv, nil
		}
	}
	return nil, fmt.Errorf("serial not found: %s for part %s", serialNumber, pn)
}

// UpdateInventoryAllocation updates inventory status after allocation
func (r *InventoryRepository) UpdateInventoryAllocation(ctx context.Context, allocations []InventoryAllocation) error {
	for _, alloc := range allocations {
		if alloc.LotNumber != "" {
			// Update lot inventory
			for i := range r.lotInventory {
				if r.lotInventory[i].LotNumber == alloc.LotNumber && r.lotInventory[i].Location == alloc.Location {
					r.lotInventory[i].Quantity = r.lotInventory[i].Quantity - alloc.Quantity
					if r.lotInventory[i].Quantity == 0 {
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
