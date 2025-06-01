package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vsinha/mrp/pkg/mrp"
	"github.com/vsinha/mrp/pkg/repository"
)

func main() {
	// Command line flags
	var (
		scenarioDir = flag.String("scenario", "", "Path to scenario directory containing CSV files")
		bomFile     = flag.String("bom", "", "Path to BOM CSV file")
		itemsFile   = flag.String("items", "", "Path to items CSV file")
		inventoryFile = flag.String("inventory", "", "Path to inventory CSV file")
		demandsFile = flag.String("demands", "", "Path to demands CSV file")
		outputDir   = flag.String("output", "", "Output directory for results (optional)")
		format      = flag.String("format", "text", "Output format: text, json, csv")
		verbose     = flag.Bool("verbose", false, "Enable verbose output")
		help        = flag.Bool("help", false, "Show help message")
	)
	
	flag.Parse()
	
	if *help {
		showHelp()
		return
	}
	
	// Validate inputs
	if *scenarioDir == "" && (*bomFile == "" || *itemsFile == "" || *inventoryFile == "" || *demandsFile == "") {
		fmt.Fprintf(os.Stderr, "Error: Must specify either -scenario directory or individual CSV files\n\n")
		showHelp()
		os.Exit(1)
	}
	
	// Determine input files
	var bomPath, itemsPath, inventoryPath, demandsPath string
	
	if *scenarioDir != "" {
		// Use scenario directory
		bomPath = filepath.Join(*scenarioDir, "bom.csv")
		itemsPath = filepath.Join(*scenarioDir, "items.csv")
		inventoryPath = filepath.Join(*scenarioDir, "inventory.csv")
		demandsPath = filepath.Join(*scenarioDir, "demands.csv")
	} else {
		// Use individual files
		bomPath = *bomFile
		itemsPath = *itemsFile
		inventoryPath = *inventoryFile
		demandsPath = *demandsFile
	}
	
	// Validate files exist
	files := map[string]string{
		"BOM":       bomPath,
		"Items":     itemsPath,
		"Inventory": inventoryPath,
		"Demands":   demandsPath,
	}
	
	for name, path := range files {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: %s file not found: %s\n", name, path)
			os.Exit(1)
		}
	}
	
	if *verbose {
		fmt.Printf("🚀 MRP Engine CLI\n")
		fmt.Printf("Input files:\n")
		fmt.Printf("  BOM: %s\n", bomPath)
		fmt.Printf("  Items: %s\n", itemsPath)
		fmt.Printf("  Inventory: %s\n", inventoryPath)
		fmt.Printf("  Demands: %s\n", demandsPath)
		fmt.Printf("Output format: %s\n", *format)
		if *outputDir != "" {
			fmt.Printf("Output directory: %s\n", *outputDir)
		}
		fmt.Println()
	}
	
	ctx := context.Background()
	
	// Load data from CSV files
	if *verbose {
		fmt.Println("📂 Loading data from CSV files...")
	}
	
	csvRepo := repository.NewCSVRepository()
	
	// Load items
	items, err := csvRepo.LoadItems(itemsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading items: %v\n", err)
		os.Exit(1)
	}
	
	// Load BOM
	bomLines, err := csvRepo.LoadBOM(bomPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading BOM: %v\n", err)
		os.Exit(1)
	}
	
	// Load inventory
	lotInventory, serialInventory, err := csvRepo.LoadInventory(inventoryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading inventory: %v\n", err)
		os.Exit(1)
	}
	
	// Load demands
	demands, err := csvRepo.LoadDemands(demandsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading demands: %v\n", err)
		os.Exit(1)
	}
	
	if *verbose {
		fmt.Printf("✅ Data loaded successfully:\n")
		fmt.Printf("  Items: %d\n", len(items))
		fmt.Printf("  BOM Lines: %d\n", len(bomLines))
		fmt.Printf("  Lot Inventory: %d\n", len(lotInventory))
		fmt.Printf("  Serial Inventory: %d\n", len(serialInventory))
		fmt.Printf("  Demands: %d\n", len(demands))
		fmt.Println()
	}
	
	// Create repositories
	var bomRepo mrp.BOMRepository
	var inventoryRepo mrp.InventoryRepository
	
	// Always use compact BOM repository for optimal performance
	compactBomRepo := mrp.NewCompactBOMRepository(len(items), len(bomLines))
	for _, item := range items {
		compactBomRepo.AddItem(item)
	}
	for _, line := range bomLines {
		compactBomRepo.AddBOMLine(line)
	}
	bomRepo = compactBomRepo
	
	if *verbose {
		fmt.Println("🔧 Using optimized compact BOM repository")
	}
	
	inMemoryInventoryRepo := mrp.NewInMemoryInventoryRepository()
	for _, lot := range lotInventory {
		inMemoryInventoryRepo.AddLotInventory(lot)
	}
	for _, serial := range serialInventory {
		inMemoryInventoryRepo.AddSerializedInventory(serial)
	}
	inventoryRepo = inMemoryInventoryRepo
	
	// Create MRP engine
	var engine mrp.MRPEngine
	
	// Always use optimized engine for best performance
	optimizationConfig := mrp.OptimizationConfig{
		EnableGCPacing:       true,
		CacheCleanupInterval: 5 * time.Minute,
		MaxCacheEntries:      10000,
		BatchSize:           1000,
	}
	engine = mrp.NewOptimizedEngine(bomRepo, inventoryRepo, optimizationConfig)
	
	if *verbose {
		fmt.Println("⚡ Using optimized MRP engine")
	}
	
	// Run MRP explosion
	if *verbose {
		fmt.Println("🔄 Running MRP explosion...")
	}
	
	startTime := time.Now()
	result, err := engine.ExplodeDemand(ctx, demands)
	explosionTime := time.Since(startTime)
	
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running MRP explosion: %v\n", err)
		os.Exit(1)
	}
	
	if *verbose {
		fmt.Printf("✅ MRP explosion completed in %v\n\n", explosionTime)
	}
	
	// Generate output
	outputConfig := OutputConfig{
		Format:        *format,
		OutputDir:     *outputDir,
		Verbose:       *verbose,
		ExplosionTime: explosionTime,
		InputFiles:    files,
	}
	
	err = generateOutput(result, outputConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating output: %v\n", err)
		os.Exit(1)
	}
	
	if *verbose {
		fmt.Println("🏁 MRP analysis complete!")
	}
}

func showHelp() {
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

    # Run with individual files
    mrp -bom data/bom.csv -items data/items.csv -inventory data/inventory.csv -demands data/demands.csv

    # Generate JSON output
    mrp -scenario examples/large_vehicle -format json -output results/

    # Run with verbose output
    mrp -scenario examples/apollo_saturn_v_stack -verbose
`)
}

type OutputConfig struct {
	Format        string
	OutputDir     string
	Verbose       bool
	ExplosionTime time.Duration
	InputFiles    map[string]string
}