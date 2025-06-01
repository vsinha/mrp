package memory

import (
	"fmt"
	"testing"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

func TestBOMRepository_SaveAndGetBOMLine(t *testing.T) {
	repo := NewBOMRepository(10)

	bomLine := &entities.BOMLine{
		ParentPN:    "ASSEMBLY_A",
		ChildPN:     "COMPONENT_B",
		QtyPer:      entities.Quantity(2),
		FindNumber:  100,
		Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: "SN999"},
	}

	// Save BOM line
	err := repo.SaveBOMLine(bomLine)
	if err != nil {
		t.Fatalf("Failed to save BOM line: %v", err)
	}

	// Get BOM lines for parent
	lines, err := repo.GetBOMLines("ASSEMBLY_A")
	if err != nil {
		t.Fatalf("Failed to get BOM lines: %v", err)
	}

	if len(lines) != 1 {
		t.Fatalf("Expected 1 BOM line, got %d", len(lines))
	}

	retrieved := lines[0]
	if retrieved.ParentPN != bomLine.ParentPN {
		t.Errorf("Expected parent %s, got %s", bomLine.ParentPN, retrieved.ParentPN)
	}

	if retrieved.ChildPN != bomLine.ChildPN {
		t.Errorf("Expected child %s, got %s", bomLine.ChildPN, retrieved.ChildPN)
	}

	if retrieved.QtyPer != bomLine.QtyPer {
		t.Errorf("Expected quantity %d, got %d", bomLine.QtyPer, retrieved.QtyPer)
	}
}

func TestBOMRepository_GetEffectiveLines(t *testing.T) {
	repo := NewBOMRepository(20)

	// Add BOM lines with different serial effectivities
	bomLines := []*entities.BOMLine{
		{
			ParentPN:    "ENGINE",
			ChildPN:     "TURBOPUMP_V1",
			QtyPer:      entities.Quantity(1),
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: "SN050"},
		},
		{
			ParentPN:    "ENGINE",
			ChildPN:     "TURBOPUMP_V2",
			QtyPer:      entities.Quantity(1),
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN051", ToSerial: ""},
		},
	}

	for _, bomLine := range bomLines {
		err := repo.SaveBOMLine(bomLine)
		if err != nil {
			t.Fatalf("Failed to save BOM line: %v", err)
		}
	}

	tests := []struct {
		name         string
		targetSerial string
		expectedPart string
	}{
		{
			name:         "early_serial_gets_v1",
			targetSerial: "SN025",
			expectedPart: "TURBOPUMP_V1",
		},
		{
			name:         "late_serial_gets_v2",
			targetSerial: "SN075",
			expectedPart: "TURBOPUMP_V2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effectiveLines, err := repo.GetEffectiveLines("ENGINE", tt.targetSerial)
			if err != nil {
				t.Fatalf("Failed to get effective lines: %v", err)
			}

			if len(effectiveLines) != 1 {
				t.Fatalf("Expected 1 effective line, got %d", len(effectiveLines))
			}

			if effectiveLines[0].ChildPN != entities.PartNumber(tt.expectedPart) {
				t.Errorf("Expected child %s, got %s", tt.expectedPart, effectiveLines[0].ChildPN)
			}
		})
	}
}

func TestBOMRepository_MultipleChildren(t *testing.T) {
	repo := NewBOMRepository(20)

	// Add BOM lines for multiple children
	for i := 1; i <= 3; i++ {
		bomLine := &entities.BOMLine{
			ParentPN:    "ASSEMBLY",
			ChildPN:     entities.PartNumber(fmt.Sprintf("CHILD_%d", i)),
			QtyPer:      entities.Quantity(i),
			FindNumber:  i * 100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		}

		err := repo.SaveBOMLine(bomLine)
		if err != nil {
			t.Fatalf("Failed to save BOM line: %v", err)
		}
	}

	// Get all BOM lines for assembly
	lines, err := repo.GetBOMLines("ASSEMBLY")
	if err != nil {
		t.Fatalf("Failed to get BOM lines: %v", err)
	}

	if len(lines) != 3 {
		t.Fatalf("Expected 3 BOM lines, got %d", len(lines))
	}

	// Verify each line
	for i, line := range lines {
		expectedChild := entities.PartNumber(fmt.Sprintf("CHILD_%d", i+1))
		if line.ChildPN != expectedChild {
			t.Errorf("Expected child %s at index %d, got %s", expectedChild, i, line.ChildPN)
		}

		expectedQty := entities.Quantity(i + 1)
		if line.QtyPer != expectedQty {
			t.Errorf("Expected quantity %d at index %d, got %d", expectedQty, i, line.QtyPer)
		}
	}
}

