package bom_validator

import (
	"testing"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

func TestBOMValidator_DetectSimpleCycle(t *testing.T) {
	// Create a simple cycle: A -> B -> A
	effectivity, _ := entities.NewSerialEffectivity("SN001", "")
	bomLines := []entities.BOMLine{
		{
			ParentPN:    "A",
			ChildPN:     "B",
			QtyPer:      1,
			FindNumber:  100,
			Effectivity: *effectivity,
			Priority:    0,
		},
		{
			ParentPN:    "B",
			ChildPN:     "A",
			QtyPer:      1,
			FindNumber:  100,
			Effectivity: *effectivity,
			Priority:    0,
		},
	}

	result := ValidateBOM(bomLines)

	if !result.HasCycles {
		t.Error("Expected cycle to be detected")
	}

	if len(result.CyclePaths) == 0 {
		t.Error("Expected at least one cycle path")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected validation errors for cycles")
	}
}

func TestBOMValidator_DetectLongerCycle(t *testing.T) {
	// Create a longer cycle: A -> B -> C -> A
	effectivity, _ := entities.NewSerialEffectivity("SN001", "")
	bomLines := []entities.BOMLine{
		{
			ParentPN:    "A",
			ChildPN:     "B",
			QtyPer:      1,
			FindNumber:  100,
			Effectivity: *effectivity,
			Priority:    0,
		},
		{
			ParentPN:    "B",
			ChildPN:     "C",
			QtyPer:      1,
			FindNumber:  100,
			Effectivity: *effectivity,
			Priority:    0,
		},
		{
			ParentPN:    "C",
			ChildPN:     "A",
			QtyPer:      1,
			FindNumber:  100,
			Effectivity: *effectivity,
			Priority:    0,
		},
	}

	result := ValidateBOM(bomLines)

	if !result.HasCycles {
		t.Error("Expected cycle to be detected")
	}

	if len(result.CyclePaths) == 0 {
		t.Error("Expected at least one cycle path")
	}
}

func TestBOMValidator_NoCycles(t *testing.T) {
	// Create a valid tree structure: A -> B, A -> C, B -> D
	effectivity, _ := entities.NewSerialEffectivity("SN001", "")
	bomLines := []entities.BOMLine{
		{
			ParentPN:    "A",
			ChildPN:     "B",
			QtyPer:      1,
			FindNumber:  100,
			Effectivity: *effectivity,
			Priority:    0,
		},
		{
			ParentPN:    "A",
			ChildPN:     "C",
			QtyPer:      1,
			FindNumber:  200,
			Effectivity: *effectivity,
			Priority:    0,
		},
		{
			ParentPN:    "B",
			ChildPN:     "D",
			QtyPer:      1,
			FindNumber:  100,
			Effectivity: *effectivity,
			Priority:    0,
		},
	}

	result := ValidateBOM(bomLines)

	if result.HasCycles {
		t.Error("Expected no cycles to be detected")
	}

	if len(result.CyclePaths) > 0 {
		t.Errorf("Expected no cycle paths, got %d", len(result.CyclePaths))
	}

	if len(result.Errors) > 0 {
		t.Errorf("Expected no validation errors, got %v", result.Errors)
	}
}

func TestBOMValidator_DetectDuplicateLines(t *testing.T) {
	effectivity, _ := entities.NewSerialEffectivity("SN001", "")

	// Create duplicate BOM lines (same parent, child, find number)
	bomLines := []entities.BOMLine{
		{
			ParentPN:    "A",
			ChildPN:     "B",
			QtyPer:      1,
			FindNumber:  100,
			Effectivity: *effectivity,
			Priority:    0,
		},
		{
			ParentPN:    "A",
			ChildPN:     "B",
			QtyPer:      2, // Different quantity, but same key fields
			FindNumber:  100,
			Effectivity: *effectivity,
			Priority:    0,
		},
	}

	result := ValidateBOM(bomLines)

	if len(result.DuplicateLines) == 0 {
		t.Error("Expected duplicate lines to be detected")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected validation errors for duplicates")
	}
}

func TestBOMValidator_ValidAlternates(t *testing.T) {
	effectivity, _ := entities.NewSerialEffectivity("SN001", "")

	// Create valid alternates (same parent and find number, different child)
	bomLines := []entities.BOMLine{
		{
			ParentPN:    "A",
			ChildPN:     "B",
			QtyPer:      1,
			FindNumber:  100,
			Effectivity: *effectivity,
			Priority:    1, // Primary
		},
		{
			ParentPN:    "A",
			ChildPN:     "C",
			QtyPer:      1,
			FindNumber:  100, // Same find number
			Effectivity: *effectivity,
			Priority:    2, // Alternate
		},
	}

	result := ValidateBOM(bomLines)

	// These should be valid alternates, not duplicates
	if len(result.DuplicateLines) > 0 {
		t.Error("Valid alternates should not be flagged as duplicates")
	}

	if result.HasCycles {
		t.Error("Valid alternates should not create cycles")
	}
}

func TestBOMValidator_PartNumberUniqueness(t *testing.T) {

	// Create items with duplicate part numbers
	items := []entities.Item{
		{PartNumber: "PART_A", Description: "Part A", UnitOfMeasure: "EA"},
		{PartNumber: "PART_B", Description: "Part B", UnitOfMeasure: "EA"},
		{PartNumber: "PART_A", Description: "Duplicate Part A", UnitOfMeasure: "EA"}, // Duplicate
	}

	result := ValidatePartNumberUniqueness(items)

	if len(result.Errors) == 0 {
		t.Error("Expected validation errors for duplicate part numbers")
	}
}

func TestBOMValidator_UniquePartNumbers(t *testing.T) {

	// Create items with unique part numbers
	items := []entities.Item{
		{PartNumber: "PART_A", Description: "Part A", UnitOfMeasure: "EA"},
		{PartNumber: "PART_B", Description: "Part B", UnitOfMeasure: "EA"},
		{PartNumber: "PART_C", Description: "Part C", UnitOfMeasure: "EA"},
	}

	result := ValidatePartNumberUniqueness(items)

	if len(result.Errors) > 0 {
		t.Errorf("Expected no validation errors for unique part numbers, got %v", result.Errors)
	}
}

func TestBOMValidator_EmptyBOM(t *testing.T) {
	result := ValidateBOM([]entities.BOMLine{})

	if result.HasCycles {
		t.Error("Empty BOM should not have cycles")
	}

	if len(result.DuplicateLines) > 0 {
		t.Error("Empty BOM should not have duplicates")
	}

	if len(result.Errors) > 0 {
		t.Error("Empty BOM should have no validation errors")
	}
}
