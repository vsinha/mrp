package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vsinha/mrp/pkg/application/services/incremental"
	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/infrastructure/events"
	"github.com/vsinha/mrp/pkg/infrastructure/repositories/csv"
	"github.com/vsinha/mrp/pkg/infrastructure/repositories/memory"
)

// IncrementalConfig holds configuration for the incremental MRP command
type IncrementalConfig struct {
	ScenarioDir   string
	BOMFile       string
	ItemsFile     string
	InventoryFile string
	DemandsFile   string
	Verbose       bool
	Help          bool
}

// IncrementalCommand handles the interactive incremental MRP session
type IncrementalCommand struct {
	config       IncrementalConfig
	orchestrator *incremental.IncrementalMRPOrchestrator
	scanner      *bufio.Scanner
}

// NewIncrementalCommand creates a new incremental command with the given configuration
func NewIncrementalCommand(config IncrementalConfig) *IncrementalCommand {
	return &IncrementalCommand{
		config:  config,
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// Execute runs the incremental MRP command
func (c *IncrementalCommand) Execute(ctx context.Context) error {
	if c.config.Help {
		c.printHelp()
		return nil
	}

	// Resolve file paths
	if err := c.resolveFilePaths(); err != nil {
		return fmt.Errorf("failed to resolve file paths: %w", err)
	}

	// Load repositories
	bomRepo, itemRepo, inventoryRepo, err := c.loadRepositories()
	if err != nil {
		return fmt.Errorf("failed to load repositories: %w", err)
	}

	// Create orchestrator
	c.orchestrator = incremental.NewIncrementalMRPOrchestrator(bomRepo, itemRepo, inventoryRepo)

	// Start interactive session
	return c.runInteractiveSession(ctx)
}

func (c *IncrementalCommand) resolveFilePaths() error {
	if c.config.ScenarioDir != "" {
		// Use scenario directory
		c.config.BOMFile = filepath.Join(c.config.ScenarioDir, "bom.csv")
		c.config.ItemsFile = filepath.Join(c.config.ScenarioDir, "items.csv")
		c.config.InventoryFile = filepath.Join(c.config.ScenarioDir, "inventory.csv")
		c.config.DemandsFile = filepath.Join(c.config.ScenarioDir, "demands.csv")
	}

	// Validate required files exist
	requiredFiles := []string{c.config.BOMFile, c.config.ItemsFile, c.config.InventoryFile}
	for _, file := range requiredFiles {
		if file == "" {
			return fmt.Errorf("missing required file configuration")
		}
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", file)
		}
	}

	return nil
}

func (c *IncrementalCommand) loadRepositories() (*memory.BOMRepository, *memory.ItemRepository, *memory.InventoryRepository, error) {
	csvLoader := csv.NewLoader()

	// Load BOM data
	bomLines, err := csvLoader.LoadBOM(c.config.BOMFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load BOM: %w", err)
	}
	bomRepo := memory.NewBOMRepository(len(bomLines))
	err = bomRepo.LoadBOMLines(bomLines)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load BOM lines into repository: %w", err)
	}

	// Load item data
	items, err := csvLoader.LoadItems(c.config.ItemsFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load items: %w", err)
	}
	itemRepo := memory.NewItemRepository(len(items))
	err = itemRepo.LoadItems(items)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load items into repository: %w", err)
	}

	// Load inventory data
	lotInventory, serialInventory, err := csvLoader.LoadInventory(c.config.InventoryFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load inventory: %w", err)
	}
	inventoryRepo := memory.NewInventoryRepository()
	err = inventoryRepo.LoadInventoryLots(lotInventory)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load lot inventory into repository: %w", err)
	}
	err = inventoryRepo.LoadSerializedInventory(serialInventory)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"failed to load serialized inventory into repository: %w",
			err,
		)
	}

	return bomRepo, itemRepo, inventoryRepo, nil
}

