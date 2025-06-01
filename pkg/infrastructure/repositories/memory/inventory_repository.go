package memory

import (
	"fmt"
	"sort"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
)

// InventoryRepository provides in-memory inventory storage
type InventoryRepository struct {
	lotInventory        []entities.InventoryLot
	serializedInventory []entities.SerializedInventory
}

// NewInventoryRepository creates a new in-memory inventory repository
func NewInventoryRepository() *InventoryRepository {
	return &InventoryRepository{
		lotInventory:        []entities.InventoryLot{},
		serializedInventory: []entities.SerializedInventory{},
	}
}

// Verify interface compliance
var _ repositories.InventoryRepository = (*InventoryRepository)(nil)

// LoadInventoryLots loads inventory lots into the repository
func (r *InventoryRepository) LoadInventoryLots(lots []*entities.InventoryLot) error {
	for _, lot := range lots {
		r.AddLotInventory(*lot)
	}
	return nil
}

// LoadSerializedInventory loads serialized inventory into the repository
func (r *InventoryRepository) LoadSerializedInventory(inventory []*entities.SerializedInventory) error {
	for _, inv := range inventory {
		r.AddSerializedInventory(*inv)
	}
	return nil
}

// AddLotInventory adds lot inventory to the repository
func (r *InventoryRepository) AddLotInventory(lot entities.InventoryLot) {
	r.lotInventory = append(r.lotInventory, lot)
}

// AddSerializedInventory adds serialized inventory to the repository
func (r *InventoryRepository) AddSerializedInventory(inv entities.SerializedInventory) {
	r.serializedInventory = append(r.serializedInventory, inv)
}

// GetInventoryLots returns available lot inventory for a part at a location
func (r *InventoryRepository) GetInventoryLots(partNumber entities.PartNumber, location string) ([]*entities.InventoryLot, error) {
	var availableLots []*entities.InventoryLot

	// Filter and sort lot inventory by receipt date (FIFO)
	for i := range r.lotInventory {
		lot := &r.lotInventory[i]
		if lot.PartNumber == partNumber && lot.Location == location && lot.Status == entities.Available {
			availableLots = append(availableLots, lot)
		}
	}
	sort.Slice(availableLots, func(i, j int) bool {
		return availableLots[i].ReceiptDate.Before(availableLots[j].ReceiptDate)
	})

	return availableLots, nil
}

// GetSerializedInventory returns available serialized inventory for a part at a location
func (r *InventoryRepository) GetSerializedInventory(partNumber entities.PartNumber, location string) ([]*entities.SerializedInventory, error) {
	var availableSerials []*entities.SerializedInventory

	// Filter and sort serialized inventory by receipt date (FIFO)
	for i := range r.serializedInventory {
		inv := &r.serializedInventory[i]
		if inv.PartNumber == partNumber && inv.Location == location && inv.Status == entities.Available {
			availableSerials = append(availableSerials, inv)
		}
	}
	sort.Slice(availableSerials, func(i, j int) bool {
		return availableSerials[i].ReceiptDate.Before(availableSerials[j].ReceiptDate)
	})

	return availableSerials, nil
}

// GetAllInventoryLots returns all inventory lots
func (r *InventoryRepository) GetAllInventoryLots() ([]*entities.InventoryLot, error) {
	var lots []*entities.InventoryLot
	for i := range r.lotInventory {
		lots = append(lots, &r.lotInventory[i])
	}
	return lots, nil
}

// GetAllSerializedInventory returns all serialized inventory
func (r *InventoryRepository) GetAllSerializedInventory() ([]*entities.SerializedInventory, error) {
	var inventory []*entities.SerializedInventory
	for i := range r.serializedInventory {
		inventory = append(inventory, &r.serializedInventory[i])
	}
	return inventory, nil
}

