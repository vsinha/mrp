package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/vsinha/mrp/pkg/application/dto"
	"github.com/vsinha/mrp/pkg/application/services/criticalpath"
	"github.com/vsinha/mrp/pkg/application/services/mrp"
	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
)

// PlanningOrchestrator coordinates between MRP and Critical Path services
type PlanningOrchestrator struct {
	mrpService          *mrp.MRPService
	criticalPathService *criticalpath.CriticalPathService
	bomRepo             repositories.BOMRepository
	itemRepo            repositories.ItemRepository
	inventoryRepo       repositories.InventoryRepository
	demandRepo          repositories.DemandRepository
}

// NewPlanningOrchestrator creates a new planning orchestrator
func NewPlanningOrchestrator(
	mrpService *mrp.MRPService,
	criticalPathService *criticalpath.CriticalPathService,
	bomRepo repositories.BOMRepository,
	itemRepo repositories.ItemRepository,
	inventoryRepo repositories.InventoryRepository,
	demandRepo repositories.DemandRepository,
) *PlanningOrchestrator {
	return &PlanningOrchestrator{
		mrpService:          mrpService,
		criticalPathService: criticalPathService,
		bomRepo:             bomRepo,
		itemRepo:            itemRepo,
		inventoryRepo:       inventoryRepo,
		demandRepo:          demandRepo,
	}
}

// PlanningResult contains the combined results of MRP and Critical Path analysis
type PlanningResult struct {
	MRPResult         *dto.MRPResult
	CriticalPath      *entities.CriticalPathAnalysis
	PlanningDate      time.Time
	TotalParts        int
	TotalLeadTime     int
	EffectiveLeadTime int
}

// RunCompletePlanning performs MRP explosion followed by allocation-aware critical path analysis
func (po *PlanningOrchestrator) RunCompletePlanning(
	ctx context.Context,
	demands []*entities.DemandRequirement,
	topPaths int,
) (*PlanningResult, error) {
	if len(demands) == 0 {
		return nil, fmt.Errorf("no demands provided for planning")
	}

	// Step 1: Run MRP explosion to get allocation results
	mrpResult, err := po.mrpService.ExplodeDemand(
		ctx,
		demands,
		po.bomRepo,
		po.itemRepo,
		po.inventoryRepo,
		po.demandRepo,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to run MRP explosion: %w", err)
	}

	// Step 2: Run critical path analysis using MRP allocation results
	// Use the first demand as representative for critical path analysis
	primaryDemand := demands[0]
	criticalPath, err := po.criticalPathService.AnalyzeCriticalPathWithAllocations(
		ctx,
		primaryDemand.PartNumber,
		primaryDemand.TargetSerial,
		primaryDemand.Location,
		topPaths,
		mrpResult.Allocations,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze critical path: %w", err)
	}

	// Step 3: Create combined result
	result := &PlanningResult{
		MRPResult:         mrpResult,
		CriticalPath:      criticalPath,
		PlanningDate:      time.Now(),
		TotalParts:        len(mrpResult.PlannedOrders),
		TotalLeadTime:     criticalPath.CriticalPath.TotalLeadTime,
		EffectiveLeadTime: criticalPath.CriticalPath.EffectiveLeadTime,
	}

	return result, nil
}

// AnalyzeCriticalPathForDemand performs critical path analysis for a specific demand using MRP allocation results
func (po *PlanningOrchestrator) AnalyzeCriticalPathForDemand(
	ctx context.Context,
	demand *entities.DemandRequirement,
	topPaths int,
) (*entities.CriticalPathAnalysis, error) {
	// First run MRP to get allocation results
	mrpResult, err := po.mrpService.ExplodeDemand(
		ctx,
		[]*entities.DemandRequirement{demand},
		po.bomRepo,
		po.itemRepo,
		po.inventoryRepo,
		po.demandRepo,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to run MRP for critical path analysis: %w", err)
	}

	// Use allocation results for critical path analysis
	return po.criticalPathService.AnalyzeCriticalPathWithAllocations(
		ctx,
		demand.PartNumber,
		demand.TargetSerial,
		demand.Location,
		topPaths,
		mrpResult.Allocations,
	)
}

// AnalyzeCriticalPathForPart performs critical path analysis for a specific part using MRP allocation results
func (po *PlanningOrchestrator) AnalyzeCriticalPathForPart(
	ctx context.Context,
	partNumber entities.PartNumber,
	targetSerial string,
	location string,
	topPaths int,
) (*entities.CriticalPathAnalysis, error) {
	// Create a demand for this part to get allocation results
	demand := &entities.DemandRequirement{
		PartNumber:   partNumber,
		Quantity:     1,                                   // Use unit quantity for analysis
		NeedDate:     time.Now().Add(30 * 24 * time.Hour), // 30 days from now
		DemandSource: "CRITICAL_PATH_ANALYSIS",
		Location:     location,
		TargetSerial: targetSerial,
	}

	return po.AnalyzeCriticalPathForDemand(ctx, demand, topPaths)
}

// AnalyzeCriticalPathWithMRPResults performs critical path analysis using existing MRP results
func (po *PlanningOrchestrator) AnalyzeCriticalPathWithMRPResults(
	ctx context.Context,
	partNumber entities.PartNumber,
	targetSerial string,
	location string,
	topPaths int,
	mrpResult *dto.MRPResult,
) (*entities.CriticalPathAnalysis, error) {
	return po.criticalPathService.AnalyzeCriticalPathWithAllocations(
		ctx,
		partNumber,
		targetSerial,
		location,
		topPaths,
		mrpResult.Allocations,
	)
}

// GetSummary returns a formatted summary of the planning results
func (result *PlanningResult) GetSummary() string {
	summary := fmt.Sprintf("Planning Summary (analyzed %d parts):\n", result.TotalParts)
	summary += fmt.Sprintf("  MRP: %d planned orders, %d allocations, %d shortages\n",
		len(result.MRPResult.PlannedOrders),
		len(result.MRPResult.Allocations),
		len(result.MRPResult.ShortageReport))
	summary += fmt.Sprintf("  Critical Path: %s\n", result.CriticalPath.GetCriticalPathSummary())
	summary += fmt.Sprintf(
		"  Inventory Coverage: %.1f%%",
		result.CriticalPath.GetInventoryCoverage(),
	)
	return summary
}