func (c *IncrementalCommand) runInteractiveSession(ctx context.Context) error {
	fmt.Println("=== Incremental MRP Session ===")
	fmt.Println("Type 'help' for available commands")
	fmt.Println()

	for {
		fmt.Print("mrp> ")
		if !c.scanner.Scan() {
			break
		}

		line := strings.TrimSpace(c.scanner.Text())
		if line == "" {
			continue
		}

		if err := c.processCommand(ctx, line); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		fmt.Println()
	}

	return nil
}

func (c *IncrementalCommand) processCommand(ctx context.Context, line string) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	command := parts[0]
	args := parts[1:]

	switch command {
	case "help", "h":
		c.printInteractiveHelp()
	case "add-demand", "demand":
		return c.handleAddDemand(args)
	case "add-inventory", "inventory":
		return c.handleAddInventory(args)
	case "status":
		return c.handleStatus()
	case "events":
		return c.handleShowEvents(args)
	case "deps", "dependencies":
		return c.handleShowDependencies(args)
	case "quit", "q", "exit":
		fmt.Println("Goodbye!")
		os.Exit(0)
	default:
		return fmt.Errorf("unknown command: %s (type 'help' for available commands)", command)
	}

	return nil
}

func (c *IncrementalCommand) handleAddDemand(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf(
			"usage: add-demand <part-number> <quantity> <need-date> [location] [source]",
		)
	}

	partNumber := entities.PartNumber(args[0])

	quantity, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid quantity: %s", args[1])
	}

	needDate, err := time.Parse("2006-01-02", args[2])
	if err != nil {
		return fmt.Errorf("invalid date format (use YYYY-MM-DD): %s", args[2])
	}

	location := "MAIN"
	if len(args) > 3 {
		location = args[3]
	}

	source := "CLI"
	if len(args) > 4 {
		source = args[4]
	}

	// Create demand
	demand := entities.DemandRequirement{
		PartNumber:   partNumber,
		Quantity:     entities.Quantity(quantity),
		NeedDate:     needDate,
		DemandSource: source,
		Location:     location,
		TargetSerial: "001", // Default target serial
	}

	// Publish demand created event
	event := events.NewDemandCreatedEvent(demand)
	if err := c.orchestrator.PublishEvent(string(partNumber), event); err != nil {
		return fmt.Errorf("failed to publish demand event: %w", err)
	}

	fmt.Printf(
		"Added demand: %s qty %d needed by %s\n",
		partNumber,
		quantity,
		needDate.Format("2006-01-02"),
	)
	return nil
}

func (c *IncrementalCommand) handleAddInventory(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: add-inventory <part-number> <quantity> <lot-number> [location]")
	}

	partNumber := entities.PartNumber(args[0])

	quantity, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid quantity: %s", args[1])
	}

	lotNumber := args[2]

	location := "MAIN"
	if len(args) > 3 {
		location = args[3]
	}

	// Create inventory lot
	inventoryLot, err := entities.NewInventoryLot(
		partNumber,
		lotNumber,
		location,
		entities.Quantity(quantity),
		time.Now(),
		entities.Available,
	)
	if err != nil {
		return fmt.Errorf("failed to create inventory lot: %w", err)
	}

	// Publish inventory received event
	event := events.NewInventoryReceivedEvent(inventoryLot)
	if err := c.orchestrator.PublishEvent(string(partNumber), event); err != nil {
		return fmt.Errorf("failed to publish inventory event: %w", err)
	}

	fmt.Printf(
		"Added inventory: %s qty %d lot %s at %s\n",
		partNumber,
		quantity,
		lotNumber,
		location,
	)
	return nil
}

func (c *IncrementalCommand) handleStatus() error {
	eventStore := c.orchestrator.GetEventStore()

	// Get all events to understand current state
	allEvents, err := eventStore.ReadAllEvents(0)
	if err != nil {
		return fmt.Errorf("failed to read events: %w", err)
	}

	fmt.Printf("=== System Status ===\n")
	fmt.Printf("Total events processed: %d\n", len(allEvents))

	// Count events by type
	eventCounts := make(map[string]int)
	for _, event := range allEvents {
		eventCounts[event.Type()]++
	}

	fmt.Printf("\nEvent counts by type:\n")
	for eventType, count := range eventCounts {
		fmt.Printf("  %s: %d\n", eventType, count)
	}

	return nil
}

