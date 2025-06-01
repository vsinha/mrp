package mrp

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// CriticalPathNode represents a node in the critical path analysis
type CriticalPathNode struct {
	PartNumber      PartNumber
	Description     string
	LeadTimeDays    int
	CumulativeTime  int
	Level           int
	HasInventory    bool
	InventoryQty    Quantity
	RequiredQty     Quantity
	EffectiveLeadTime int // Lead time after considering inventory
}

// CriticalPath represents a complete path through the BOM with timing information
type CriticalPath struct {
	TotalLeadTime   int
	EffectiveLeadTime int // Total lead time considering inventory
	PathLength      int  // Number of levels in the path
	Path            []PartNumber
	PathDetails     []CriticalPathNode
	BottleneckPart  PartNumber // Part with longest lead time in this path
}

// CriticalPathAnalysis contains the results of critical path analysis
type CriticalPathAnalysis struct {
	TopLevelPart    PartNumber
	TargetSerial    string
	Location        string
	AnalysisDate    time.Time
	CriticalPath    CriticalPath    // The longest path
	TopPaths        []CriticalPath  // Top N longest paths
	TotalPaths      int            // Total number of paths analyzed
}

// CriticalPathAnalyzer performs critical path analysis on BOM structures
type CriticalPathAnalyzer struct {
	engine *Engine
}

// NewCriticalPathAnalyzer creates a new critical path analyzer
func NewCriticalPathAnalyzer(engine *Engine) *CriticalPathAnalyzer {
	return &CriticalPathAnalyzer{
		engine: engine,
	}
}

// AnalyzeCriticalPath performs critical path analysis for a given part and returns top N paths
func (cpa *CriticalPathAnalyzer) AnalyzeCriticalPath(ctx context.Context, partNumber PartNumber, targetSerial string, location string, topN int) (*CriticalPathAnalysis, error) {
	// Get all paths through the BOM
	allPaths, err := cpa.findAllPaths(ctx, partNumber, targetSerial, location, 1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to find paths for %s: %w", partNumber, err)
	}

	if len(allPaths) == 0 {
		return &CriticalPathAnalysis{
			TopLevelPart: partNumber,
			TargetSerial: targetSerial,
			Location:     location,
			AnalysisDate: time.Now(),
			TotalPaths:   0,
		}, nil
	}

	// Sort paths by effective lead time (descending)
	sort.Slice(allPaths, func(i, j int) bool {
		// Primary sort: effective lead time
		if allPaths[i].EffectiveLeadTime != allPaths[j].EffectiveLeadTime {
			return allPaths[i].EffectiveLeadTime > allPaths[j].EffectiveLeadTime
		}
		// Secondary sort: total lead time (ignoring inventory)
		if allPaths[i].TotalLeadTime != allPaths[j].TotalLeadTime {
			return allPaths[i].TotalLeadTime > allPaths[j].TotalLeadTime
		}
		// Tertiary sort: path length (longer paths first)
		return allPaths[i].PathLength > allPaths[j].PathLength
	})

	// Get top N paths
	topPaths := allPaths
	if len(allPaths) > topN {
		topPaths = allPaths[:topN]
	}

	analysis := &CriticalPathAnalysis{
		TopLevelPart: partNumber,
		TargetSerial: targetSerial,
		Location:     location,
		AnalysisDate: time.Now(),
		CriticalPath: allPaths[0], // Longest path
		TopPaths:     topPaths,
		TotalPaths:   len(allPaths),
	}

	return analysis, nil
}

