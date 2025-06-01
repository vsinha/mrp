package entities

import "testing"

func TestItem_Validation(t *testing.T) {
	validItem, err := NewItem("PART123", "Test Part", 10, LotForLot, 1, 0, "EA")
	if err != nil {
		t.Fatalf("Expected valid item creation to succeed: %v", err)
	}
	if validItem.PartNumber != "PART123" {
		t.Errorf("Expected part number PART123, got %s", validItem.PartNumber)
	}

	// Test validation failures
	testCases := []struct {
		name        string
		partNumber  PartNumber
		description string
		leadTime    int
		lotRule     LotSizeRule
		minOrderQty Quantity
		safetyStock Quantity
		uom         string
		expectError string
	}{
		{"empty part number", "", "desc", 1, LotForLot, 0, 0, "EA", "part number cannot be empty"},
		{"empty description", "PART", "", 1, LotForLot, 0, 0, "EA", "description cannot be empty"},
		{
			"zero lead time",
			"PART",
			"desc",
			0,
			LotForLot,
			0,
			0,
			"EA",
			"lead time must be positive, got 0",
		},
		{
			"negative lead time",
			"PART",
			"desc",
			-1,
			LotForLot,
			0,
			0,
			"EA",
			"lead time must be positive, got -1",
		},
		{
			"negative min order qty",
			"PART",
			"desc",
			1,
			LotForLot,
			-1,
			0,
			"EA",
			"minimum order quantity cannot be negative, got -1",
		},
		{
			"negative safety stock",
			"PART",
			"desc",
			1,
			LotForLot,
			0,
			-1,
			"EA",
			"safety stock cannot be negative, got -1",
		},
		{"empty UOM", "PART", "desc", 1, LotForLot, 0, 0, "", "unit of measure cannot be empty"},
		{
			"MinimumQty with zero order qty",
			"PART",
			"desc",
			1,
			MinimumQty,
			0,
			0,
			"EA",
			"lot sizing rule MinimumQty requires non-zero minimum order quantity",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewItem(
				tc.partNumber,
				tc.description,
				tc.leadTime,
				tc.lotRule,
				tc.minOrderQty,
				tc.safetyStock,
				tc.uom,
			)
			if err == nil {
				t.Fatalf("Expected error for %s, but got none", tc.name)
			}
			if err.Error() != tc.expectError {
				t.Errorf("Expected error '%s', got '%s'", tc.expectError, err.Error())
			}
		})
	}
}
