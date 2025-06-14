package incremental

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vsinha/mrp/pkg/application/services/shared"
	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
	"github.com/vsinha/mrp/pkg/infrastructure/events"
)

type RequirementProcessor struct {
	bomRepo       repositories.BOMRepository
	itemRepo      repositories.ItemRepository
	inventoryRepo repositories.InventoryRepository
	eventStore    events.EventStore

	// Cache for incremental BOM explosions
	explosionCache map[string]*ExplosionCacheEntry
	cacheMutex     sync.RWMutex
}

type ExplosionCacheEntry struct {
	Requirements []entities.GrossRequirement
	ComputedAt   time.Time
	TargetSerial string
}

func NewRequirementProcessor(
	bomRepo repositories.BOMRepository,
	itemRepo repositories.ItemRepository,
	inventoryRepo repositories.InventoryRepository,
	eventStore events.EventStore,
) *RequirementProcessor {
	return &RequirementProcessor{
		bomRepo:        bomRepo,
		itemRepo:       itemRepo,
		inventoryRepo:  inventoryRepo,
		eventStore:     eventStore,
		explosionCache: make(map[string]*ExplosionCacheEntry),
	}
}

func (p *RequirementProcessor) Handle(event events.Event) error {
	switch event.Type() {
	case events.DemandCreatedEvent:
		return p.handleDemandCreated(event)
	case events.DemandUpdatedEvent:
		return p.handleDemandUpdated(event)
	case events.BOMLineCreatedEvent, events.BOMLineUpdatedEvent:
		return p.handleBOMChanged(event)
	default:
		return nil // Ignore events we don't handle
	}
}

func (p *RequirementProcessor) CanHandle(eventType string) bool {
	switch eventType {
	case events.DemandCreatedEvent,
		events.DemandUpdatedEvent,
		events.BOMLineCreatedEvent,
		events.BOMLineUpdatedEvent:
		return true
	default:
		return false
	}
}

func (p *RequirementProcessor) handleDemandCreated(event events.Event) error {
	demandData, ok := event.Data().(events.DemandCreated)
	if !ok {
		return fmt.Errorf("invalid event data for demand created")
	}

	ctx := context.Background()
	demand := demandData.Demand

	// Perform incremental BOM explosion for this specific demand
	requirements, err := p.explodeRequirementIncremental(
		ctx,
		demand.PartNumber,
		demand.TargetSerial,
		demand.NeedDate,
		demand.DemandSource,
		demand.Location,
		demand.Quantity,
	)
	if err != nil {
		return fmt.Errorf("failed to explode requirement for %s: %w", demand.PartNumber, err)
	}

	// Publish requirement calculated events for each exploded requirement
	for _, req := range requirements {
		reqEvent := events.NewRequirementCalculatedEvent(*req, demand, []entities.BOMLine{})
		if err := p.eventStore.AppendEvent(string(req.PartNumber), reqEvent); err != nil {
			fmt.Printf("Warning: failed to publish requirement calculated event: %v\n", err)
		}
	}

	return nil
}

func (p *RequirementProcessor) handleDemandUpdated(event events.Event) error {
	updateData, ok := event.Data().(events.DemandUpdated)
	if !ok {
		return fmt.Errorf("invalid event data for demand updated")
	}

	// For now, treat as delete old + create new
	// TODO: Implement more sophisticated delta processing
	oldDemand := updateData.OldDemand
	newDemand := updateData.NewDemand

	// Invalidate cache entries for the old demand
	p.invalidateCacheForPart(oldDemand.PartNumber, oldDemand.TargetSerial)

	// Process the new demand
	return p.handleDemandCreated(
		events.NewEvent(
			events.DemandCreatedEvent,
			string(newDemand.PartNumber),
			events.DemandCreated{Demand: newDemand},
		),
	)
}

