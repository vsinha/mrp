package incremental

import (
	"fmt"
	"sync"
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/infrastructure/events"
)

type SchedulingProcessor struct {
	dependencyGraph *IncrementalDependencyGraph
	eventStore      events.EventStore

	// Track completion times for incremental scheduling
	completionTimes map[entities.PartNumber]time.Time
	scheduledOrders map[entities.PartNumber][]*entities.PlannedOrder
	mutex           sync.RWMutex
}

func NewSchedulingProcessor(
	dependencyGraph *IncrementalDependencyGraph,
	eventStore events.EventStore,
) *SchedulingProcessor {
	return &SchedulingProcessor{
		dependencyGraph: dependencyGraph,
		eventStore:      eventStore,
		completionTimes: make(map[entities.PartNumber]time.Time),
		scheduledOrders: make(map[entities.PartNumber][]*entities.PlannedOrder),
	}
}

func (sp *SchedulingProcessor) Handle(event events.Event) error {
	switch event.Type() {
	case "net.requirement.created":
		return sp.handleNetRequirementCreated(event)
	case events.InventoryAllocatedEvent:
		return sp.handleInventoryAllocated(event)
	case events.OrderCancelledEvent:
		return sp.handleOrderCancelled(event)
	default:
		return nil
	}
}

func (sp *SchedulingProcessor) CanHandle(eventType string) bool {
	switch eventType {
	case "net.requirement.created", events.InventoryAllocatedEvent, events.OrderCancelledEvent:
		return true
	default:
		return false
	}
}

func (sp *SchedulingProcessor) handleNetRequirementCreated(event events.Event) error {
	// Extract net requirement from event data
	eventData, ok := event.Data().(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format for net requirement")
	}

	netReqData, ok := eventData["net_requirement"]
	if !ok {
		return fmt.Errorf("no net requirement found in event data")
	}

	netReq, ok := netReqData.(entities.NetRequirement)
	if !ok {
		return fmt.Errorf("invalid net requirement data type")
	}

	// Schedule this requirement incrementally
	orders, err := sp.schedulePartIncremental(netReq)
	if err != nil {
		return fmt.Errorf("failed to schedule part %s: %w", netReq.PartNumber, err)
	}

	// Track scheduled orders
	sp.mutex.Lock()
	sp.scheduledOrders[netReq.PartNumber] = append(sp.scheduledOrders[netReq.PartNumber], orders...)
	sp.mutex.Unlock()

	// Publish order planned events
	for _, order := range orders {
		orderEvent := events.NewOrderPlannedEvent(*order, netReq)
		if err := sp.eventStore.AppendEvent(string(order.PartNumber), orderEvent); err != nil {
			fmt.Printf("Warning: failed to publish order planned event: %v\n", err)
		}
	}

	// Update completion time and trigger dependent part rescheduling
	if len(orders) > 0 {
		latestCompletion := orders[len(orders)-1].DueDate
		sp.updateCompletionTime(netReq.PartNumber, latestCompletion)
		sp.rescheduleAffectedParts(netReq.PartNumber)
	}

	return nil
}

func (sp *SchedulingProcessor) handleInventoryAllocated(event events.Event) error {
	allocData, ok := event.Data().(events.InventoryAllocated)
	if !ok {
		return fmt.Errorf("invalid event data for inventory allocated")
	}

	allocation := allocData.AllocationResult

	// If part is fully satisfied by inventory, update completion time
	if allocation.RemainingDemand == 0 {
		sp.updateCompletionTime(allocation.PartNumber, time.Now())
		sp.rescheduleAffectedParts(allocation.PartNumber)
	}

	return nil
}

func (sp *SchedulingProcessor) handleOrderCancelled(event events.Event) error {
	cancelData, ok := event.Data().(events.OrderCancelled)
	if !ok {
		return fmt.Errorf("invalid event data for order cancelled")
	}

	cancelledOrder := cancelData.PlannedOrder

	// Remove cancelled order from tracking
	sp.mutex.Lock()
	orders := sp.scheduledOrders[cancelledOrder.PartNumber]
	newOrders := make([]*entities.PlannedOrder, 0, len(orders))
	for _, order := range orders {
		if !sp.ordersEqual(*order, cancelledOrder) {
			newOrders = append(newOrders, order)
		}
	}
	sp.scheduledOrders[cancelledOrder.PartNumber] = newOrders
	sp.mutex.Unlock()

	// Recalculate completion time and reschedule affected parts
	sp.recalculateCompletionTime(cancelledOrder.PartNumber)
	sp.rescheduleAffectedParts(cancelledOrder.PartNumber)

	return nil
}

func (sp *SchedulingProcessor) schedulePartIncremental(
	netReq entities.NetRequirement,
) ([]*entities.PlannedOrder, error) {
	// Get node info from dependency graph
	nodeInfo := sp.dependencyGraph.GetNodeInfo(netReq.PartNumber)
	if nodeInfo == nil || nodeInfo.Item == nil {
		return nil, fmt.Errorf("no item info found for part %s", netReq.PartNumber)
	}

	item := nodeInfo.Item

	// Calculate earliest start time based on dependencies
	earliestStart := sp.calculateEarliestStartTime(netReq.PartNumber, nodeInfo)

	// Apply lot sizing
	orderQty := sp.applyLotSizing(netReq.Quantity, item)

	// Determine order type
	var orderType entities.OrderType
	switch item.MakeBuyCode {
	case entities.MakeBuyMake:
		orderType = entities.Make
	case entities.MakeBuyBuy:
		orderType = entities.Buy
	default:
		orderType = entities.Make
	}

	// Split orders if necessary and schedule them sequentially
	orders := sp.splitOrderByMaxQtyForward(orderQty, item, netReq, orderType, earliestStart)

	// Convert to pointer slice
	result := make([]*entities.PlannedOrder, len(orders))
	for i := range orders {
		result[i] = &orders[i]
	}

	return result, nil
}

