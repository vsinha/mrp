package services

import (
	"testing"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

func TestSerialComparator_CompareSerials(t *testing.T) {
	sc := NewSerialComparator()

	tests := []struct {
		name     string
		serial1  string
		serial2  string
		expected int
	}{
		{"equal_serials", "SN001", "SN001", 0},
		{"first_less_than_second", "SN001", "SN002", -1},
		{"first_greater_than_second", "SN002", "SN001", 1},
		{"numeric_ordering", "SN009", "SN010", -1},
		{"larger_numbers", "SN100", "SN099", 1},
		{"different_prefixes", "SN001", "TN001", -1},
		{"invalid_format_fallback", "INVALID", "SN001", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sc.CompareSerials(tt.serial1, tt.serial2)
			if result != tt.expected {
				t.Errorf("CompareSerials(%s, %s) = %d, want %d",
					tt.serial1, tt.serial2, result, tt.expected)
			}
		})
	}
}

func TestSerialComparator_IsSerialInRange(t *testing.T) {
	sc := NewSerialComparator()

	tests := []struct {
		name         string
		targetSerial string
		effectivity  entities.SerialEffectivity
		expected     bool
	}{
		{
			name:         "in_closed_range",
			targetSerial: "SN025",
			effectivity:  entities.SerialEffectivity{FromSerial: "SN001", ToSerial: "SN050"},
			expected:     true,
		},
		{
			name:         "above_closed_range",
			targetSerial: "SN075",
			effectivity:  entities.SerialEffectivity{FromSerial: "SN001", ToSerial: "SN050"},
			expected:     false,
		},
		{
			name:         "below_closed_range",
			targetSerial: "SN001",
			effectivity:  entities.SerialEffectivity{FromSerial: "SN010", ToSerial: "SN050"},
			expected:     false,
		},
		{
			name:         "in_open_range",
			targetSerial: "SN100",
			effectivity:  entities.SerialEffectivity{FromSerial: "SN051", ToSerial: ""},
			expected:     true,
		},
		{
			name:         "below_open_range",
			targetSerial: "SN040",
			effectivity:  entities.SerialEffectivity{FromSerial: "SN051", ToSerial: ""},
			expected:     false,
		},
		{
			name:         "at_range_start",
			targetSerial: "SN001",
			effectivity:  entities.SerialEffectivity{FromSerial: "SN001", ToSerial: "SN050"},
			expected:     true,
		},
		{
			name:         "at_range_end",
			targetSerial: "SN050",
			effectivity:  entities.SerialEffectivity{FromSerial: "SN001", ToSerial: "SN050"},
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sc.IsSerialInRange(tt.targetSerial, tt.effectivity)
			if result != tt.expected {
				t.Errorf("IsSerialInRange(%s, %+v) = %t, want %t",
					tt.targetSerial, tt.effectivity, result, tt.expected)
			}
		})
	}
}

func TestSerialComparator_ResolveSerialEffectivity(t *testing.T) {
	sc := NewSerialComparator()

	bomLines := []*entities.BOMLine{
		{
			ParentPN:    "ENGINE",
			ChildPN:     "TURBOPUMP_V1",
			QtyPer:      1,
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: "SN024"},
		},
		{
			ParentPN:    "ENGINE",
			ChildPN:     "TURBOPUMP_V2",
			QtyPer:      1,
			FindNumber:  100,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN025", ToSerial: ""},
		},
		{
			ParentPN:    "ENGINE",
			ChildPN:     "COMMON_PART",
			QtyPer:      1,
			FindNumber:  200,
			Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: ""},
		},
	}

	tests := []struct {
		name          string
		targetSerial  string
		expectedParts []string
	}{
		{
			name:          "early_serial_gets_v1",
			targetSerial:  "SN020",
			expectedParts: []string{"TURBOPUMP_V1", "COMMON_PART"},
		},
		{
			name:          "late_serial_gets_v2",
			targetSerial:  "SN030",
			expectedParts: []string{"TURBOPUMP_V2", "COMMON_PART"},
		},
		{
			name:          "boundary_serial_gets_v1",
			targetSerial:  "SN024",
			expectedParts: []string{"TURBOPUMP_V1", "COMMON_PART"},
		},
		{
			name:          "boundary_serial_gets_v2",
			targetSerial:  "SN025",
			expectedParts: []string{"TURBOPUMP_V2", "COMMON_PART"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sc.ResolveSerialEffectivity(tt.targetSerial, bomLines)

			if len(result) != len(tt.expectedParts) {
				t.Fatalf("Expected %d effective lines, got %d",
					len(tt.expectedParts), len(result))
			}

			for i, expectedPart := range tt.expectedParts {
				if string(result[i].ChildPN) != expectedPart {
					t.Errorf("Expected part %s at index %d, got %s",
						expectedPart, i, result[i].ChildPN)
				}
			}
		})
	}
}

func TestSerialComparator_ValidateSerialEffectivity(t *testing.T) {
	sc := NewSerialComparator()

	tests := []struct {
		name      string
		bomLines  []*entities.BOMLine
		expectErr bool
	}{
		{
			name: "no_overlaps",
			bomLines: []*entities.BOMLine{
				{
					ParentPN: "ENGINE", ChildPN: "PART_A",
					Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: "SN050"},
				},
				{
					ParentPN: "ENGINE", ChildPN: "PART_B",
					Effectivity: entities.SerialEffectivity{FromSerial: "SN051", ToSerial: ""},
				},
			},
			expectErr: false,
		},
		{
			name: "overlapping_ranges",
			bomLines: []*entities.BOMLine{
				{
					ParentPN: "ENGINE", ChildPN: "PART_A",
					Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: "SN050"},
				},
				{
					ParentPN: "ENGINE", ChildPN: "PART_A",
					Effectivity: entities.SerialEffectivity{FromSerial: "SN040", ToSerial: "SN060"},
				},
			},
			expectErr: true,
		},
		{
			name: "different_parent_child_ok",
			bomLines: []*entities.BOMLine{
				{
					ParentPN: "ENGINE", ChildPN: "PART_A",
					Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: "SN050"},
				},
				{
					ParentPN: "ENGINE", ChildPN: "PART_B",
					Effectivity: entities.SerialEffectivity{FromSerial: "SN001", ToSerial: "SN050"},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sc.ValidateSerialEffectivity(tt.bomLines)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}
