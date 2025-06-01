package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/vsinha/mrp/pkg/mrp"
)

// generateOutput generates formatted output based on configuration
func generateOutput(result *mrp.MRPResult, config OutputConfig) error {
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

// generateTextOutput generates human-readable text output
func generateTextOutput(result *mrp.MRPResult, config OutputConfig) error {
	var output string
	
	// Header
	output += "═══════════════════════════════════════════════════════════════\n"
	output += "                    MRP ANALYSIS RESULTS\n"
	output += "═══════════════════════════════════════════════════════════════\n\n"
	
	// Summary statistics
	output += fmt.Sprintf("📊 SUMMARY\n")
	output += fmt.Sprintf("  Explosion Time: %v\n", config.ExplosionTime)
	output += fmt.Sprintf("  Planned Orders: %d\n", len(result.PlannedOrders))
	output += fmt.Sprintf("  Inventory Allocations: %d\n", len(result.Allocations))
	output += fmt.Sprintf("  Material Shortages: %d\n", len(result.ShortageReport))
	output += fmt.Sprintf("  Cache Entries: %d\n", len(result.ExplosionCache))
	output += "\n"
	
	// Planned Orders
	if len(result.PlannedOrders) > 0 {
		output += "📝 PLANNED ORDERS\n"
		output += "────────────────────────────────────────────────────────────────\n"
		
		// Sort by due date
		sortedOrders := make([]mrp.PlannedOrder, len(result.PlannedOrders))
		copy(sortedOrders, result.PlannedOrders)
		sort.Slice(sortedOrders, func(i, j int) bool {
			return sortedOrders[i].DueDate.Before(sortedOrders[j].DueDate)
		})
		
		for _, order := range sortedOrders {
			output += fmt.Sprintf("Part: %-20s Qty: %8s  Due: %s\n",
				order.PartNumber,
				order.Quantity.Decimal().String(),
				order.DueDate.Format("2006-01-02"))
			output += fmt.Sprintf("  Type: %-8s Location: %-12s Serial: %s\n",
				order.OrderType.String(),
				order.Location,
				order.TargetSerial)
			output += fmt.Sprintf("  Trace: %s\n", order.DemandTrace)
			output += "\n"
		}
	}
	
	// Inventory Allocations
	if len(result.Allocations) > 0 {
		output += "📦 INVENTORY ALLOCATIONS\n"
		output += "────────────────────────────────────────────────────────────────\n"
		
		for _, alloc := range result.Allocations {
			output += fmt.Sprintf("Part: %-20s Location: %-12s\n",
				alloc.PartNumber, alloc.Location)
			output += fmt.Sprintf("  Allocated: %8s  Remaining Demand: %8s\n",
				alloc.AllocatedQty.Decimal().String(),
				alloc.RemainingDemand.Decimal().String())
			
			if len(alloc.AllocatedFrom) > 0 {
				output += "  Allocated From:\n"
				for _, from := range alloc.AllocatedFrom {
					if from.SerialNumber != "" {
						output += fmt.Sprintf("    Serial %-15s Qty: %s\n",
							from.SerialNumber, from.Quantity.Decimal().String())
					} else {
						output += fmt.Sprintf("    Lot %-18s Qty: %s\n",
							from.LotNumber, from.Quantity.Decimal().String())
					}
				}
			}
			output += "\n"
		}
	}
	
	// Material Shortages
	if len(result.ShortageReport) > 0 {
		output += "🚨 MATERIAL SHORTAGES\n"
		output += "────────────────────────────────────────────────────────────────\n"
		
		// Sort by need date
		sortedShortages := make([]mrp.Shortage, len(result.ShortageReport))
		copy(sortedShortages, result.ShortageReport)
		sort.Slice(sortedShortages, func(i, j int) bool {
			return sortedShortages[i].NeedDate.Before(sortedShortages[j].NeedDate)
		})
		
		for _, shortage := range sortedShortages {
			output += fmt.Sprintf("Part: %-20s Short: %8s  Need: %s\n",
				shortage.PartNumber,
				shortage.ShortQty.Decimal().String(),
				shortage.NeedDate.Format("2006-01-02"))
			output += fmt.Sprintf("  Location: %-12s Serial: %s\n",
				shortage.Location, shortage.TargetSerial)
			output += fmt.Sprintf("  Trace: %s\n", shortage.DemandTrace)
			output += "\n"
		}
	}
	
	// Cache Statistics
	if config.Verbose && len(result.ExplosionCache) > 0 {
		output += "🔧 EXPLOSION CACHE STATISTICS\n"
		output += "────────────────────────────────────────────────────────────────\n"
		
		// Analyze cache by part level
		levelStats := make(map[string]int)
		for key := range result.ExplosionCache {
			partStr := string(key.PartNumber)
			if len(partStr) > 2 && partStr[0] == 'L' {
				level := string(partStr[1])
				levelStats[level]++
			}
		}
		
		output += fmt.Sprintf("  Total Cache Entries: %d\n", len(result.ExplosionCache))
		if len(levelStats) > 0 {
			output += "  Entries by BOM Level:\n"
			for level, count := range levelStats {
				output += fmt.Sprintf("    Level %s: %d entries\n", level, count)
			}
		}
		output += "\n"
	}
	
	output += "═══════════════════════════════════════════════════════════════\n"
	
	// Write to file or stdout
	if config.OutputDir != "" {
		err := os.MkdirAll(config.OutputDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		
		filename := filepath.Join(config.OutputDir, "mrp_results.txt")
		err = os.WriteFile(filename, []byte(output), 0644)
		if err != nil {
			return fmt.Errorf("failed to write text output: %w", err)
		}
		
		if config.Verbose {
			fmt.Printf("📄 Text output written to: %s\n", filename)
		}
	} else {
		fmt.Print(output)
	}
	
	return nil
}

// generateJSONOutput generates JSON output
func generateJSONOutput(result *mrp.MRPResult, config OutputConfig) error {
	// Create a JSON-friendly structure
	jsonResult := struct {
		Metadata struct {
			ExplosionTime string            `json:"explosion_time"`
			GeneratedAt   string            `json:"generated_at"`
			InputFiles    map[string]string `json:"input_files"`
		} `json:"metadata"`
		Summary struct {
			PlannedOrdersCount    int `json:"planned_orders_count"`
			AllocationsCount      int `json:"allocations_count"`
			ShortagesCount        int `json:"shortages_count"`
			CacheEntriesCount     int `json:"cache_entries_count"`
		} `json:"summary"`
		PlannedOrders   []mrp.PlannedOrder     `json:"planned_orders"`
		Allocations     []mrp.AllocationResult `json:"allocations"`
		ShortageReport  []mrp.Shortage         `json:"shortages"`
	}{
		PlannedOrders:  result.PlannedOrders,
		Allocations:    result.Allocations,
		ShortageReport: result.ShortageReport,
	}
	
	// Set metadata
	jsonResult.Metadata.ExplosionTime = config.ExplosionTime.String()
	jsonResult.Metadata.GeneratedAt = time.Now().Format(time.RFC3339)
	jsonResult.Metadata.InputFiles = config.InputFiles
	
	// Set summary
	jsonResult.Summary.PlannedOrdersCount = len(result.PlannedOrders)
	jsonResult.Summary.AllocationsCount = len(result.Allocations)
	jsonResult.Summary.ShortagesCount = len(result.ShortageReport)
	jsonResult.Summary.CacheEntriesCount = len(result.ExplosionCache)
	
	// Marshal to JSON
	jsonBytes, err := json.MarshalIndent(jsonResult, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	// Write to file or stdout
	if config.OutputDir != "" {
		err := os.MkdirAll(config.OutputDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		
		filename := filepath.Join(config.OutputDir, "mrp_results.json")
		err = os.WriteFile(filename, jsonBytes, 0644)
		if err != nil {
			return fmt.Errorf("failed to write JSON output: %w", err)
		}
		
		if config.Verbose {
			fmt.Printf("📄 JSON output written to: %s\n", filename)
		}
	} else {
		fmt.Printf("%s\n", jsonBytes)
	}
	
	return nil
}

// generateCSVOutput generates CSV output files
func generateCSVOutput(result *mrp.MRPResult, config OutputConfig) error {
	if config.OutputDir == "" {
		return fmt.Errorf("CSV output requires an output directory (-output)")
	}
	
	err := os.MkdirAll(config.OutputDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Generate planned orders CSV
	if len(result.PlannedOrders) > 0 {
		err := writeOrdersCSV(result.PlannedOrders, filepath.Join(config.OutputDir, "planned_orders.csv"))
		if err != nil {
			return fmt.Errorf("failed to write planned orders CSV: %w", err)
		}
		
		if config.Verbose {
			fmt.Printf("📄 Planned orders CSV written to: %s\n", filepath.Join(config.OutputDir, "planned_orders.csv"))
		}
	}
	
	// Generate allocations CSV
	if len(result.Allocations) > 0 {
		err := writeAllocationsCSV(result.Allocations, filepath.Join(config.OutputDir, "allocations.csv"))
		if err != nil {
			return fmt.Errorf("failed to write allocations CSV: %w", err)
		}
		
		if config.Verbose {
			fmt.Printf("📄 Allocations CSV written to: %s\n", filepath.Join(config.OutputDir, "allocations.csv"))
		}
	}
	
	// Generate shortages CSV
	if len(result.ShortageReport) > 0 {
		err := writeShortagesCSV(result.ShortageReport, filepath.Join(config.OutputDir, "shortages.csv"))
		if err != nil {
			return fmt.Errorf("failed to write shortages CSV: %w", err)
		}
		
		if config.Verbose {
			fmt.Printf("📄 Shortages CSV written to: %s\n", filepath.Join(config.OutputDir, "shortages.csv"))
		}
	}
	
	return nil
}

// Helper functions for CSV writing

func writeOrdersCSV(orders []mrp.PlannedOrder, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	// Write header
	header := []string{"part_number", "quantity", "start_date", "due_date", "demand_trace", "location", "order_type", "target_serial"}
	err = writer.Write(header)
	if err != nil {
		return err
	}
	
	// Write data
	for _, order := range orders {
		record := []string{
			string(order.PartNumber),
			order.Quantity.Decimal().String(),
			order.StartDate.Format("2006-01-02"),
			order.DueDate.Format("2006-01-02"),
			order.DemandTrace,
			order.Location,
			order.OrderType.String(),
			order.TargetSerial,
		}
		
		err = writer.Write(record)
		if err != nil {
			return err
		}
	}
	
	return nil
}

func writeAllocationsCSV(allocations []mrp.AllocationResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	// Write header
	header := []string{"part_number", "location", "allocated_qty", "remaining_demand", "allocation_source", "source_identifier", "source_qty"}
	err = writer.Write(header)
	if err != nil {
		return err
	}
	
	// Write data
	for _, alloc := range allocations {
		if len(alloc.AllocatedFrom) == 0 {
			// No specific allocations, write summary row
			record := []string{
				string(alloc.PartNumber),
				alloc.Location,
				alloc.AllocatedQty.Decimal().String(),
				alloc.RemainingDemand.Decimal().String(),
				"",
				"",
				"",
			}
			
			err = writer.Write(record)
			if err != nil {
				return err
			}
		} else {
			// Write detailed allocation rows
			for _, from := range alloc.AllocatedFrom {
				sourceType := "lot"
				sourceID := from.LotNumber
				
				if from.SerialNumber != "" {
					sourceType = "serial"
					sourceID = from.SerialNumber
				}
				
				record := []string{
					string(alloc.PartNumber),
					alloc.Location,
					alloc.AllocatedQty.Decimal().String(),
					alloc.RemainingDemand.Decimal().String(),
					sourceType,
					sourceID,
					from.Quantity.Decimal().String(),
				}
				
				err = writer.Write(record)
				if err != nil {
					return err
				}
			}
		}
	}
	
	return nil
}

func writeShortagesCSV(shortages []mrp.Shortage, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	// Write header
	header := []string{"part_number", "location", "short_qty", "need_date", "demand_trace", "target_serial"}
	err = writer.Write(header)
	if err != nil {
		return err
	}
	
	// Write data
	for _, shortage := range shortages {
		record := []string{
			string(shortage.PartNumber),
			shortage.Location,
			shortage.ShortQty.Decimal().String(),
			shortage.NeedDate.Format("2006-01-02"),
			shortage.DemandTrace,
			shortage.TargetSerial,
		}
		
		err = writer.Write(record)
		if err != nil {
			return err
		}
	}
	
	return nil
}