func TestBOMRepository_GetAlternateGroups(t *testing.T) {
	repo := NewBOMRepository(20)

	// Create test effectivity
	effectivity, err := entities.NewSerialEffectivity("AS501", "AS505")
	if err != nil {
		t.Fatalf("Failed to create serial effectivity: %v", err)
	}

	// Add BOM lines with same FindNumber (alternates)
	primary, err := entities.NewBOMLine("F1_ENGINE", "F1_TURBOPUMP_V1", 1, 300, *effectivity, 0)
	if err != nil {
		t.Fatalf("Failed to create primary BOM line: %v", err)
	}

	alternate, err := entities.NewBOMLine("F1_ENGINE", "F1_TURBOPUMP_V2", 1, 300, *effectivity, 1)
	if err != nil {
		t.Fatalf("Failed to create alternate BOM line: %v", err)
	}

	// Add a different FindNumber line
	different, err := entities.NewBOMLine("F1_ENGINE", "F1_IGNITER", 1, 400, *effectivity, 0)
	if err != nil {
		t.Fatalf("Failed to create different BOM line: %v", err)
	}

	repo.AddBOMLine(*primary)
	repo.AddBOMLine(*alternate)
	repo.AddBOMLine(*different)

	// Get alternate groups
	groups, err := repo.GetAlternateGroups("F1_ENGINE")
	if err != nil {
		t.Fatalf("Failed to get alternate groups: %v", err)
	}

	// Should have 2 groups: FindNumber 300 (2 alternates) and FindNumber 400 (1 line)
	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}

	// Check group 300 has 2 alternates
	group300, exists := groups[300]
	if !exists {
		t.Error("Expected group 300 to exist")
	} else if len(group300) != 2 {
		t.Errorf("Expected 2 alternates in group 300, got %d", len(group300))
	}

	// Check group 400 has 1 line
	group400, exists := groups[400]
	if !exists {
		t.Error("Expected group 400 to exist")
	} else if len(group400) != 1 {
		t.Errorf("Expected 1 line in group 400, got %d", len(group400))
	}
}

func TestBOMRepository_GetEffectiveAlternates(t *testing.T) {
	repo := NewBOMRepository(20)

	// Create test effectivities
	earlyEffectivity, err := entities.NewSerialEffectivity("AS501", "AS505")
	if err != nil {
		t.Fatalf("Failed to create early serial effectivity: %v", err)
	}

	lateEffectivity, err := entities.NewSerialEffectivity("AS506", "")
	if err != nil {
		t.Fatalf("Failed to create late serial effectivity: %v", err)
	}

	// Add alternates with different effectivities
	earlyPrimary, err := entities.NewBOMLine(
		"F1_ENGINE",
		"F1_TURBOPUMP_V1",
		1,
		300,
		*earlyEffectivity,
		0,
	)
	if err != nil {
		t.Fatalf("Failed to create early primary BOM line: %v", err)
	}

	latePrimary, err := entities.NewBOMLine(
		"F1_ENGINE",
		"F1_TURBOPUMP_V2",
		1,
		300,
		*lateEffectivity,
		0,
	)
	if err != nil {
		t.Fatalf("Failed to create late primary BOM line: %v", err)
	}

	// Add different FindNumber for comparison
	differentFind, err := entities.NewBOMLine(
		"F1_ENGINE",
		"F1_IGNITER",
		1,
		400,
		*earlyEffectivity,
		0,
	)
	if err != nil {
		t.Fatalf("Failed to create different FindNumber BOM line: %v", err)
	}

	repo.AddBOMLine(*earlyPrimary)
	repo.AddBOMLine(*latePrimary)
	repo.AddBOMLine(*differentFind)

	// Test: Early serial should get V1 turbopump
	earlyAlternates, err := repo.GetEffectiveAlternates("F1_ENGINE", 300, "AS503")
	if err != nil {
		t.Fatalf("Failed to get early effective alternates: %v", err)
	}

	if len(earlyAlternates) != 1 {
		t.Errorf("Expected 1 effective alternate for early serial, got %d", len(earlyAlternates))
	} else if earlyAlternates[0].ChildPN != "F1_TURBOPUMP_V1" {
		t.Errorf("Expected F1_TURBOPUMP_V1 for early serial, got %s", earlyAlternates[0].ChildPN)
	}

	// Test: Late serial should get V2 turbopump
	lateAlternates, err := repo.GetEffectiveAlternates("F1_ENGINE", 300, "AS507")
	if err != nil {
		t.Fatalf("Failed to get late effective alternates: %v", err)
	}

	if len(lateAlternates) != 1 {
		t.Errorf("Expected 1 effective alternate for late serial, got %d", len(lateAlternates))
	} else if lateAlternates[0].ChildPN != "F1_TURBOPUMP_V2" {
		t.Errorf("Expected F1_TURBOPUMP_V2 for late serial, got %s", lateAlternates[0].ChildPN)
	}

	// Test: Wrong FindNumber should return empty
	wrongFind, err := repo.GetEffectiveAlternates("F1_ENGINE", 999, "AS503")
	if err != nil {
		t.Fatalf("Failed to get alternates for wrong FindNumber: %v", err)
	}

	if len(wrongFind) != 0 {
		t.Errorf("Expected 0 alternates for wrong FindNumber, got %d", len(wrongFind))
	}
}

func TestBOMRepository_GetAlternateGroups_NonExistentPart(t *testing.T) {
	repo := NewBOMRepository(20)

	// Test: Non-existent part should return empty map
	groups, err := repo.GetAlternateGroups("NON_EXISTENT")
	if err != nil {
		t.Fatalf("Failed to get alternate groups for non-existent part: %v", err)
	}

	if len(groups) != 0 {
		t.Errorf("Expected empty groups for non-existent part, got %d", len(groups))
	}
}
