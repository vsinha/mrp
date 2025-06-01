package repositories

import "github.com/vsinha/mrp/pkg/domain/entities"

// InventoryRepository provides access to inventory data
type InventoryRepository interface {
	GetInventoryLots(
		partNumber entities.PartNumber,
		location string,
	) ([]*entities.InventoryLot, error)
	GetSerializedInventory(
		partNumber entities.PartNumber,
		location string,
	) ([]*entities.SerializedInventory, error)
	GetAllInventoryLots() ([]*entities.InventoryLot, error)
	GetAllSerializedInventory() ([]*entities.SerializedInventory, error)
	LoadInventoryLots(lots []*entities.InventoryLot) error
	LoadSerializedInventory(inventory []*entities.SerializedInventory) error
	AllocateInventory(
		partNumber entities.PartNumber,
		location string,
		quantity entities.Quantity,
	) (*entities.AllocationResult, error)
}
