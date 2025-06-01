package entities

import "testing"

func TestBOMLine_Validation(t *testing.T) {
	effectivity := SerialEffectivity{FromSerial: "SN001", ToSerial: ""}

	validBOM, err := NewBOMLine("PARENT", "CHILD", 2, 100, effectivity, "", 0)
	if err != nil {
		t.Fatalf("Expected valid BOM creation to succeed: %v", err)
	}
	if validBOM.QtyPer != 2 {
		t.Errorf("Expected quantity per 2, got %d", validBOM.QtyPer)
	}

	// Test validation failures
	testCases := []struct {
		name        string
		parentPN    PartNumber
		childPN     PartNumber
		qtyPer      Quantity
		findNumber  int
		expectError string
	}{
		{"empty parent", "", "CHILD", 1, 100, "parent part number cannot be empty"},
		{"empty child", "PARENT", "", 1, 100, "child part number cannot be empty"},
		{"parent equals child", "SAME_PART", "SAME_PART", 1, 100, "parent and child part numbers cannot be the same: SAME_PART"},
		{"zero quantity", "PARENT", "CHILD", 0, 100, "quantity per must be positive, got 0"},
		{"negative quantity", "PARENT", "CHILD", -1, 100, "quantity per must be positive, got -1"},
		{"zero find number", "PARENT", "CHILD", 1, 0, "find number must be positive, got 0"},
		{"negative find number", "PARENT", "CHILD", 1, -1, "find number must be positive, got -1"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewBOMLine(tc.parentPN, tc.childPN, tc.qtyPer, tc.findNumber, effectivity, "", 0)
			if err == nil {
				t.Fatalf("Expected error for %s, but got none", tc.name)
			}
			if err.Error() != tc.expectError {
				t.Errorf("Expected error '%s', got '%s'", tc.expectError, err.Error())
			}
		})
	}

	// Test negative priority
	_, err = NewBOMLine("PARENT", "CHILD", 1, 100, effectivity, "", -1)
	if err == nil {
		t.Fatal("Expected error for negative priority")
	}
	if err.Error() != "priority cannot be negative, got -1" {
		t.Errorf("Expected 'priority cannot be negative, got -1', got '%s'", err.Error())
	}
}

func TestBOMLine_AlternateFields(t *testing.T) {
	effectivity := SerialEffectivity{FromSerial: "SN001", ToSerial: ""}

	// Test valid alternate BOM line
	bomLine, err := NewBOMLine("F1_ENGINE", "F1_TURBOPUMP_V1", 1, 300, effectivity, "TURBOPUMP_ALT", 1)
	if err != nil {
		t.Fatalf("Expected valid alternate BOM creation to succeed: %v", err)
	}

	if bomLine.AlternateGroup != "TURBOPUMP_ALT" {
		t.Errorf("Expected alternate group 'TURBOPUMP_ALT', got '%s'", bomLine.AlternateGroup)
	}
	if bomLine.Priority != 1 {
		t.Errorf("Expected priority 1, got %d", bomLine.Priority)
	}

	// Test empty alternate group (standard BOM line)
	standardBOM, err := NewBOMLine("PARENT", "CHILD", 1, 100, effectivity, "", 0)
	if err != nil {
		t.Fatalf("Expected standard BOM creation to succeed: %v", err)
	}

	if standardBOM.AlternateGroup != "" {
		t.Errorf("Expected empty alternate group, got '%s'", standardBOM.AlternateGroup)
	}
	if standardBOM.Priority != 0 {
		t.Errorf("Expected priority 0, got %d", standardBOM.Priority)
	}
}

