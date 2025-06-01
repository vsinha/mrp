package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vsinha/mrp/pkg/application/services"
	"github.com/vsinha/mrp/pkg/domain/entities"
	"github.com/vsinha/mrp/pkg/domain/services/bom_validator"
	"github.com/vsinha/mrp/pkg/infrastructure/repositories/csv"
	"github.com/vsinha/mrp/pkg/infrastructure/repositories/memory"
	"github.com/vsinha/mrp/pkg/interfaces/cli/output"
)

// Config holds configuration for the MRP command
type Config struct {
	ScenarioDir   string
	BOMFile       string
	ItemsFile     string
	InventoryFile string
	DemandsFile   string
	OutputDir     string
	Format        string
	Verbose       bool
	CriticalPath  bool
	TopPaths      int
	Help          bool
}

// MRPCommand handles the main MRP execution logic
type MRPCommand struct {
	config Config
}

// NewMRPCommand creates a new MRP command with the given configuration
func NewMRPCommand(config Config) *MRPCommand {
	return &MRPCommand{
		config: config,
	}
}

// Execute runs the MRP command
func (c *MRPCommand) Execute(ctx context.Context) error {
	if c.config.Help {
		c.showHelp()
		return nil
	}

	// Validate inputs
	if err := c.validateInputs(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	// Determine input files
	files, err := c.resolveInputFiles()
	if err != nil {
		return fmt.Errorf("failed to resolve input files: %w", err)
	}

	if c.config.Verbose {
		c.printHeader(files)
	}

	// Load data from CSV files
	if c.config.Verbose {
		fmt.Println("📂 Loading data from CSV files...")
	}

	csvLoader := csv.NewLoader()

	// Load data
	items, err := csvLoader.LoadItems(files["Items"])
	if err != nil {
		return fmt.Errorf("error loading items: %w", err)
	}

	bomLines, err := csvLoader.LoadBOM(files["BOM"])
	if err != nil {
		return fmt.Errorf("error loading BOM: %w", err)
	}

	lotInventory, serialInventory, err := csvLoader.LoadInventory(files["Inventory"])
	if err != nil {
		return fmt.Errorf("error loading inventory: %w", err)
	}

	demands, err := csvLoader.LoadDemands(files["Demands"])
	if err != nil {
		return fmt.Errorf("error loading demands: %w", err)
	}

	if c.config.Verbose {
		fmt.Printf("✅ Data loaded successfully:\n")
		fmt.Printf("  Items: %d\n", len(items))
		fmt.Printf("  BOM Lines: %d\n", len(bomLines))
		fmt.Printf("  Lot Inventory: %d\n", len(lotInventory))
		fmt.Printf("  Serial Inventory: %d\n", len(serialInventory))
		fmt.Printf("  Demands: %d\n", len(demands))
		fmt.Println()
	}

	// Create repositories
	bomRepo := memory.NewBOMRepository(len(bomLines))
	err = bomRepo.LoadBOMLines(bomLines)
	if err != nil {
		return fmt.Errorf("failed to load BOM lines into repository: %w", err)
	}

	itemRepo := memory.NewItemRepository(len(items))
	err = itemRepo.LoadItems(items)
	if err != nil {
		return fmt.Errorf("failed to load items into repository: %w", err)
	}

	// Validate BOM-Item consistency
	if c.config.Verbose {
		fmt.Println("🔍 Validating BOM-Item consistency...")
	}

	itemSlice := make([]entities.Item, len(items))
	for i, item := range items {
		itemSlice[i] = *item
	}

	bomSlice := make([]entities.BOMLine, len(bomLines))
	for i, line := range bomLines {
		bomSlice[i] = *line
	}

	consistencyValidation := bom_validator.ValidateBOMItemConsistency(bomSlice, itemSlice)
	if len(consistencyValidation.Errors) > 0 {
		return fmt.Errorf("BOM-Item consistency validation failed: %s",
			strings.Join(consistencyValidation.Errors, "; "))
	}

	if c.config.Verbose && len(consistencyValidation.OrphanedParts) == 0 {
		fmt.Println("✅ BOM-Item consistency validation passed")
	}

	inventoryRepo := memory.NewInventoryRepository()
	err = inventoryRepo.LoadInventoryLots(lotInventory)
	if err != nil {
		return fmt.Errorf("failed to load lot inventory into repository: %w", err)
	}
	err = inventoryRepo.LoadSerializedInventory(serialInventory)
	if err != nil {
		return fmt.Errorf("failed to load serialized inventory into repository: %w", err)
	}

	demandRepo := memory.NewDemandRepository()
	err = demandRepo.LoadDemands(demands)
	if err != nil {
		return fmt.Errorf("failed to load demands into repository: %w", err)
	}

	// Create services
	mrpService := services.NewMRPService()
	criticalPathService := services.NewCriticalPathService(bomRepo, itemRepo, inventoryRepo, nil)
	orchestrator := services.NewPlanningOrchestrator(
		mrpService,
		criticalPathService,
		bomRepo,
		itemRepo,
		inventoryRepo,
		demandRepo,
	)

	if c.config.Verbose {
		fmt.Println("⚡ Using optimized MRP service with clean architecture")
	}

	// Run MRP explosion
	if c.config.Verbose {
		fmt.Println("🔄 Running MRP explosion...")
	}

	startTime := time.Now()
	result, err := mrpService.ExplodeDemand(
		ctx,
		demands,
		bomRepo,
		itemRepo,
		inventoryRepo,
		demandRepo,
	)
	explosionTime := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("error running MRP explosion: %w", err)
	}

	if c.config.Verbose {
		fmt.Printf("✅ MRP explosion completed in %v\n\n", explosionTime)
	}

	// Perform critical path analysis if requested
	var criticalPathResults []*entities.CriticalPathAnalysis
	if c.config.CriticalPath {
		if c.config.Verbose {
			fmt.Println("🔍 Performing critical path analysis...")
		}

		criticalPathStartTime := time.Now()

		for _, demand := range demands {
			analysis, err := orchestrator.AnalyzeCriticalPathWithMRPResults(
				ctx,
				demand.PartNumber,
				demand.TargetSerial,
				demand.Location,
				c.config.TopPaths,
				result,
			)
			if err != nil {
				fmt.Printf(
					"Warning: Failed to analyze critical path for %s: %v\n",
					demand.PartNumber,
					err,
				)
				continue
			}
			criticalPathResults = append(criticalPathResults, analysis)

			if c.config.Verbose {
				fmt.Printf("📊 %s: %s\n", demand.PartNumber, analysis.GetCriticalPathSummary())
			}
		}

		criticalPathTime := time.Since(criticalPathStartTime)
		if c.config.Verbose {
			fmt.Printf("✅ Critical path analysis completed in %v\n\n", criticalPathTime)
		}
	}

	// Generate output
	outputConfig := output.Config{
		Format:        c.config.Format,
		OutputDir:     c.config.OutputDir,
		Verbose:       c.config.Verbose,
		ExplosionTime: explosionTime,
		InputFiles:    files,
	}

	err = output.Generate(result, outputConfig)
	if err != nil {
		return fmt.Errorf("error generating output: %w", err)
	}

	if c.config.Verbose {
		fmt.Println("🏁 MRP analysis complete!")
	}

	return nil
}

