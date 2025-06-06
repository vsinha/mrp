package entities

import "testing"

func TestItem_Validation(t *testing.T) {
	validItem, err := NewItem("PART123", "Test Part", 10, LotForLot, 1, 100, 0, "EA", MakeBuyMake)
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
		maxOrderQty Quantity
		safetyStock Quantity
		uom         string
		expectError string
		makeBuyCode MakeBuyCode
	}{
		{"empty part number", "", "desc", 1, LotForLot, 0, 100, 0, "EA", "part number cannot be empty", MakeBuyMake},
		{"empty description", "PART", "", 1, LotForLot, 0, 100, 0, "EA", "description cannot be empty",
			MakeBuyMake,
		},
		{
			"zero lead time",
			"PART",
			"desc",
			0,
			LotForLot,
			0,
			100,
			0,
			"EA",
			"lead time must be positive, got 0",
			MakeBuyMake,
		},
		{
			"negative lead time",
			"PART",
			"desc",
			-1,
			LotForLot,
			0,
			100,
			0,
			"EA",
			"lead time must be positive, got -1",
			MakeBuyMake,
		},
		{
			"negative min order qty",
			"PART",
			"desc",
			1,
			LotForLot,
			-1,
			100,
			0,
			"EA",
			"minimum order quantity cannot be negative, got -1",
			MakeBuyMake,
		},
		{
			"negative safety stock",
			"PART",
			"desc",
			1,
			LotForLot,
			0,
			100,
			-1,
			"EA",
			"safety stock cannot be negative, got -1",
			MakeBuyMake,
		},
		{"empty UOM", "PART", "desc", 1, LotForLot, 0, 100, 0, "", "unit of measure cannot be empty", MakeBuyMake},
		{
			"MinimumQty with zero order qty",
			"PART",
			"desc",
			1,
			MinimumQty,
			0,
			100,
			0,
			"EA",
			"lot sizing rule MinimumQty requires non-zero minimum order quantity",
			MakeBuyMake,
		},
		{
			"zero max order qty",
			"PART",
			"desc",
			1,
			LotForLot,
			1,
			0,
			0,
			"EA",
			"maximum order quantity must be positive, got 0",
			MakeBuyMake,
		},
		{
			"negative max order qty",
			"PART",
			"desc",
			1,
			LotForLot,
			1,
			-1,
			0,
			"EA",
			"maximum order quantity must be positive, got -1",
			MakeBuyMake,
		},
		{
			"max order qty less than min order qty",
			"PART",
			"desc",
			1,
			LotForLot,
			10,
			5,
			0,
			"EA",
			"maximum order quantity (5) cannot be less than minimum order quantity (10)",
			MakeBuyMake,
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
				tc.maxOrderQty,
				tc.safetyStock,
				tc.uom,
				tc.makeBuyCode,
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