func (c *IncrementalCommand) handleShowEvents(args []string) error {
	eventStore := c.orchestrator.GetEventStore()

	limit := 10 // Default limit
	if len(args) > 0 {
		if l, err := strconv.Atoi(args[0]); err == nil {
			limit = l
		}
	}

	allEvents, err := eventStore.ReadAllEvents(0)
	if err != nil {
		return fmt.Errorf("failed to read events: %w", err)
	}

	fmt.Printf("=== Recent Events (last %d) ===\n", limit)
	start := len(allEvents) - limit
	if start < 0 {
		start = 0
	}

	for i := start; i < len(allEvents); i++ {
		event := allEvents[i]
		fmt.Printf("[%s] %s -> %s\n",
			event.Timestamp().Format("15:04:05"),
			event.Type(),
			event.StreamID())
	}

	return nil
}

func (c *IncrementalCommand) handleShowDependencies(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: dependencies <part-number>")
	}

	partNumber := entities.PartNumber(args[0])
	depGraph := c.orchestrator.GetDependencyGraph()

	nodeInfo := depGraph.GetNodeInfo(partNumber)
	if nodeInfo == nil {
		fmt.Printf("No dependency information found for part: %s\n", partNumber)
		return nil
	}

	fmt.Printf("=== Dependencies for %s ===\n", partNumber)
	fmt.Printf("Level: %d\n", nodeInfo.Level)
	fmt.Printf("Last updated: %s\n", nodeInfo.LastUpdated.Format("15:04:05"))

	if len(nodeInfo.DirectChildren) > 0 {
		fmt.Printf("\nDirect children (dependencies):\n")
		for childPN := range nodeInfo.DirectChildren {
			fmt.Printf("  - %s\n", childPN)
		}
	}

	if len(nodeInfo.DirectParents) > 0 {
		fmt.Printf("\nDirect parents (dependents):\n")
		for parentPN := range nodeInfo.DirectParents {
			fmt.Printf("  - %s\n", parentPN)
		}
	}

	if len(nodeInfo.Requirements) > 0 {
		fmt.Printf("\nActive requirements:\n")
		for trace, req := range nodeInfo.Requirements {
			fmt.Printf("  - %s: qty %d (trace: %s)\n", req.PartNumber, req.Quantity, trace)
		}
	}

	return nil
}

func (c *IncrementalCommand) printHelp() {
	fmt.Println(`Incremental MRP Command

USAGE:
    mrp incremental [OPTIONS]

OPTIONS:
    --scenario <DIR>    Path to scenario directory containing CSV files
    --bom <FILE>        Path to BOM CSV file
    --items <FILE>      Path to items CSV file
    --inventory <FILE>  Path to inventory CSV file
    --demands <FILE>    Path to demands CSV file (optional)
    --verbose           Enable verbose output
    --help              Show this help message

DESCRIPTION:
    Starts an interactive incremental MRP session where you can add demands,
    inventory, and see real-time MRP calculations.`)
}

func (c *IncrementalCommand) printInteractiveHelp() {
	fmt.Println(`Available commands:

  add-demand <part> <qty> <date> [location] [source]
      Add a new demand requirement
      Example: add-demand ENGINE_ASSEMBLY 5 2024-12-31 FACTORY

  add-inventory <part> <qty> <lot> [location]
      Add inventory to the system
      Example: add-inventory BOLT_M8 100 LOT001 WAREHOUSE

  status
      Show current system status and event counts

  events [limit]
      Show recent events (default: 10)
      Example: events 20

  dependencies <part>
      Show dependency information for a part
      Example: dependencies ENGINE_ASSEMBLY

  help, h
      Show this help message

  quit, q, exit
      Exit the incremental MRP session`)
}
