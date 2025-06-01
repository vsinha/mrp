package entities

// SerialEffectivity defines the range of serials for which a BOM line is effective
type SerialEffectivity struct {
	FromSerial string
	ToSerial   string // empty = open ended
}

// BOMLine represents a single line in a Bill of Materials
type BOMLine struct {
	ParentPN    PartNumber
	ChildPN     PartNumber
	QtyPer      Quantity
	FindNumber  int
	Effectivity SerialEffectivity
}