// validateInputs validates the command configuration
func (c *MRPCommand) validateInputs() error {
	if c.config.ScenarioDir == "" &&
		(c.config.BOMFile == "" || c.config.ItemsFile == "" ||
			c.config.InventoryFile == "" || c.config.DemandsFile == "") {
		return fmt.Errorf("must specify either -scenario directory or individual CSV files")
	}
	return nil
}

// resolveInputFiles determines the actual file paths to use
func (c *MRPCommand) resolveInputFiles() (map[string]string, error) {
	var bomPath, itemsPath, inventoryPath, demandsPath string

	if c.config.ScenarioDir != "" {
		// Use scenario directory
		bomPath = filepath.Join(c.config.ScenarioDir, "bom.csv")
		itemsPath = filepath.Join(c.config.ScenarioDir, "items.csv")
		inventoryPath = filepath.Join(c.config.ScenarioDir, "inventory.csv")
		demandsPath = filepath.Join(c.config.ScenarioDir, "demands.csv")
	} else {
		// Use individual files
		bomPath = c.config.BOMFile
		itemsPath = c.config.ItemsFile
		inventoryPath = c.config.InventoryFile
		demandsPath = c.config.DemandsFile
	}

	files := map[string]string{
		"BOM":       bomPath,
		"Items":     itemsPath,
		"Inventory": inventoryPath,
		"Demands":   demandsPath,
	}

	// Validate files exist
	for name, path := range files {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil, fmt.Errorf("%s file not found: %s", name, path)
		}
	}

	return files, nil
}

