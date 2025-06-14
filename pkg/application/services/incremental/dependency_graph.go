package incremental

import (
	"fmt"
	"sync"
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/repositories"
	"github.com/vsinha/mrp/pkg/infrastructure/events"
)

type IncrementalDependencyNode struct {
	PartNumber     entities.PartNumber
	Item           *entities.Item
	Level          int
	DirectChildren map[entities.PartNumber]bool          // Set for fast lookup
	DirectParents  map[entities.PartNumber]bool          // Set for fast lookup
	Requirements   map[string]*entities.GrossRequirement // Key: demandTrace
	LastUpdated    time.Time
}

type IncrementalDependencyGraph struct {
	nodes      map[entities.PartNumber]*IncrementalDependencyNode
	mutex      sync.RWMutex
	eventStore events.EventStore
	bomRepo    repositories.BOMRepository
	itemRepo   repositories.ItemRepository
}

func NewIncrementalDependencyGraph(
	eventStore events.EventStore,
	bomRepo repositories.BOMRepository,
	itemRepo repositories.ItemRepository,
) *IncrementalDependencyGraph {
	return &IncrementalDependencyGraph{
		nodes:      make(map[entities.PartNumber]*IncrementalDependencyNode),
		eventStore: eventStore,
		bomRepo:    bomRepo,
		itemRepo:   itemRepo,
	}
}

func (dg *IncrementalDependencyGraph) Handle(event events.Event) error {
	switch event.Type() {
	case events.RequirementCalculatedEvent:
		return dg.handleRequirementCalculated(event)
	case events.BOMLineCreatedEvent, events.BOMLineUpdatedEvent, events.BOMLineDeletedEvent:
		return dg.handleBOMChanged(event)
	default:
		return nil
	}
}

func (dg *IncrementalDependencyGraph) CanHandle(eventType string) bool {
	switch eventType {
	case events.RequirementCalculatedEvent,
		events.BOMLineCreatedEvent,
		events.BOMLineUpdatedEvent,
		events.BOMLineDeletedEvent:
		return true
	default:
		return false
	}
}

func (dg *IncrementalDependencyGraph) handleRequirementCalculated(event events.Event) error {
	reqData, ok := event.Data().(events.RequirementCalculated)
	if !ok {
		return fmt.Errorf("invalid event data for requirement calculated")
	}

	requirement := reqData.GrossRequirement
	sourceDemand := reqData.SourceDemand
	bomPath := reqData.BOMPath

	dg.mutex.Lock()
	defer dg.mutex.Unlock()

	// Ensure node exists for this part
	if err := dg.ensureNodeExists(requirement.PartNumber); err != nil {
		return fmt.Errorf("failed to ensure node exists for %s: %w", requirement.PartNumber, err)
	}

	node := dg.nodes[requirement.PartNumber]

	// Add/update requirement
	requirementKey := fmt.Sprintf("%s|%s", sourceDemand.DemandSource, requirement.DemandTrace)
	node.Requirements[requirementKey] = &requirement
	node.LastUpdated = time.Now()

	// Update dependencies from BOM path
	if err := dg.updateDependenciesFromBOMPath(bomPath); err != nil {
		return fmt.Errorf("failed to update dependencies: %w", err)
	}

	// Also build dependencies by querying BOM repository for this part
	if err := dg.buildDependenciesForPart(requirement.PartNumber); err != nil {
		return fmt.Errorf("failed to build dependencies for %s: %w", requirement.PartNumber, err)
	}

	// Recalculate levels for affected parts
	dg.recalculateLevelsIncremental(requirement.PartNumber)

	return nil
}