// AllocateInventory allocates inventory using FIFO allocation strategy
func (r *InventoryRepository) AllocateInventory(partNumber entities.PartNumber, location string, quantity entities.Quantity) (*entities.AllocationResult, error) {
	result := &entities.AllocationResult{
		PartNumber:      partNumber,
		Location:        location,
		AllocatedQty:    0,
		RemainingDemand: quantity,
		AllocatedFrom:   []entities.InventoryAllocation{},
	}

	remainingQty := quantity

	// First, try to allocate from lot inventory
	lots, err := r.GetInventoryLots(partNumber, location)
	if err != nil {
		return nil, err
	}

	for _, lot := range lots {
		if remainingQty <= 0 {
			break
		}

		allocQty := remainingQty
		if allocQty > lot.Quantity {
			allocQty = lot.Quantity
		}

		allocation := entities.InventoryAllocation{
			LotNumber: lot.LotNumber,
			Quantity:  allocQty,
			Location:  location,
		}

		result.AllocatedFrom = append(result.AllocatedFrom, allocation)
		result.AllocatedQty += allocQty
		remainingQty -= allocQty

		// Update lot quantity
		lot.Quantity -= allocQty
		if lot.Quantity == 0 {
			lot.Status = entities.Allocated
		}
	}

	// Then, try to allocate from serialized inventory (each serial = quantity 1)
	serials, err := r.GetSerializedInventory(partNumber, location)
	if err != nil {
		return nil, err
	}

	for _, serial := range serials {
		if remainingQty <= 0 {
			break
		}

		allocation := entities.InventoryAllocation{
			SerialNumber: serial.SerialNumber,
			Quantity:     1,
			Location:     location,
		}

		result.AllocatedFrom = append(result.AllocatedFrom, allocation)
		result.AllocatedQty += 1
		remainingQty -= 1

		// Mark serial as allocated
		serial.Status = entities.Allocated
	}

	result.RemainingDemand = remainingQty
	return result, nil
}

// GetInventoryByLot returns inventory for a specific lot
func (r *InventoryRepository) GetInventoryByLot(partNumber entities.PartNumber, lotNumber string) (*entities.InventoryLot, error) {
	for i := range r.lotInventory {
		lot := &r.lotInventory[i]
		if lot.PartNumber == partNumber && lot.LotNumber == lotNumber {
			return lot, nil
		}
	}
	return nil, fmt.Errorf("lot not found: %s for part %s", lotNumber, partNumber)
}

// GetInventoryBySerial returns inventory for a specific serial number
func (r *InventoryRepository) GetInventoryBySerial(partNumber entities.PartNumber, serialNumber string) (*entities.SerializedInventory, error) {
	for i := range r.serializedInventory {
		inv := &r.serializedInventory[i]
		if inv.PartNumber == partNumber && inv.SerialNumber == serialNumber {
			return inv, nil
		}
	}
	return nil, fmt.Errorf("serial not found: %s for part %s", serialNumber, partNumber)
}

// SaveInventoryLot saves an inventory lot to the repository
func (r *InventoryRepository) SaveInventoryLot(lot *entities.InventoryLot) error {
	r.AddLotInventory(*lot)
	return nil
}

// SaveSerializedInventory saves serialized inventory to the repository
func (r *InventoryRepository) SaveSerializedInventory(inv *entities.SerializedInventory) error {
	r.AddSerializedInventory(*inv)
	return nil
}

// GetAvailableQuantity returns the total available quantity for a part at a location
func (r *InventoryRepository) GetAvailableQuantity(partNumber entities.PartNumber, location string) (entities.Quantity, error) {
	var totalQty entities.Quantity = 0

	// Add lot inventory quantities
	lots, err := r.GetInventoryLots(partNumber, location)
	if err != nil {
		return 0, err
	}

	for _, lot := range lots {
		totalQty += lot.Quantity
	}

	// Add serialized inventory (each serial = quantity 1)
	serials, err := r.GetSerializedInventory(partNumber, location)
	if err != nil {
		return 0, err
	}

	totalQty += entities.Quantity(len(serials))

	return totalQty, nil
}

// UpdateInventoryStatus updates the status of inventory
func (r *InventoryRepository) UpdateInventoryStatus(partNumber entities.PartNumber, lotNumber string, location string, status entities.InventoryStatus) error {
	for i := range r.lotInventory {
		lot := &r.lotInventory[i]
		if lot.PartNumber == partNumber && lot.LotNumber == lotNumber && lot.Location == location {
			lot.Status = status
			return nil
		}
	}
	return fmt.Errorf("lot not found: %s for part %s at %s", lotNumber, partNumber, location)
}
