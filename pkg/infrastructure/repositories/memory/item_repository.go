package memory

import (
	"fmt"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
)

// ItemRepository provides in-memory item storage
type ItemRepository struct {
	items    []entities.Item
	itemsMap map[entities.PartNumber]int
}

// NewItemRepository creates a new in-memory item repository
func NewItemRepository(expectedItems int) *ItemRepository {
	return &ItemRepository{
		items:    make([]entities.Item, 0, expectedItems),
		itemsMap: make(map[entities.PartNumber]int, expectedItems),
	}
}

// Verify interface compliance
var _ repositories.ItemRepository = (*ItemRepository)(nil)

// LoadItems loads items into the repository
func (r *ItemRepository) LoadItems(items []*entities.Item) error {
	for _, item := range items {
		r.AddItem(*item)
	}
	return nil
}

// AddItem adds an item to the repository
func (r *ItemRepository) AddItem(item entities.Item) {
	r.itemsMap[item.PartNumber] = len(r.items)
	r.items = append(r.items, item)
}

// GetItem returns item master data for a part number
func (r *ItemRepository) GetItem(partNumber entities.PartNumber) (*entities.Item, error) {
	index, exists := r.itemsMap[partNumber]
	if !exists {
		return nil, fmt.Errorf("item not found: %s", partNumber)
	}
	return &r.items[index], nil
}

// GetAllItems returns all items
func (r *ItemRepository) GetAllItems() ([]*entities.Item, error) {
	var items []*entities.Item
	for i := range r.items {
		items = append(items, &r.items[i])
	}
	return items, nil
}

// SaveItem saves an item to the repository
func (r *ItemRepository) SaveItem(item *entities.Item) error {
	r.AddItem(*item)
	return nil
}