func (dg *IncrementalDependencyGraph) handleBOMChanged(event events.Event) error {
	var affectedPartNumber entities.PartNumber
	var bomLine entities.BOMLine

	switch event.Type() {
	case events.BOMLineCreatedEvent:
		bomData, ok := event.Data().(events.BOMLineCreated)
		if !ok {
			return fmt.Errorf("invalid event data for BOM line created")
		}
		affectedPartNumber = bomData.BOMLine.ParentPN
		bomLine = bomData.BOMLine

	case events.BOMLineUpdatedEvent:
		bomData, ok := event.Data().(events.BOMLineUpdated)
		if !ok {
			return fmt.Errorf("invalid event data for BOM line updated")
		}
		affectedPartNumber = bomData.NewBOMLine.ParentPN
		bomLine = bomData.NewBOMLine

	case events.BOMLineDeletedEvent:
		bomData, ok := event.Data().(events.BOMLineDeleted)
		if !ok {
			return fmt.Errorf("invalid event data for BOM line deleted")
		}
		affectedPartNumber = bomData.BOMLine.ParentPN
		bomLine = bomData.BOMLine
	}

	dg.mutex.Lock()
	defer dg.mutex.Unlock()

	// Update the dependency relationship
	if err := dg.updateSingleDependency(bomLine, event.Type() == events.BOMLineDeletedEvent); err != nil {
		return fmt.Errorf("failed to update dependency: %w", err)
	}

	// Recalculate levels for affected parts and their dependencies
	dg.recalculateLevelsIncremental(affectedPartNumber)
	dg.recalculateLevelsIncremental(bomLine.ChildPN)

	return nil
}

func (dg *IncrementalDependencyGraph) ensureNodeExists(partNumber entities.PartNumber) error {
	if _, exists := dg.nodes[partNumber]; !exists {
		item, err := dg.itemRepo.GetItem(partNumber)
		if err != nil {
			return fmt.Errorf("failed to get item %s: %w", partNumber, err)
		}

		dg.nodes[partNumber] = &IncrementalDependencyNode{
			PartNumber:     partNumber,
			Item:           item,
			Level:          0,
			DirectChildren: make(map[entities.PartNumber]bool),
			DirectParents:  make(map[entities.PartNumber]bool),
			Requirements:   make(map[string]*entities.GrossRequirement),
			LastUpdated:    time.Now(),
		}
	}
	return nil
}

func (dg *IncrementalDependencyGraph) updateDependenciesFromBOMPath(
	bomPath []entities.BOMLine,
) error {
	for _, bomLine := range bomPath {
		if err := dg.updateSingleDependency(bomLine, false); err != nil {
			return err
		}
	}
	return nil
}

func (dg *IncrementalDependencyGraph) updateSingleDependency(
	bomLine entities.BOMLine,
	isDelete bool,
) error {
	// Ensure both parent and child nodes exist
	if err := dg.ensureNodeExists(bomLine.ParentPN); err != nil {
		return err
	}
	if err := dg.ensureNodeExists(bomLine.ChildPN); err != nil {
		return err
	}

	parentNode := dg.nodes[bomLine.ParentPN]
	childNode := dg.nodes[bomLine.ChildPN]

	if isDelete {
		// Remove dependency relationship
		delete(parentNode.DirectChildren, bomLine.ChildPN)
		delete(childNode.DirectParents, bomLine.ParentPN)
	} else {
		// Add dependency relationship
		parentNode.DirectChildren[bomLine.ChildPN] = true
		childNode.DirectParents[bomLine.ParentPN] = true
	}

	parentNode.LastUpdated = time.Now()
	childNode.LastUpdated = time.Now()

	return nil
}

func (dg *IncrementalDependencyGraph) buildDependenciesForPart(partNumber entities.PartNumber) error {
	// Get all BOM lines where this part is the parent
	alternateGroups, err := dg.bomRepo.GetAlternateGroups(partNumber)
	if err != nil {
		return fmt.Errorf("failed to get alternate groups for %s: %w", partNumber, err)
	}

	// Ensure node exists for this part
	if err := dg.ensureNodeExists(partNumber); err != nil {
		return err
	}

	// For each find number, get the effective alternates
	for findNumber := range alternateGroups {
		// Use a default target serial for dependency building
		// In a more sophisticated implementation, we'd track per-serial dependencies
		effectiveAlternates, err := dg.bomRepo.GetEffectiveAlternates(partNumber, findNumber, "SN001")
		if err != nil {
			return fmt.Errorf("failed to get effective alternates for %s find %d: %w", partNumber, findNumber, err)
		}

		if len(effectiveAlternates) > 0 {
			// Select the best alternate (highest priority)
			var selectedAlternate *entities.BOMLine
			for _, alt := range effectiveAlternates {
				if selectedAlternate == nil || alt.Priority < selectedAlternate.Priority {
					selectedAlternate = alt
				}
			}

			if selectedAlternate != nil {
				// Ensure child node exists
				if err := dg.ensureNodeExists(selectedAlternate.ChildPN); err != nil {
					return err
				}

				// Add parent-child relationship
				dg.nodes[partNumber].DirectChildren[selectedAlternate.ChildPN] = true
				dg.nodes[selectedAlternate.ChildPN].DirectParents[partNumber] = true

				// Update timestamps
				dg.nodes[partNumber].LastUpdated = time.Now()
				dg.nodes[selectedAlternate.ChildPN].LastUpdated = time.Now()
			}
		}
	}

	return nil
}