// findAllPaths recursively finds all paths through the BOM structure
func (cpa *CriticalPathAnalyzer) findAllPaths(ctx context.Context, partNumber PartNumber, targetSerial string, location string, quantity Quantity, level int) ([]CriticalPath, error) {
	// Get item master data
	item, err := cpa.engine.bomRepo.GetItem(ctx, partNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get item %s: %w", partNumber, err)
	}

	// Check inventory availability
	hasInventory, inventoryQty, effectiveLeadTime := cpa.checkInventoryAvailability(ctx, partNumber, location, quantity, item.LeadTimeDays)

	// Create node for this part
	node := CriticalPathNode{
		PartNumber:        partNumber,
		Description:       item.Description,
		LeadTimeDays:      item.LeadTimeDays,
		Level:             level,
		HasInventory:      hasInventory,
		InventoryQty:      inventoryQty,
		RequiredQty:       quantity,
		EffectiveLeadTime: effectiveLeadTime,
	}

	// Get effective BOM for this part
	bomLines, err := cpa.engine.bomRepo.GetEffectiveBOM(ctx, partNumber, targetSerial)
	if err != nil {
		return nil, fmt.Errorf("failed to get BOM for %s: %w", partNumber, err)
	}

	// Filter BOM lines by serial effectivity
	effectiveLines := cpa.engine.serialComp.ResolveSerialEffectivity(targetSerial, bomLines)

	// If no children (leaf node), return single path
	if len(effectiveLines) == 0 {
		path := CriticalPath{
			TotalLeadTime:     item.LeadTimeDays,
			EffectiveLeadTime: effectiveLeadTime,
			PathLength:        1,
			Path:              []PartNumber{partNumber},
			PathDetails:       []CriticalPathNode{node},
			BottleneckPart:    partNumber,
		}
		return []CriticalPath{path}, nil
	}

	// Recursively get paths for all children
	var allChildPaths []CriticalPath
	
	for _, line := range effectiveLines {
		childQty := line.QtyPer * quantity
		childPaths, err := cpa.findAllPaths(ctx, line.ChildPN, targetSerial, location, childQty, level+1)
		if err != nil {
			return nil, fmt.Errorf("failed to get paths for child %s: %w", line.ChildPN, err)
		}
		allChildPaths = append(allChildPaths, childPaths...)
	}

	// Build paths from this node through each child path
	var resultPaths []CriticalPath
	
	for _, childPath := range allChildPaths {
		// Calculate cumulative times
		node.CumulativeTime = effectiveLeadTime + childPath.EffectiveLeadTime
		
		// Determine bottleneck (part with longest individual lead time in path)
		bottleneck := node.PartNumber
		if item.LeadTimeDays < getLeadTimeForPart(childPath.BottleneckPart, childPath.PathDetails) {
			bottleneck = childPath.BottleneckPart
		}

		// Create new path by prepending this node
		newPath := CriticalPath{
			TotalLeadTime:     item.LeadTimeDays + childPath.TotalLeadTime,
			EffectiveLeadTime: effectiveLeadTime + childPath.EffectiveLeadTime,
			PathLength:        1 + childPath.PathLength,
			Path:              append([]PartNumber{partNumber}, childPath.Path...),
			PathDetails:       append([]CriticalPathNode{node}, childPath.PathDetails...),
			BottleneckPart:    bottleneck,
		}
		
		resultPaths = append(resultPaths, newPath)
	}

	return resultPaths, nil
}

// checkInventoryAvailability checks if inventory is available and calculates effective lead time
func (cpa *CriticalPathAnalyzer) checkInventoryAvailability(ctx context.Context, partNumber PartNumber, location string, requiredQty Quantity, baseLeadTime int) (bool, Quantity, int) {
	// Get available inventory
	lotInventory, serialInventory, err := cpa.engine.inventoryRepo.GetAvailableInventory(ctx, partNumber, location)
	if err != nil {
		// If we can't get inventory info, assume no inventory
		return false, 0, baseLeadTime
	}

	// Calculate total available quantity
	var totalAvailable Quantity
	
	// Add serialized inventory (each serial = 1 unit)
	for _, inv := range serialInventory {
		if inv.Status == Available {
			totalAvailable += 1
		}
	}
	
	// Add lot inventory
	for _, lot := range lotInventory {
		if lot.Status == Available {
			totalAvailable += lot.Quantity
		}
	}

	hasInventory := totalAvailable > 0
	availableQty := totalAvailable
	
	// Calculate effective lead time based on inventory coverage
	if totalAvailable >= requiredQty {
		// Full inventory coverage - zero lead time
		return hasInventory, availableQty, 0
	} else if totalAvailable > 0 {
		// Partial inventory coverage - reduced lead time
		// Simple model: reduce lead time proportionally
		coverageRatio := float64(totalAvailable) / float64(requiredQty)
		effectiveLeadTime := float64(baseLeadTime) * (1.0 - coverageRatio)
		return hasInventory, availableQty, int(effectiveLeadTime)
	} else {
		// No inventory - full lead time
		return false, 0, baseLeadTime
	}
}

// getLeadTimeForPart finds the lead time for a specific part in the path details
func getLeadTimeForPart(partNumber PartNumber, pathDetails []CriticalPathNode) int {
	for _, node := range pathDetails {
		if node.PartNumber == partNumber {
			return node.LeadTimeDays
		}
	}
	return 0
}

// Helper methods for formatting and analysis

// GetCriticalPathSummary returns a formatted summary of the critical path
func (analysis *CriticalPathAnalysis) GetCriticalPathSummary() string {
	if len(analysis.TopPaths) == 0 {
		return "No critical path found"
	}

	cp := analysis.CriticalPath
	summary := fmt.Sprintf("Critical Path: %d days (%d effective)", cp.TotalLeadTime, cp.EffectiveLeadTime)
	if cp.BottleneckPart != "" {
		summary += fmt.Sprintf(" | Bottleneck: %s", cp.BottleneckPart)
	}
	return summary
}

// GetPathSummary returns a formatted summary for a specific path
func (path *CriticalPath) GetPathSummary() string {
	return fmt.Sprintf("%d days (%d effective) - %d levels - %s", 
		path.TotalLeadTime, path.EffectiveLeadTime, path.PathLength, path.BottleneckPart)
}

// GetInventoryCoverage returns the percentage of paths that have some inventory coverage
func (analysis *CriticalPathAnalysis) GetInventoryCoverage() float64 {
	if len(analysis.TopPaths) == 0 {
		return 0.0
	}

	pathsWithInventory := 0
	for _, path := range analysis.TopPaths {
		hasInventory := false
		for _, node := range path.PathDetails {
			if node.HasInventory {
				hasInventory = true
				break
			}
		}
		if hasInventory {
			pathsWithInventory++
		}
	}

	return float64(pathsWithInventory) / float64(len(analysis.TopPaths)) * 100.0
}