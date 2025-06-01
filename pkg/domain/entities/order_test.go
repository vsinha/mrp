package entities

import (
	"testing"
	"time"
)

func TestPlannedOrder_Validation(t *testing.T) {
	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	dueDate := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)

	validOrder, err := NewPlannedOrder(
		"PART123",
		5,
		startDate,
		dueDate,
		"test",
		"FACTORY",
		Make,
		"SN001",
	)
	if err != nil {
		t.Fatalf("Expected valid order creation to succeed: %v", err)
	}
	if validOrder.Quantity != 5 {
		t.Errorf("Expected quantity 5, got %d", validOrder.Quantity)
	}

	// Test validation failures
	testCases := []struct {
		name        string
		partNumber  PartNumber
		quantity    Quantity
		startDate   time.Time
		dueDate     time.Time
		location    string
		expectError string
	}{
		{"empty part number", "", 5, startDate, dueDate, "FACTORY", "part number cannot be empty"},
		{
			"zero quantity",
			"PART",
			0,
			startDate,
			dueDate,
			"FACTORY",
			"quantity must be positive, got 0",
		},
		{
			"negative quantity",
			"PART",
			-1,
			startDate,
			dueDate,
			"FACTORY",
			"quantity must be positive, got -1",
		},
		{
			"start after due",
			"PART",
			5,
			dueDate,
			startDate,
			"FACTORY",
			"start date 2025-01-10 00:00:00 +0000 UTC cannot be after due date 2025-01-01 00:00:00 +0000 UTC",
		},
		{"empty location", "PART", 5, startDate, dueDate, "", "location cannot be empty"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPlannedOrder(
				tc.partNumber,
				tc.quantity,
				tc.startDate,
				tc.dueDate,
				"test",
				tc.location,
				Make,
				"SN001",
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