// printHeader prints the command header information
func (c *MRPCommand) printHeader(files map[string]string) {
	fmt.Printf("🚀 MRP Engine CLI\n")
	fmt.Printf("Input files:\n")
	fmt.Printf("  BOM: %s\n", files["BOM"])
	fmt.Printf("  Items: %s\n", files["Items"])
	fmt.Printf("  Inventory: %s\n", files["Inventory"])
	fmt.Printf("  Demands: %s\n", files["Demands"])
	fmt.Printf("Output format: %s\n", c.config.Format)
	if c.config.OutputDir != "" {
		fmt.Printf("Output directory: %s\n", c.config.OutputDir)
	}
	fmt.Println()
}

// showHelp displays the help message
func (c *MRPCommand) showHelp() {
	fmt.Printf(`MRP Engine CLI - Material Requirements Planning for Aerospace Manufacturing

USAGE:
    mrp -scenario <directory>              # Use scenario directory with CSV files
    mrp -bom <file> -items <file> ...      # Use individual CSV files

OPTIONS:
    -scenario <dir>     Path to scenario directory containing CSV files
    -bom <file>         Path to BOM CSV file
    -items <file>       Path to items CSV file  
    -inventory <file>   Path to inventory CSV file
    -demands <file>     Path to demands CSV file
    -output <dir>       Output directory for results (optional)
    -format <fmt>       Output format: text, json, csv (default: text)
    -verbose            Enable verbose output
    -critical-path      Perform critical path analysis on demands
    -top-paths <n>      Number of top critical paths to analyze (default: 3)
    -help               Show this help message

SCENARIO DIRECTORY STRUCTURE:
    scenario_name/
    ├── bom.csv         # Bill of Materials
    ├── items.csv       # Item master data
    ├── inventory.csv   # Available inventory
    └── demands.csv     # Demand requirements

CSV FILE FORMATS:

items.csv:
    part_number,description,lead_time_days,lot_size_rule,min_order_qty,safety_stock,unit_of_measure
    F1_ENGINE,F-1 Engine,120,LotForLot,1,2,EA

bom.csv:
    parent_pn,child_pn,qty_per,find_number,from_serial,to_serial
    F1_ENGINE,F1_TURBOPUMP_V1,1,100,AS501,AS506
    F1_ENGINE,F1_TURBOPUMP_V2,1,100,AS507,

inventory.csv:
    part_number,type,identifier,location,quantity,receipt_date,status
    F1_ENGINE,serial,F1_001,MICHOUD,1,1968-09-15,Available
    BOLT_M12,lot,BOLT_LOT_001,KENNEDY,1000,1968-04-10,Available

demands.csv:
    part_number,quantity,need_date,demand_source,location,target_serial
    F1_ENGINE,5,1969-07-04,APOLLO_11,KENNEDY,AS506

EXAMPLES:
    # Run aerospace scenario
    mrp -scenario examples/aerospace_basic -verbose

    # Run with critical path analysis
    mrp -scenario examples/aerospace_basic -critical-path -verbose

    # Analyze top 5 critical paths
    mrp -scenario examples/aerospace_basic -critical-path -top-paths 5

    # Run with individual files
    mrp -bom data/bom.csv -items data/items.csv -inventory data/inventory.csv -demands data/demands.csv

    # Generate JSON output with critical path
    mrp -scenario examples/large_vehicle -format json -output results/ -critical-path

    # Run with verbose output
    mrp -scenario examples/apollo_saturn_v_stack -verbose
`)
}
