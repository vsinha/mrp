package shared

import (
	"sort"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
)

// SelectBestAlternateByPriority selects the best alternate from a group based solely on priority
// Returns nil if no alternates are provided
// Priority rules: 0 = standard/primary, 1+ = alternates with lower number = higher priority
func SelectBestAlternateByPriority(alternates []*entities.BOMLine) *entities.BOMLine {
	if len(alternates) == 0 {
		return nil
	}

	// Sort alternates by priority (0 = standard/primary, 1+ = alternates with lower number = higher priority)
	sortedAlternates := make([]*entities.BOMLine, len(alternates))
	copy(sortedAlternates, alternates)
	sort.Slice(sortedAlternates, func(i, j int) bool {
		return sortedAlternates[i].Priority < sortedAlternates[j].Priority
	})

	// Return the highest priority (lowest Priority value) alternate
	return sortedAlternates[0]
}

// SelectBestAlternateWithInventory selects the best alternate considering inventory availability
// Falls back to priority-based selection if no alternate has sufficient inventory
func SelectBestAlternateWithInventory(
	alternates []*entities.BOMLine,
	requiredQty entities.Quantity,
	inventoryRepo repositories.InventoryRepository,
) *entities.BOMLine {
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
		availableQty, err := getAvailableQuantity(alternate.ChildPN, inventoryRepo)
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
func getAvailableQuantity(
	partNumber entities.PartNumber,
	inventoryRepo repositories.InventoryRepository,
) (entities.Quantity, error) {
	// Get all inventory lots for this part (we'll assume "" location means all locations)
	lots, err := inventoryRepo.GetInventoryLots(partNumber, "")
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