func (dg *IncrementalDependencyGraph) recalculateLevelsIncremental(
	startingPart entities.PartNumber,
) {
	// Recalculate levels using BFS starting from the affected part
	visited := make(map[entities.PartNumber]bool)
	queue := []entities.PartNumber{startingPart}

	for len(queue) > 0 {
		currentPart := queue[0]
		queue = queue[1:]

		if visited[currentPart] {
			continue
		}
		visited[currentPart] = true

		node, exists := dg.nodes[currentPart]
		if !exists {
			continue
		}

		// Calculate new level based on children
		newLevel := 0
		for childPN := range node.DirectChildren {
			if childNode, exists := dg.nodes[childPN]; exists {
				if childNode.Level+1 > newLevel {
					newLevel = childNode.Level + 1
				}
			}
		}

		// If level changed, update and queue parents for recalculation
		if newLevel != node.Level {
			node.Level = newLevel
			node.LastUpdated = time.Now()

			// Queue all parents for level recalculation
			for parentPN := range node.DirectParents {
				if !visited[parentPN] {
					queue = append(queue, parentPN)
				}
			}
		}
	}
}

func (dg *IncrementalDependencyGraph) GetAffectedParts(
	partNumber entities.PartNumber,
) []entities.PartNumber {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	var affected []entities.PartNumber
	visited := make(map[entities.PartNumber]bool)
	queue := []entities.PartNumber{partNumber}

	for len(queue) > 0 {
		currentPart := queue[0]
		queue = queue[1:]

		if visited[currentPart] {
			continue
		}
		visited[currentPart] = true
		affected = append(affected, currentPart)

		// Add all parents (parts that depend on this part)
		if node, exists := dg.nodes[currentPart]; exists {
			for parentPN := range node.DirectParents {
				if !visited[parentPN] {
					queue = append(queue, parentPN)
				}
			}
		}
	}

	return affected
}

func (dg *IncrementalDependencyGraph) GetTopologicalOrder() []entities.PartNumber {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	// Kahn's algorithm for topological sorting
	inDegree := make(map[entities.PartNumber]int)
	queue := make([]entities.PartNumber, 0)
	result := make([]entities.PartNumber, 0, len(dg.nodes))

	// Calculate in-degree for each node
	for partNumber, node := range dg.nodes {
		inDegree[partNumber] = len(node.DirectChildren)
	}

	// Add all nodes with no dependencies (leaf parts) to queue
	for partNumber, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, partNumber)
		}
	}

	// Process queue
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// For each parent of current node, reduce its in-degree
		if node, exists := dg.nodes[current]; exists {
			for parentPN := range node.DirectParents {
				inDegree[parentPN]--
				if inDegree[parentPN] == 0 {
					queue = append(queue, parentPN)
				}
			}
		}
	}

	return result
}

func (dg *IncrementalDependencyGraph) GetNodeInfo(
	partNumber entities.PartNumber,
) *IncrementalDependencyNode {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	if node, exists := dg.nodes[partNumber]; exists {
		// Return a copy to avoid concurrent access issues
		result := &IncrementalDependencyNode{
			PartNumber:     node.PartNumber,
			Item:           node.Item,
			Level:          node.Level,
			DirectChildren: make(map[entities.PartNumber]bool),
			DirectParents:  make(map[entities.PartNumber]bool),
			Requirements:   make(map[string]*entities.GrossRequirement),
			LastUpdated:    node.LastUpdated,
		}

		for child := range node.DirectChildren {
			result.DirectChildren[child] = true
		}
		for parent := range node.DirectParents {
			result.DirectParents[parent] = true
		}
		for key, req := range node.Requirements {
			result.Requirements[key] = req
		}

		return result
	}

	return nil
}