func (sp *SchedulingProcessor) calculateEarliestStartTime(
	partNumber entities.PartNumber,
	nodeInfo *IncrementalDependencyNode,
) time.Time {
	if len(nodeInfo.DirectChildren) == 0 {
		// Leaf part - can start immediately
		return time.Now()
	}

	// Find latest completion time among direct children
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	var latestChildCompletion time.Time
	for childPN := range nodeInfo.DirectChildren {
		if childCompletion, exists := sp.completionTimes[childPN]; exists {
			if childCompletion.After(latestChildCompletion) {
				latestChildCompletion = childCompletion
			}
		}
	}

	if latestChildCompletion.IsZero() {
		return time.Now()
	}

	return latestChildCompletion
}

func (sp *SchedulingProcessor) applyLotSizing(
	netQty entities.Quantity,
	item *entities.Item,
) entities.Quantity {
	switch item.LotSizeRule {
	case entities.LotForLot:
		return netQty
	case entities.MinimumQty:
		if netQty < item.MinOrderQty {
			return item.MinOrderQty
		}
		return netQty
	case entities.StandardPack:
		if item.MinOrderQty > 0 {
			packs := (netQty + item.MinOrderQty - 1) / item.MinOrderQty
			return packs * item.MinOrderQty
		}
		return netQty
	default:
		return netQty
	}
}

func (sp *SchedulingProcessor) splitOrderByMaxQtyForward(
	totalQty entities.Quantity,
	item *entities.Item,
	netReq entities.NetRequirement,
	orderType entities.OrderType,
	earliestStart time.Time,
) []entities.PlannedOrder {
	var orders []entities.PlannedOrder

	if totalQty <= item.MaxOrderQty {
		dueDate := earliestStart.Add(time.Duration(item.LeadTimeDays) * 24 * time.Hour)
		order, err := entities.NewPlannedOrder(
			netReq.PartNumber,
			totalQty,
			earliestStart,
			dueDate,
			netReq.DemandTrace,
			netReq.Location,
			orderType,
			netReq.TargetSerial,
		)
		if err == nil {
			orders = append(orders, *order)
		}
		return orders
	}

	// Split into multiple sequential orders
	remainingQty := totalQty
	orderNum := 1
	currentStartDate := earliestStart

	for remainingQty > 0 {
		thisOrderQty := remainingQty
		if thisOrderQty > item.MaxOrderQty {
			thisOrderQty = item.MaxOrderQty
		}

		dueDate := currentStartDate.Add(time.Duration(item.LeadTimeDays) * 24 * time.Hour)

		demandTrace := netReq.DemandTrace
		if orderNum > 1 {
			demandTrace = fmt.Sprintf("%s (Split %d)", netReq.DemandTrace, orderNum)
		}

		order, err := entities.NewPlannedOrder(
			netReq.PartNumber,
			thisOrderQty,
			currentStartDate,
			dueDate,
			demandTrace,
			netReq.Location,
			orderType,
			netReq.TargetSerial,
		)
		if err == nil {
			orders = append(orders, *order)
		}

		currentStartDate = dueDate
		remainingQty -= thisOrderQty
		orderNum++
	}

	return orders
}

func (sp *SchedulingProcessor) updateCompletionTime(
	partNumber entities.PartNumber,
	completionTime time.Time,
) {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	sp.completionTimes[partNumber] = completionTime
}

func (sp *SchedulingProcessor) rescheduleAffectedParts(changedPart entities.PartNumber) {
	// Get all parts that depend on the changed part
	affectedParts := sp.dependencyGraph.GetAffectedParts(changedPart)

	// Reschedule affected parts in topological order
	topologicalOrder := sp.dependencyGraph.GetTopologicalOrder()

	for _, partNumber := range topologicalOrder {
		// Only reschedule if this part is affected
		isAffected := false
		for _, affected := range affectedParts {
			if affected == partNumber {
				isAffected = true
				break
			}
		}

		if !isAffected {
			continue
		}

		// Check if this part has pending net requirements that need rescheduling
		nodeInfo := sp.dependencyGraph.GetNodeInfo(partNumber)
		if nodeInfo != nil && len(nodeInfo.Requirements) > 0 {
			// For simplicity, just log that rescheduling is needed
			// In a full implementation, would create new scheduling events
			fmt.Printf("Part %s needs rescheduling due to dependency change\n", partNumber)
		}
	}
}

func (sp *SchedulingProcessor) recalculateCompletionTime(partNumber entities.PartNumber) {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	orders := sp.scheduledOrders[partNumber]
	if len(orders) == 0 {
		delete(sp.completionTimes, partNumber)
		return
	}

	// Find latest completion time among remaining orders
	var latestCompletion time.Time
	for _, order := range orders {
		if order.DueDate.After(latestCompletion) {
			latestCompletion = order.DueDate
		}
	}

	sp.completionTimes[partNumber] = latestCompletion
}

func (sp *SchedulingProcessor) ordersEqual(order1, order2 entities.PlannedOrder) bool {
	return order1.PartNumber == order2.PartNumber &&
		order1.Quantity == order2.Quantity &&
		order1.StartDate.Equal(order2.StartDate) &&
		order1.DueDate.Equal(order2.DueDate) &&
		order1.DemandTrace == order2.DemandTrace &&
		order1.Location == order2.Location &&
		order1.OrderType == order2.OrderType &&
		order1.TargetSerial == order2.TargetSerial
}
