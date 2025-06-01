package entities

import "testing"

func TestBOMLine_Validation(t *testing.T) {
	effectivity := SerialEffectivity{FromSerial: "SN001", ToSerial: ""}

	validBOM, err := NewBOMLine("PARENT", "CHILD", 2, 100, effectivity)
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
			_, err := NewBOMLine(tc.parentPN, tc.childPN, tc.qtyPer, tc.findNumber, effectivity)
			if err == nil {
				t.Fatalf("Expected error for %s, but got none", tc.name)
			}
			if err.Error() != tc.expectError {
				t.Errorf("Expected error '%s', got '%s'", tc.expectError, err.Error())
			}
		})
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
