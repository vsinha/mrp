package entities

import (
	"testing"
	"time"
)

func TestInventoryLot_Validation(t *testing.T) {
	receiptDate := time.Now()

	validLot, err := NewInventoryLot("PART123", "LOT001", "WAREHOUSE", 10, receiptDate, Available)
	if err != nil {
		t.Fatalf("Expected valid lot creation to succeed: %v", err)
	}
	if validLot.Quantity != 10 {
		t.Errorf("Expected quantity 10, got %d", validLot.Quantity)
	}

	// Test validation failures
	testCases := []struct {
		name        string
		partNumber  PartNumber
		lotNumber   string
		location    string
		quantity    Quantity
		expectError string
	}{
		{"empty part number", "", "LOT001", "WAREHOUSE", 10, "part number cannot be empty"},
		{"empty lot number", "PART123", "", "WAREHOUSE", 10, "lot number cannot be empty"},
		{"empty location", "PART123", "LOT001", "", 10, "location cannot be empty"},
		{"negative quantity", "PART123", "LOT001", "WAREHOUSE", -5, "quantity cannot be negative, got -5"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewInventoryLot(tc.partNumber, tc.lotNumber, tc.location, tc.quantity, receiptDate, Available)
			if err == nil {
				t.Fatalf("Expected error for %s, but got none", tc.name)
			}
			if err.Error() != tc.expectError {
				t.Errorf("Expected error '%s', got '%s'", tc.expectError, err.Error())
			}
		})
	}
}

func TestSerializedInventory_Validation(t *testing.T) {
	receiptDate := time.Now()

	validSerial, err := NewSerializedInventory("PART123", "SN001", "WAREHOUSE", Available, receiptDate)
	if err != nil {
		t.Fatalf("Expected valid serialized inventory creation to succeed: %v", err)
	}
	if validSerial.SerialNumber != "SN001" {
		t.Errorf("Expected serial number SN001, got %s", validSerial.SerialNumber)
	}

	// Test validation failures
	testCases := []struct {
		name         string
		partNumber   PartNumber
		serialNumber string
		location     string
		expectError  string
	}{
		{"empty part number", "", "SN001", "WAREHOUSE", "part number cannot be empty"},
		{"empty serial number", "PART123", "", "WAREHOUSE", "serial number cannot be empty"},
		{"empty location", "PART123", "SN001", "", "location cannot be empty"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSerializedInventory(tc.partNumber, tc.serialNumber, tc.location, Available, receiptDate)
			if err == nil {
				t.Fatalf("Expected error for %s, but got none", tc.name)
			}
			if err.Error() != tc.expectError {
				t.Errorf("Expected error '%s', got '%s'", tc.expectError, err.Error())
			}
		})
	}
}
