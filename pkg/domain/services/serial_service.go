package services

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

// SerialComparator handles serial number comparison and effectivity resolution
type SerialComparator struct {
	serialPattern *regexp.Regexp
}

// NewSerialComparator creates a new serial comparator with the default pattern
func NewSerialComparator() *SerialComparator {
	// Pattern matches serials like SN001, SN123, etc.
	pattern := regexp.MustCompile(`^([A-Z]+)(\d+)$`)
	return &SerialComparator{
		serialPattern: pattern,
	}
}

// IsSerialInRange checks if a target serial falls within the effectivity range
func (sc *SerialComparator) IsSerialInRange(targetSerial string, effectivity entities.SerialEffectivity) bool {
	// Handle open-ended ranges
	if effectivity.ToSerial == "" {
		return sc.CompareSerials(targetSerial, effectivity.FromSerial) >= 0
	}

	// Check if target falls within range
	return sc.CompareSerials(targetSerial, effectivity.FromSerial) >= 0 &&
		sc.CompareSerials(targetSerial, effectivity.ToSerial) <= 0
}

// CompareSerials compares two serial numbers with numeric sorting
// Returns: -1 if serial1 < serial2, 0 if equal, 1 if serial1 > serial2
func (sc *SerialComparator) CompareSerials(serial1, serial2 string) int {
	// First try direct string comparison for identical prefixes
	if serial1 == serial2 {
		return 0
	}

	// Extract prefix and numeric parts
	prefix1, num1, err1 := sc.parseSerial(serial1)
	prefix2, num2, err2 := sc.parseSerial(serial2)

	// If either parsing fails, fall back to string comparison
	if err1 != nil || err2 != nil {
		return strings.Compare(serial1, serial2)
	}

	// Compare prefixes first
	if prefix1 != prefix2 {
		return strings.Compare(prefix1, prefix2)
	}

	// Compare numeric parts
	if num1 < num2 {
		return -1
	} else if num1 > num2 {
		return 1
	}
	return 0
}

// parseSerial extracts the prefix and numeric portion from a serial number
func (sc *SerialComparator) parseSerial(serial string) (string, int, error) {
	matches := sc.serialPattern.FindStringSubmatch(serial)
	if len(matches) != 3 {
		return "", 0, fmt.Errorf("invalid serial format: %s", serial)
	}

	prefix := matches[1]
	numStr := matches[2]

	num, err := strconv.Atoi(numStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid numeric portion in serial %s: %v", serial, err)
	}

	return prefix, num, nil
}

// ResolveSerialEffectivity filters BOM lines to only those effective for the target serial
func (sc *SerialComparator) ResolveSerialEffectivity(targetSerial string, bomLines []*entities.BOMLine) []*entities.BOMLine {
	var effective []*entities.BOMLine

	for _, line := range bomLines {
		if sc.IsSerialInRange(targetSerial, line.Effectivity) {
			effective = append(effective, line)
		}
	}

	return effective
}

// ValidateSerialEffectivity checks for overlapping effectivity ranges for the same parent/child combination
func (sc *SerialComparator) ValidateSerialEffectivity(bomLines []*entities.BOMLine) error {
	// Group by parent/child combination
	effectivityMap := make(map[string][]entities.SerialEffectivity)

	for _, line := range bomLines {
		key := fmt.Sprintf("%s->%s", line.ParentPN, line.ChildPN)
		effectivityMap[key] = append(effectivityMap[key], line.Effectivity)
	}

	// Check for overlaps within each group
	for key, effectivities := range effectivityMap {
		if err := sc.checkOverlaps(effectivities); err != nil {
			return fmt.Errorf("effectivity overlap for %s: %v", key, err)
		}
	}

	return nil
}

// checkOverlaps verifies that effectivity ranges don't overlap
func (sc *SerialComparator) checkOverlaps(effectivities []entities.SerialEffectivity) error {
	for i := 0; i < len(effectivities); i++ {
		for j := i + 1; j < len(effectivities); j++ {
			if sc.rangesOverlap(effectivities[i], effectivities[j]) {
				return fmt.Errorf("ranges overlap: [%s-%s] and [%s-%s]",
					effectivities[i].FromSerial, effectivities[i].ToSerial,
					effectivities[j].FromSerial, effectivities[j].ToSerial)
			}
		}
	}
	return nil
}

// rangesOverlap checks if two effectivity ranges overlap
func (sc *SerialComparator) rangesOverlap(range1, range2 entities.SerialEffectivity) bool {
	// Convert open-ended ranges to comparable form
	end1 := range1.ToSerial
	end2 := range2.ToSerial

	// Handle open-ended ranges - use a very high value for comparison
	if end1 == "" {
		end1 = "ZZ999999" // Assumes this is higher than any real serial
	}
	if end2 == "" {
		end2 = "ZZ999999"
	}

	// Check for overlap: range1.start <= range2.end && range2.start <= range1.end
	return sc.CompareSerials(range1.FromSerial, end2) <= 0 &&
		sc.CompareSerials(range2.FromSerial, end1) <= 0
}
