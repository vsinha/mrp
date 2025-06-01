package shared

import (
	"fmt"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

// AllocationContext holds allocation information for a specific part and location
type AllocationContext struct {
	AllocatedQty    entities.Quantity
	RemainingDemand entities.Quantity
	HasAllocation   bool
}

// AllocationMap manages allocation context by part number and location
type AllocationMap map[string]*AllocationContext

// NewAllocationMap creates a new empty allocation map
func NewAllocationMap() AllocationMap {
	return make(AllocationMap)
}

// NewAllocationMapFromResults creates an allocation map from MRP allocation results
func NewAllocationMapFromResults(allocations []entities.AllocationResult) AllocationMap {
	allocMap := make(AllocationMap)
	for _, alloc := range allocations {
		key := allocMap.makeKey(alloc.PartNumber, alloc.Location)
		allocMap[key] = &AllocationContext{
			AllocatedQty:    alloc.AllocatedQty,
			RemainingDemand: alloc.RemainingDemand,
			HasAllocation:   alloc.AllocatedQty > 0,
		}
	}
	return allocMap
}

// Get retrieves allocation context for a part and location
func (am AllocationMap) Get(partNumber entities.PartNumber, location string) *AllocationContext {
	key := am.makeKey(partNumber, location)
	return am[key]
}

// Set stores allocation context for a part and location
func (am AllocationMap) Set(
	partNumber entities.PartNumber,
	location string,
	context *AllocationContext,
) {
	key := am.makeKey(partNumber, location)
	am[key] = context
}

// Has checks if allocation context exists for a part and location
func (am AllocationMap) Has(partNumber entities.PartNumber, location string) bool {
	key := am.makeKey(partNumber, location)
	_, exists := am[key]
	return exists
}

// Clear removes all allocation contexts
func (am AllocationMap) Clear() {
	for key := range am {
		delete(am, key)
	}
}

// Size returns the number of allocation contexts stored
func (am AllocationMap) Size() int {
	return len(am)
}

// GetAllParts returns all part numbers that have allocation context
func (am AllocationMap) GetAllParts() []entities.PartNumber {
	partSet := make(map[entities.PartNumber]bool)
	for key := range am {
		// Extract part number from key (format: "partNumber|location")
		if partNumber, _, found := am.parseKey(key); found {
			partSet[partNumber] = true
		}
	}

	var parts []entities.PartNumber
	for part := range partSet {
		parts = append(parts, part)
	}
	return parts
}

// GetTotalAllocated returns the total allocated quantity across all parts
func (am AllocationMap) GetTotalAllocated() entities.Quantity {
	var total entities.Quantity
	for _, context := range am {
		total += context.AllocatedQty
	}
	return total
}

// GetTotalDemand returns the total demand (allocated + remaining) across all parts
func (am AllocationMap) GetTotalDemand() entities.Quantity {
	var total entities.Quantity
	for _, context := range am {
		total += context.AllocatedQty + context.RemainingDemand
	}
	return total
}

// GetCoverageRatio returns the overall allocation coverage ratio (0.0 to 1.0)
func (am AllocationMap) GetCoverageRatio() float64 {
	totalDemand := am.GetTotalDemand()
	if totalDemand == 0 {
		return 0.0
	}
	totalAllocated := am.GetTotalAllocated()
	return float64(totalAllocated) / float64(totalDemand)
}

// makeKey creates a consistent key for part number and location
func (am AllocationMap) makeKey(partNumber entities.PartNumber, location string) string {
	return fmt.Sprintf("%s|%s", partNumber, location)
}

// parseKey extracts part number and location from a key
func (am AllocationMap) parseKey(key string) (entities.PartNumber, string, bool) {
	for i, char := range key {
		if char == '|' {
			partNumber := entities.PartNumber(key[:i])
			location := key[i+1:]
			return partNumber, location, true
		}
	}
	return "", "", false
}

// String returns a string representation of the allocation map for debugging
func (am AllocationMap) String() string {
	if len(am) == 0 {
		return "AllocationMap{empty}"
	}

	result := fmt.Sprintf("AllocationMap{%d entries:\n", len(am))
	for key, context := range am {
		if partNumber, location, found := am.parseKey(key); found {
			result += fmt.Sprintf(
				"  %s@%s: allocated=%d, remaining=%d, hasAllocation=%t\n",
				partNumber,
				location,
				context.AllocatedQty,
				context.RemainingDemand,
				context.HasAllocation,
			)
		}
	}
	result += "}"
	return result
}