func (p *RequirementProcessor) handleBOMChanged(event events.Event) error {
	var affectedPartNumber entities.PartNumber

	switch event.Type() {
	case events.BOMLineCreatedEvent:
		bomData, ok := event.Data().(events.BOMLineCreated)
		if !ok {
			return fmt.Errorf("invalid event data for BOM line created")
		}
		affectedPartNumber = bomData.BOMLine.ParentPN

	case events.BOMLineUpdatedEvent:
		bomData, ok := event.Data().(events.BOMLineUpdated)
		if !ok {
			return fmt.Errorf("invalid event data for BOM line updated")
		}
		affectedPartNumber = bomData.NewBOMLine.ParentPN
	}

	// Invalidate all cache entries for the affected part
	p.invalidateAllCacheForPart(affectedPartNumber)

	// TODO: Trigger re-processing of affected demands
	// This would require tracking which demands depend on which parts

	return nil
}

func (p *RequirementProcessor) explodeRequirementIncremental(
	ctx context.Context,
	partNumber entities.PartNumber,
	targetSerial string,
	needDate time.Time,
	demandTrace string,
	location string,
	quantity entities.Quantity,
) ([]*entities.GrossRequirement, error) {

	// Check cache first
	cacheKey := fmt.Sprintf("%s|%s", partNumber, targetSerial)
	p.cacheMutex.RLock()
	cached, exists := p.explosionCache[cacheKey]
	p.cacheMutex.RUnlock()

	if exists && time.Since(cached.ComputedAt) < 5*time.Minute {
		// Scale cached requirements by current quantity
		var scaledRequirements []*entities.GrossRequirement
		for _, req := range cached.Requirements {
			scaledReq := &entities.GrossRequirement{
				PartNumber:   req.PartNumber,
				Quantity:     req.Quantity * quantity,
				NeedDate:     needDate,
				DemandTrace:  demandTrace + " -> " + req.DemandTrace,
				Location:     location,
				TargetSerial: req.TargetSerial,
			}
			scaledRequirements = append(scaledRequirements, scaledReq)
		}
		return scaledRequirements, nil
	}

	// Use existing BOM traverser for explosion
	bomTraverser := shared.NewBOMTraverser(p.bomRepo, p.itemRepo, p.inventoryRepo)
	visitor := NewIncrementalMRPVisitor(demandTrace, needDate)

	result, err := bomTraverser.TraverseBOM(
		ctx,
		partNumber,
		targetSerial,
		location,
		quantity,
		0,
		visitor,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to traverse BOM for %s: %w", partNumber, err)
	}

	requirements := result.([]*entities.GrossRequirement)

	// Cache the base requirements (normalized to unit quantity)
	baseRequirements := make([]entities.GrossRequirement, len(requirements))
	for i, req := range requirements {
		baseRequirements[i] = entities.GrossRequirement{
			PartNumber:   req.PartNumber,
			Quantity:     req.Quantity / quantity, // Normalize to unit quantity
			NeedDate:     req.NeedDate,
			DemandTrace:  string(req.PartNumber),
			Location:     req.Location,
			TargetSerial: req.TargetSerial,
		}
	}

	// Store in cache
	p.cacheMutex.Lock()
	p.explosionCache[cacheKey] = &ExplosionCacheEntry{
		Requirements: baseRequirements,
		ComputedAt:   time.Now(),
		TargetSerial: targetSerial,
	}
	p.cacheMutex.Unlock()

	return requirements, nil
}

func (p *RequirementProcessor) invalidateCacheForPart(
	partNumber entities.PartNumber,
	targetSerial string,
) {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	cacheKey := fmt.Sprintf("%s|%s", partNumber, targetSerial)
	delete(p.explosionCache, cacheKey)
}

func (p *RequirementProcessor) invalidateAllCacheForPart(partNumber entities.PartNumber) {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	// Remove all cache entries that contain this part number
	for key, entry := range p.explosionCache {
		// Check if this part appears in any of the cached requirements
		for _, req := range entry.Requirements {
			if req.PartNumber == partNumber {
				delete(p.explosionCache, key)
				break
			}
		}
	}
}
