package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/vsinha/mrp/pkg/interfaces/cli/commands"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	ctx := context.Background()

	switch command {
	case "run":
		runMRPCommand(ctx, os.Args[2:])
	case "generate":
		runGenerateCommand(ctx, os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runMRPCommand(ctx context.Context, args []string) {
	flagSet := flag.NewFlagSet("run", flag.ExitOnError)

	var (
		scenarioDir = flagSet.String(
			"scenario",
			"",
			"Path to scenario directory containing CSV files",
		)
		bomFile       = flagSet.String("bom", "", "Path to BOM CSV file")
		itemsFile     = flagSet.String("items", "", "Path to items CSV file")
		inventoryFile = flagSet.String("inventory", "", "Path to inventory CSV file")
		demandsFile   = flagSet.String("demands", "", "Path to demands CSV file")
		outputDir     = flagSet.String("output", "", "Output directory for results (optional)")
		format        = flagSet.String("format", "text", "Output format: text, json, csv")
		verbose       = flagSet.Bool("verbose", false, "Enable verbose output")
		criticalPath  = flagSet.Bool("critical-path", false, "Perform critical path analysis")
		topPaths      = flagSet.Int("top-paths", 3, "Number of top critical paths to analyze")
		help          = flagSet.Bool("help", false, "Show help message")
	)

	flagSet.Parse(args)

	// Create command configuration
	config := commands.Config{
		ScenarioDir:   *scenarioDir,
		BOMFile:       *bomFile,
		ItemsFile:     *itemsFile,
		InventoryFile: *inventoryFile,
		DemandsFile:   *demandsFile,
		OutputDir:     *outputDir,
		Format:        *format,
		Verbose:       *verbose,
		CriticalPath:  *criticalPath,
		TopPaths:      *topPaths,
		Help:          *help,
	}

	// Create and execute command
	cmd := commands.NewMRPCommand(config)

	if err := cmd.Execute(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runGenerateCommand(ctx context.Context, args []string) {
	flagSet := flag.NewFlagSet("generate", flag.ExitOnError)

	var (
		items     = flagSet.Int("items", 0, "Number of items to generate (required)")
		maxDepth  = flagSet.Int("max-depth", 0, "Maximum depth of BOM tree (required)")
		demands   = flagSet.Int("demands", 0, "Number of demand lines to generate (required)")
		inventory = flagSet.String("inventory", "", "Inventory multiplier (e.g., 0.5, 4.0) (required)")
		outputDir = flagSet.String("output", "", "Output directory for generated files (required)")
		seed      = flagSet.Int64("seed", 0, "Random seed for reproducible generation (optional)")
		verbose   = flagSet.Bool("verbose", false, "Enable verbose output")
		help      = flagSet.Bool("help", false, "Show help message")
	)

	flagSet.Parse(args)

	if *help {
		cmd := commands.NewGenerateCommand(commands.GenerateConfig{Help: true})
		cmd.Execute(ctx)
		return
	}

	// Validate required parameters
	if *items <= 0 || *maxDepth <= 0 || *demands <= 0 || *inventory == "" || *outputDir == "" {
		fmt.Fprintf(os.Stderr, "Error: Missing required parameters\n\n")
		cmd := commands.NewGenerateCommand(commands.GenerateConfig{Help: true})
		cmd.Execute(ctx)
		os.Exit(1)
	}

	// Parse inventory multiplier
	inventoryFloat, err := strconv.ParseFloat(*inventory, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid inventory multiplier: %s\n", *inventory)
		os.Exit(1)
	}

	// Create generate configuration
	config := commands.GenerateConfig{
		Items:     *items,
		MaxDepth:  *maxDepth,
		Demands:   *demands,
		Inventory: inventoryFloat,
		OutputDir: *outputDir,
		Seed:      *seed,
		Verbose:   *verbose,
	}

	// Create and execute command
	cmd := commands.NewGenerateCommand(config)

	if err := cmd.Execute(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`MRP Planning System

USAGE:
    mrp <COMMAND> [OPTIONS]

COMMANDS:
    run         Run MRP analysis on existing scenario
    generate    Generate new test scenarios
    help        Show this help message

EXAMPLES:
    # Run MRP on existing scenario
    mrp run --scenario ./examples/apollo_saturn_v

    # Generate new test scenario
    mrp generate --items 1000 --max-depth 6 --demands 20 --inventory 0.5 --output ./test_scenario

For command-specific help:
    mrp <COMMAND> --help`)
}
