package repositories

import "github.com/vsinha/mrp/pkg/domain/entities"

// ItemRepository provides access to item master data
type ItemRepository interface {
	GetItem(partNumber entities.PartNumber) (*entities.Item, error)
	GetAllItems() ([]*entities.Item, error)
	LoadItems(items []*entities.Item) error
}
