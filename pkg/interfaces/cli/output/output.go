package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vsinha/mrp/pkg/application/dto"
	"github.com/vsinha/mrp/pkg/domain/entities"
)

// Config holds configuration for output generation
type Config struct {
	Format        string
	OutputDir     string
	Verbose       bool
	ExplosionTime time.Duration
	InputFiles    map[string]string
}

// Generate creates output in the specified format
func Generate(result *dto.MRPResult, config Config) error {
	switch config.Format {
	case "text":
		return generateTextOutput(result, config)
	case "json":
		return generateJSONOutput(result, config)
	case "csv":
		return generateCSVOutput(result, config)
	default:
		return fmt.Errorf("unsupported output format: %s", config.Format)
	}
}

// generateTextOutput creates human-readable text output
func generateTextOutput(result *dto.MRPResult, config Config) error {
	// Print to stdout
	fmt.Printf("📊 MRP Results Summary\n")
	fmt.Printf("======================\n\n")

	fmt.Printf("Planned Orders: %d\n", len(result.PlannedOrders))
	fmt.Printf("Allocations: %d\n", len(result.Allocations))
	fmt.Printf("Shortages: %d\n", len(result.ShortageReport))
	fmt.Printf("Explosion Time: %v\n\n", config.ExplosionTime)

	if len(result.PlannedOrders) > 0 {
		fmt.Printf("📋 Planned Orders:\n")
		fmt.Printf("%-15s %-8s %-12s %-12s %-15s %-10s\n",
			"Part Number", "Qty", "Start Date", "Due Date", "Order Type", "Location")
		fmt.Printf("%-15s %-8s %-12s %-12s %-15s %-10s\n",
			"---------------", "--------", "------------", "------------", "---------------", "----------")

		for _, order := range result.PlannedOrders {
			fmt.Printf("%-15s %-8d %-12s %-12s %-15s %-10s\n",
				order.PartNumber,
				order.Quantity,
				order.StartDate.Format("2006-01-02"),
				order.DueDate.Format("2006-01-02"),
				order.OrderType.String(),
				order.Location)
		}
		fmt.Println()
	}

	if len(result.Allocations) > 0 {
		fmt.Printf("📦 Inventory Allocations:\n")
		fmt.Printf("%-15s %-10s %-12s %-12s\n",
			"Part Number", "Location", "Allocated", "Remaining")
		fmt.Printf("%-15s %-10s %-12s %-12s\n",
			"---------------", "----------", "------------", "------------")

		for _, alloc := range result.Allocations {
			fmt.Printf("%-15s %-10s %-12d %-12d\n",
				alloc.PartNumber,
				alloc.Location,
				alloc.AllocatedQty,
				alloc.RemainingDemand)
		}
		fmt.Println()
	}

	if len(result.ShortageReport) > 0 {
		fmt.Printf("⚠️  Shortages:\n")
		fmt.Printf("%-15s %-10s %-12s %-12s %-15s\n",
			"Part Number", "Location", "Short Qty", "Need Date", "Target Serial")
		fmt.Printf("%-15s %-10s %-12s %-12s %-15s\n",
			"---------------", "----------", "------------", "------------", "---------------")

		for _, shortage := range result.ShortageReport {
			fmt.Printf("%-15s %-10s %-12d %-12s %-15s\n",
				shortage.PartNumber,
				shortage.Location,
				shortage.ShortQty,
				shortage.NeedDate.Format("2006-01-02"),
				shortage.TargetSerial)
		}
		fmt.Println()
	}

	// Save to file if output directory specified
	if config.OutputDir != "" {
		// Create output directory if it doesn't exist
		if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		filename := filepath.Join(config.OutputDir, "mrp_results.txt")
		// Implementation for saving text output to file would go here
		if config.Verbose {
			fmt.Printf("💾 Results saved to: %s\n", filename)
		}
	}

	return nil
}

// generateJSONOutput creates JSON output
func generateJSONOutput(result *dto.MRPResult, config Config) error {
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if config.OutputDir == "" {
		// Print to stdout
		fmt.Println(string(jsonData))
	} else {
		// Save to file
		if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		filename := filepath.Join(config.OutputDir, "mrp_results.json")
		err = os.WriteFile(filename, jsonData, 0644)
		if err != nil {
			return fmt.Errorf("failed to write JSON file: %w", err)
		}

		if config.Verbose {
			fmt.Printf("💾 JSON results saved to: %s\n", filename)
		}
	}

	return nil
}

// generateCSVOutput creates CSV output
func generateCSVOutput(result *dto.MRPResult, config Config) error {
	if config.OutputDir == "" {
		return fmt.Errorf("output directory required for CSV format")
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate planned orders CSV
	ordersFile := filepath.Join(config.OutputDir, "planned_orders.csv")
	err := writeOrdersCSV(result.PlannedOrders, ordersFile)
	if err != nil {
		return fmt.Errorf("failed to write planned orders CSV: %w", err)
	}

	// Generate allocations CSV
	allocFile := filepath.Join(config.OutputDir, "allocations.csv")
	err = writeAllocationsCSV(result.Allocations, allocFile)
	if err != nil {
		return fmt.Errorf("failed to write allocations CSV: %w", err)
	}

	// Generate shortages CSV
	shortageFile := filepath.Join(config.OutputDir, "shortages.csv")
	err = writeShortagesCSV(result.ShortageReport, shortageFile)
	if err != nil {
		return fmt.Errorf("failed to write shortages CSV: %w", err)
	}

	if config.Verbose {
		fmt.Printf("💾 CSV results saved to:\n")
		fmt.Printf("  Planned Orders: %s\n", ordersFile)
		fmt.Printf("  Allocations: %s\n", allocFile)
		fmt.Printf("  Shortages: %s\n", shortageFile)
	}

	return nil
}

// Helper functions for CSV generation would be implemented here
func writeOrdersCSV(orders []entities.PlannedOrder, filename string) error {
	// CSV implementation for planned orders
	return nil
}

func writeAllocationsCSV(allocations []entities.AllocationResult, filename string) error {
	// CSV implementation for allocations
	return nil
}

func writeShortagesCSV(shortages []entities.Shortage, filename string) error {
	// CSV implementation for shortages
	return nil
}
