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
	ParentPN PartNumber
	ChildPN  PartNumber
	QtyPer   Quantity

	// FindNumber identifies the physical location/position where this part is installed
	// on the parent assembly. It's like a "slot number" that corresponds to assembly
	// drawings, installation procedures, and maintenance documentation.
	//
	// Example: On F1_ENGINE, FindNumber 300 might be the turbopump mounting location.
	// All parts that can be installed at the same physical position share the same
	// FindNumber, making it useful for grouping alternates and assembly instructions.
	FindNumber int

	Effectivity SerialEffectivity

	// Priority determines selection order for alternates at the same FindNumber.
	// Multiple BOM lines with the same ParentPN and FindNumber represent alternates.
	// Lower numbers = higher priority (1 = primary, 2 = first alternate, etc.).
	// Priority 0 means standard BOM line (no alternates).
	//
	// Example: All F1_TURBOPUMP parts at FindNumber 300 compete as alternates,
	// with MRP selecting the highest priority part that satisfies serial effectivity.
	// Inventory availability may also influence selection within the same priority level.
	Priority int
}

// NewBOMLine creates a validated BOMLine
func NewBOMLine(
	parentPN, childPN PartNumber,
	qtyPer Quantity,
	findNumber int,
	effectivity SerialEffectivity,
	priority int,
) (*BOMLine, error) {
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
		ParentPN:    parentPN,
		ChildPN:     childPN,
		QtyPer:      qtyPer,
		FindNumber:  findNumber,
		Effectivity: effectivity,
		Priority:    priority,
	}, nil
}
