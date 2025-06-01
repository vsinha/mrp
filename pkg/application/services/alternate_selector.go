package services

import (
	"sort"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
)

// AlternateSelector provides logic for selecting the best alternate from a group
type AlternateSelector struct {
	inventoryRepo repositories.InventoryRepository
	itemRepo      repositories.ItemRepository
}

// NewAlternateSelector creates a new alternate selector
func NewAlternateSelector(inventoryRepo repositories.InventoryRepository, itemRepo repositories.ItemRepository) *AlternateSelector {
	return &AlternateSelector{
		inventoryRepo: inventoryRepo,
		itemRepo:      itemRepo,
	}
}

// SelectBestAlternate selects the best alternate from a group based on priority and availability
// Returns nil if no suitable alternate is found
func (s *AlternateSelector) SelectBestAlternate(alternates []*entities.BOMLine) *entities.BOMLine {
	if len(alternates) == 0 {
		return nil
	}

	// Sort alternates by priority (0 = standard/primary, 1+ = alternates with lower number = higher priority)
	sortedAlternates := make([]*entities.BOMLine, len(alternates))
	copy(sortedAlternates, alternates)
	sort.Slice(sortedAlternates, func(i, j int) bool {
		return sortedAlternates[i].Priority < sortedAlternates[j].Priority
	})

	// For now, simply return the highest priority (lowest Priority value) alternate
	// Future enhancement: could check inventory availability and select based on that
	return sortedAlternates[0]
}

// SelectBestAlternateWithAvailability selects the best alternate considering inventory availability
// This is an enhanced version that checks inventory before selecting
func (s *AlternateSelector) SelectBestAlternateWithAvailability(alternates []*entities.BOMLine, requiredQty entities.Quantity) *entities.BOMLine {
	if len(alternates) == 0 {
		return nil
	}

	// Sort alternates by priority (0 = standard/primary, 1+ = alternates with lower number = higher priority)
	sortedAlternates := make([]*entities.BOMLine, len(alternates))
	copy(sortedAlternates, alternates)
	sort.Slice(sortedAlternates, func(i, j int) bool {
		return sortedAlternates[i].Priority < sortedAlternates[j].Priority
	})

	// Try each alternate in priority order
	for _, alternate := range sortedAlternates {
		// Check if we have sufficient inventory for this alternate
		availableQty, err := s.getAvailableQuantity(alternate.ChildPN)
		if err != nil {
			// If we can't determine availability, skip this alternate
			continue
		}

		totalRequired := entities.Quantity(int64(requiredQty) * int64(alternate.QtyPer))
		if availableQty >= totalRequired {
			return alternate
		}
	}

	// If no alternate has sufficient inventory, return the highest priority one
	// The MRP system will create planned orders to fulfill the shortage
	return sortedAlternates[0]
}

// getAvailableQuantity calculates total available quantity for a part number
func (s *AlternateSelector) getAvailableQuantity(partNumber entities.PartNumber) (entities.Quantity, error) {
	// Get all inventory lots for this part (we'll assume "" location means all locations)
	lots, err := s.inventoryRepo.GetInventoryLots(partNumber, "")
	if err != nil {
		return 0, err
	}

	var total entities.Quantity
	for _, lot := range lots {
		// Only count available inventory
		if lot.Status == entities.Available {
			total += lot.Quantity
		}
	}

	return total, nil
}