func TestSerialEffectivity_Validation(t *testing.T) {
	validEffectivity, err := NewSerialEffectivity("SN001", "SN100")
	if err != nil {
		t.Fatalf("Expected valid effectivity creation to succeed: %v", err)
	}
	if validEffectivity.FromSerial != "SN001" {
		t.Errorf("Expected from serial SN001, got %s", validEffectivity.FromSerial)
	}

	// Test open-ended range
	openEnded, err := NewSerialEffectivity("SN001", "")
	if err != nil {
		t.Fatalf("Expected valid open-ended effectivity creation to succeed: %v", err)
	}
	if openEnded.ToSerial != "" {
		t.Errorf("Expected empty to serial for open-ended range, got %s", openEnded.ToSerial)
	}

	// Test empty from serial
	_, err = NewSerialEffectivity("", "SN100")
	if err == nil {
		t.Fatal("Expected error for empty from serial")
	}
	if err.Error() != "from serial cannot be empty" {
		t.Errorf("Expected 'from serial cannot be empty', got '%s'", err.Error())
	}
}

func TestBOMAlternatesExample(t *testing.T) {
	// Example: F-1 engine can use different turbopump versions based on serial effectivity

	// Primary turbopump for early Apollo missions (AS501-AS505)
	turbopumpV1, err := NewBOMLine(
		"F1_ENGINE",
		"F1_TURBOPUMP_V1",
		1,
		300,
		SerialEffectivity{FromSerial: "AS501", ToSerial: "AS505"},
		"F1_TURBOPUMP_ALT", // Alternate group
		1,                  // Priority 1 (primary)
	)
	if err != nil {
		t.Fatalf("Failed to create turbopump V1 BOM line: %v", err)
	}

	// Primary turbopump for later Apollo missions (AS506+)
	turbopumpV2, err := NewBOMLine(
		"F1_ENGINE",
		"F1_TURBOPUMP_V2",
		1,
		300,
		SerialEffectivity{FromSerial: "AS506", ToSerial: ""},
		"F1_TURBOPUMP_ALT", // Same alternate group
		1,                  // Priority 1 (also primary)
	)
	if err != nil {
		t.Fatalf("Failed to create turbopump V2 BOM line: %v", err)
	}

	// Backup turbopump that works for all serials (emergency substitute)
	turbopumpBackup, err := NewBOMLine(
		"F1_ENGINE",
		"F1_TURBOPUMP_BACKUP",
		1,
		300,
		SerialEffectivity{FromSerial: "AS501", ToSerial: ""},
		"F1_TURBOPUMP_ALT", // Same alternate group
		2,                  // Priority 2 (backup)
	)
	if err != nil {
		t.Fatalf("Failed to create backup turbopump BOM line: %v", err)
	}

	// Verify the BOM lines are configured correctly
	bomLines := []*BOMLine{turbopumpV1, turbopumpV2, turbopumpBackup}

	// All should have the same parent, find number, and alternate group
	for _, line := range bomLines {
		if line.ParentPN != "F1_ENGINE" {
			t.Errorf("Expected parent F1_ENGINE, got %s", line.ParentPN)
		}
		if line.FindNumber != 300 {
			t.Errorf("Expected find number 300, got %d", line.FindNumber)
		}
		if line.AlternateGroup != "F1_TURBOPUMP_ALT" {
			t.Errorf("Expected alternate group F1_TURBOPUMP_ALT, got %s", line.AlternateGroup)
		}
	}

	// Verify priority differences
	if turbopumpV1.Priority != 1 {
		t.Errorf("Expected V1 priority 1, got %d", turbopumpV1.Priority)
	}
	if turbopumpV2.Priority != 1 {
		t.Errorf("Expected V2 priority 1, got %d", turbopumpV2.Priority)
	}
	if turbopumpBackup.Priority != 2 {
		t.Errorf("Expected backup priority 2, got %d", turbopumpBackup.Priority)
	}

	// Verify different part numbers
	partNumbers := map[PartNumber]bool{}
	for _, line := range bomLines {
		if partNumbers[line.ChildPN] {
			t.Errorf("Duplicate part number in alternates: %s", line.ChildPN)
		}
		partNumbers[line.ChildPN] = true
	}

	t.Logf("✅ Successfully created alternate group with %d parts:", len(bomLines))
	for _, line := range bomLines {
		t.Logf("  - %s (priority %d, effective %s-%s)",
			line.ChildPN, line.Priority, line.Effectivity.FromSerial, line.Effectivity.ToSerial)
	}
}
