package criticalpath

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/vsinha/mrp/pkg/application/services/shared"
	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
	"github.com/vsinha/mrp/pkg/domain/services"
)

// CriticalPathService performs critical path analysis on BOM structures
type CriticalPathService struct {
	bomRepo       repositories.BOMRepository
	itemRepo      repositories.ItemRepository
	inventoryRepo repositories.InventoryRepository
	serialComp    *services.SerialComparator
	bomTraverser  *shared.BOMTraverser
}

// NewCriticalPathService creates a new critical path service
func NewCriticalPathService(
	bomRepo repositories.BOMRepository,
	itemRepo repositories.ItemRepository,
	inventoryRepo repositories.InventoryRepository,
	serialComp *services.SerialComparator,
) *CriticalPathService {
	bomTraverser := shared.NewBOMTraverser(bomRepo, itemRepo, inventoryRepo)
	return &CriticalPathService{
		bomRepo:       bomRepo,
		itemRepo:      itemRepo,
		inventoryRepo: inventoryRepo,
		serialComp:    serialComp,
		bomTraverser:  bomTraverser,
	}
}

// AnalyzeCriticalPath performs critical path analysis for a given part and returns top N paths
func (cps *CriticalPathService) AnalyzeCriticalPath(
	ctx context.Context,
	partNumber entities.PartNumber,
	targetSerial string,
	location string,
	topN int,
) (*entities.CriticalPathAnalysis, error) {
	// Get all paths through the BOM
	allPaths, err := cps.findAllPaths(ctx, partNumber, targetSerial, location, 1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to find paths for %s: %w", partNumber, err)
	}

	if len(allPaths) == 0 {
		return &entities.CriticalPathAnalysis{
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

	analysis := &entities.CriticalPathAnalysis{
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

// AnalyzeCriticalPathWithAllocations performs critical path analysis using MRP allocation results
func (cps *CriticalPathService) AnalyzeCriticalPathWithAllocations(
	ctx context.Context,
	partNumber entities.PartNumber,
	targetSerial string,
	location string,
	topN int,
	allocations []entities.AllocationResult,
) (*entities.CriticalPathAnalysis, error) {
	// Set allocation context in the BOM traverser
	cps.bomTraverser.SetAllocationContext(allocations)
	defer cps.bomTraverser.ClearAllocationContext() // Clean up after analysis

	// Get all paths through the BOM using allocation context
	allPaths, err := cps.findAllPaths(ctx, partNumber, targetSerial, location, 1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to find paths for %s: %w", partNumber, err)
	}

	if len(allPaths) == 0 {
		return &entities.CriticalPathAnalysis{
			TopLevelPart: partNumber,
			TargetSerial: targetSerial,
			Location:     location,
			AnalysisDate: time.Now(),
			TotalPaths:   0,
		}, nil
	}

	// Sort paths by total lead time (descending - longest first)
	sort.Slice(allPaths, func(i, j int) bool {
		return allPaths[i].TotalLeadTime > allPaths[j].TotalLeadTime
	})

	// Get top N paths
	if topN > len(allPaths) {
		topN = len(allPaths)
	}
	topPaths := allPaths[:topN]

	analysis := &entities.CriticalPathAnalysis{
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

// findAllPaths recursively finds all paths through the BOM structure using BOMTraverser
func (cps *CriticalPathService) findAllPaths(
	ctx context.Context,
	partNumber entities.PartNumber,
	targetSerial string,
	location string,
	quantity entities.Quantity,
	level int,
) ([]entities.CriticalPath, error) {
	// Use BOMTraverser with CriticalPathVisitor to perform the traversal
	visitor := NewCriticalPathVisitor(cps.inventoryRepo, cps.serialComp)
	result, err := cps.bomTraverser.TraverseBOM(
		ctx,
		partNumber,
		targetSerial,
		location,
		quantity,
		level,
		visitor,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to traverse BOM for %s: %w", partNumber, err)
	}

	paths := result.([]entities.CriticalPath)
	return paths, nil
}
