package bom_validator

import (
	"fmt"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

// ValidationResult contains the results of BOM validation
type ValidationResult struct {
	HasCycles      bool
	CyclePaths     [][]entities.PartNumber
	DuplicateLines []entities.BOMLine
	OrphanedParts  []entities.PartNumber
	Errors         []string
}

// ValidateBOM performs comprehensive validation on a set of BOM lines
func ValidateBOM(bomLines []entities.BOMLine) *ValidationResult {
	result := &ValidationResult{
		CyclePaths:     make([][]entities.PartNumber, 0),
		DuplicateLines: make([]entities.BOMLine, 0),
		OrphanedParts:  make([]entities.PartNumber, 0),
		Errors:         make([]string, 0),
	}

	// Build adjacency map for cycle detection
	adjacencyMap := buildAdjacencyMap(bomLines)

	// Detect cycles
	cycles := detectCycles(adjacencyMap)
	result.HasCycles = len(cycles) > 0
	result.CyclePaths = cycles

	// Detect duplicate BOM lines
	duplicates := detectDuplicateLines(bomLines)
	result.DuplicateLines = duplicates

	// Add validation errors
	if result.HasCycles {
		for _, cycle := range result.CyclePaths {
			result.Errors = append(result.Errors, fmt.Sprintf("BOM cycle detected: %v", cycle))
		}
	}

	if len(result.DuplicateLines) > 0 {
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Found %d duplicate BOM lines", len(result.DuplicateLines)),
		)
	}

	return result
}

// buildAdjacencyMap creates a map of parent -> children relationships
func buildAdjacencyMap(bomLines []entities.BOMLine) map[entities.PartNumber][]entities.PartNumber {
	adjacencyMap := make(map[entities.PartNumber][]entities.PartNumber)

	for _, line := range bomLines {
		children, exists := adjacencyMap[line.ParentPN]
		if !exists {
			children = make([]entities.PartNumber, 0)
		}

		// Avoid duplicate children in adjacency list
		found := false
		for _, child := range children {
			if child == line.ChildPN {
				found = true
				break
			}
		}

		if !found {
			children = append(children, line.ChildPN)
			adjacencyMap[line.ParentPN] = children
		}
	}

	return adjacencyMap
}

// detectCycles uses DFS to find cycles in the BOM structure
func detectCycles(
	adjacencyMap map[entities.PartNumber][]entities.PartNumber,
) [][]entities.PartNumber {
	visited := make(map[entities.PartNumber]bool)
	recursionStack := make(map[entities.PartNumber]bool)
	cycles := make([][]entities.PartNumber, 0)

	// Try to find cycles starting from each part
	for parent := range adjacencyMap {
		if !visited[parent] {
			path := make([]entities.PartNumber, 0)
			dfsDetectCycle(parent, adjacencyMap, visited, recursionStack, path, &cycles)
		}
	}

	return cycles
}

// dfsDetectCycle performs depth-first search to detect cycles
func dfsDetectCycle(
	current entities.PartNumber,
	adjacencyMap map[entities.PartNumber][]entities.PartNumber,
	visited map[entities.PartNumber]bool,
	recursionStack map[entities.PartNumber]bool,
	path []entities.PartNumber,
	cycles *[][]entities.PartNumber,
) {
	visited[current] = true
	recursionStack[current] = true
	path = append(path, current)

	// Check all children
	children, exists := adjacencyMap[current]
	if exists {
		for _, child := range children {
			if !visited[child] {
				dfsDetectCycle(child, adjacencyMap, visited, recursionStack, path, cycles)
			} else if recursionStack[child] {
				// Found a cycle - extract the cycle path
				cycleStart := -1
				for i, part := range path {
					if part == child {
						cycleStart = i
						break
					}
				}

				if cycleStart != -1 {
					cycle := make([]entities.PartNumber, 0)
					cycle = append(cycle, path[cycleStart:]...)
					cycle = append(cycle, child) // Close the cycle
					*cycles = append(*cycles, cycle)
				}
			}
		}
	}

	recursionStack[current] = false
}

// detectDuplicateLines finds duplicate BOM lines (same parent, child, find number)
func detectDuplicateLines(bomLines []entities.BOMLine) []entities.BOMLine {
	seen := make(map[string]entities.BOMLine)
	duplicates := make([]entities.BOMLine, 0)

	for _, line := range bomLines {
		// Create a unique key for parent, child, and find number
		key := fmt.Sprintf("%s|%s|%d", line.ParentPN, line.ChildPN, line.FindNumber)

		if existingLine, exists := seen[key]; exists {
			// This is a duplicate
			duplicates = append(duplicates, line)
			duplicates = append(duplicates, existingLine)
		} else {
			seen[key] = line
		}
	}

	return duplicates
}

// ValidatePartNumberUniqueness validates that part numbers are unique across items
func ValidatePartNumberUniqueness(items []entities.Item) *ValidationResult {
	result := &ValidationResult{
		Errors: make([]string, 0),
	}

	seen := make(map[entities.PartNumber]bool)
	duplicates := make([]entities.PartNumber, 0)

	for _, item := range items {
		if seen[item.PartNumber] {
			duplicates = append(duplicates, item.PartNumber)
		} else {
			seen[item.PartNumber] = true
		}
	}

	if len(duplicates) > 0 {
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Duplicate part numbers found: %v", duplicates),
		)
	}

	return result
}

// ValidateBOMItemConsistency validates that all part numbers in BOM lines exist as items
func ValidateBOMItemConsistency(bomLines []entities.BOMLine, items []entities.Item) *ValidationResult {
	result := &ValidationResult{
		OrphanedParts: make([]entities.PartNumber, 0),
		Errors:        make([]string, 0),
	}

	// Create set of valid part numbers from items
	validParts := make(map[entities.PartNumber]bool)
	for _, item := range items {
		validParts[item.PartNumber] = true
	}

	// Check all part numbers referenced in BOM lines
	referencedParts := make(map[entities.PartNumber]bool)

	for _, line := range bomLines {
		// Track both parent and child part numbers
		referencedParts[line.ParentPN] = true
		referencedParts[line.ChildPN] = true
	}

	// Find orphaned parts (referenced in BOM but not defined as items)
	for partNumber := range referencedParts {
		if !validParts[partNumber] {
			result.OrphanedParts = append(result.OrphanedParts, partNumber)
		}
	}

	if len(result.OrphanedParts) > 0 {
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("BOM references undefined part numbers: %v", result.OrphanedParts),
		)
	}

	return result
}
