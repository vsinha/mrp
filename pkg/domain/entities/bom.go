package entities

import "fmt"

// SerialEffectivity defines the range of serials for which a BOM line is effective
type SerialEffectivity struct {
	FromSerial string
	ToSerial   string // empty = open ended
}

// NewSerialEffectivity creates a validated SerialEffectivity
func NewSerialEffectivity(fromSerial, toSerial string) (*SerialEffectivity, error) {
	if fromSerial == "" {
		return nil, fmt.Errorf("from serial cannot be empty")
	}
	// Note: toSerial can be empty for open-ended ranges
	// TODO: Add serial comparison validation when we have access to SerialComparator

	return &SerialEffectivity{
		FromSerial: fromSerial,
		ToSerial:   toSerial,
	}, nil
}

// BOMLine represents a single line in a Bill of Materials
type BOMLine struct {
	ParentPN    PartNumber
	ChildPN     PartNumber
	QtyPer      Quantity
	FindNumber  int
	Effectivity SerialEffectivity

	// AlternateGroup groups multiple BOM lines that can substitute for each other.
	// Lines with the same AlternateGroup, ParentPN, and FindNumber represent
	// interchangeable parts. Empty string means no alternates (standard BOM line).
	//
	// Example: "F1_TURBOPUMP_ALT" groups F1_TURBOPUMP_V1 and F1_TURBOPUMP_V2
	// as alternates that can both fulfill the turbopump requirement on F1_ENGINE.
	AlternateGroup string

	// Priority determines selection order within an AlternateGroup.
	// Lower numbers = higher priority (1 = primary, 2 = first alternate, etc.).
	// When multiple lines have the same priority, they are equally preferred.
	//
	// MRP logic selects the highest priority alternate that satisfies the serial
	// effectivity for the target serial number. Inventory availability may also
	// influence selection within the same priority level.
	Priority int
}

// NewBOMLine creates a validated BOMLine
func NewBOMLine(parentPN, childPN PartNumber, qtyPer Quantity, findNumber int, effectivity SerialEffectivity, alternateGroup string, priority int) (*BOMLine, error) {
	if string(parentPN) == "" {
		return nil, fmt.Errorf("parent part number cannot be empty")
	}
	if string(childPN) == "" {
		return nil, fmt.Errorf("child part number cannot be empty")
	}
	if parentPN == childPN {
		return nil, fmt.Errorf("parent and child part numbers cannot be the same: %s", parentPN)
	}
	if qtyPer <= 0 {
		return nil, fmt.Errorf("quantity per must be positive, got %d", qtyPer)
	}
	if findNumber <= 0 {
		return nil, fmt.Errorf("find number must be positive, got %d", findNumber)
	}
	if priority < 0 {
		return nil, fmt.Errorf("priority cannot be negative, got %d", priority)
	}

	return &BOMLine{
		ParentPN:       parentPN,
		ChildPN:        childPN,
		QtyPer:         qtyPer,
		FindNumber:     findNumber,
		Effectivity:    effectivity,
		AlternateGroup: alternateGroup,
		Priority:       priority,
	}, nil
}
