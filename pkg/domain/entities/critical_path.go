package entities

import (
	"fmt"
	"time"
)

// CriticalPathNode represents a node in the critical path analysis
type CriticalPathNode struct {
	PartNumber        PartNumber
	Description       string
	LeadTimeDays      int
	CumulativeTime    int
	Level             int
	HasInventory      bool
	InventoryQty      Quantity
	RequiredQty       Quantity
	EffectiveLeadTime int // Lead time after considering inventory
}

// CriticalPath represents a complete path through the BOM with timing information
type CriticalPath struct {
	TotalLeadTime     int
	EffectiveLeadTime int // Total lead time considering inventory
	PathLength        int // Number of levels in the path
	Path              []PartNumber
	PathDetails       []CriticalPathNode
	BottleneckPart    PartNumber // Part with longest lead time in this path
}

// CriticalPathAnalysis contains the results of critical path analysis
type CriticalPathAnalysis struct {
	TopLevelPart PartNumber
	TargetSerial string
	Location     string
	AnalysisDate time.Time
	CriticalPath CriticalPath   // The longest path
	TopPaths     []CriticalPath // Top N longest paths
	TotalPaths   int            // Total number of paths analyzed
}

// GetCriticalPathSummary returns a formatted summary of the critical path
func (analysis *CriticalPathAnalysis) GetCriticalPathSummary() string {
	if len(analysis.TopPaths) == 0 {
		return "No critical path found"
	}

	cp := analysis.CriticalPath
	summary := fmt.Sprintf(
		"Critical Path: %d days (%d effective)",
		cp.TotalLeadTime,
		cp.EffectiveLeadTime,
	)
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
