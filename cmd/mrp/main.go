package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/vsinha/mrp/pkg/interfaces/cli/commands"
)

func main() {
	// Command line flags
	var (
		scenarioDir = flag.String(
			"scenario",
			"",
			"Path to scenario directory containing CSV files",
		)
		bomFile       = flag.String("bom", "", "Path to BOM CSV file")
		itemsFile     = flag.String("items", "", "Path to items CSV file")
		inventoryFile = flag.String("inventory", "", "Path to inventory CSV file")
		demandsFile   = flag.String("demands", "", "Path to demands CSV file")
		outputDir     = flag.String("output", "", "Output directory for results (optional)")
		format        = flag.String("format", "text", "Output format: text, json, csv")
		verbose       = flag.Bool("verbose", false, "Enable verbose output")
		criticalPath  = flag.Bool("critical-path", false, "Perform critical path analysis")
		topPaths      = flag.Int("top-paths", 3, "Number of top critical paths to analyze")
		help          = flag.Bool("help", false, "Show help message")
	)

	flag.Parse()

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
	ctx := context.Background()

	if err := cmd.Execute(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
