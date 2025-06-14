package commands

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

// GenerateConfig holds configuration for scenario generation
type GenerateConfig struct {
	Items     int     // Total number of items to generate
	MaxDepth  int     // Maximum depth of BOM tree
	Demands   int     // Number of top-level demand lines
	Inventory float64 // Inventory multiplier (e.g., 0.5 = half coverage, 4.0 = 4x coverage)
	OutputDir string  // Output directory for generated files
	Seed      int64   // Random seed for reproducible generation
	Help      bool    // Show help
	Verbose   bool    // Verbose output
}

// GenerateCommand handles scenario generation
type GenerateCommand struct {
	config GenerateConfig
	rand   *rand.Rand
}

// NewGenerateCommand creates a new generate command
func NewGenerateCommand(config GenerateConfig) *GenerateCommand {
	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	return &GenerateCommand{
		config: config,
		rand:   rand.New(rand.NewSource(seed)),
	}
}

// BOMNode represents a node in the BOM tree
type BOMNode struct {
	PartNumber string
	Level      int
	Children   []*BOMNode
	Parents    []*BOMNode
	Quantity   entities.Quantity
	IsRoot     bool
	IsShared   bool
}

// Execute runs the generate command
func (cmd *GenerateCommand) Execute(ctx context.Context) error {
	if cmd.config.Help {
		cmd.printHelp()
		return nil
	}

	if cmd.config.Verbose {
		fmt.Printf(
			"🔧 Generating scenario with %d items, max depth %d, %d demands, %.1fx inventory\n",
			cmd.config.Items,
			cmd.config.MaxDepth,
			cmd.config.Demands,
			cmd.config.Inventory,
		)
		fmt.Printf("📁 Output directory: %s\n", cmd.config.OutputDir)
		fmt.Printf("🎲 Random seed: %d\n", cmd.config.Seed)
	}

	// Create output directory
	if err := os.MkdirAll(cmd.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate BOM tree structure
	if cmd.config.Verbose {
		fmt.Println("🌳 Generating BOM tree structure...")
	}
	tree, err := cmd.generateBOMTree()
	if err != nil {
		return fmt.Errorf("failed to generate BOM tree: %w", err)
	}

	// Generate items.csv
	if cmd.config.Verbose {
		fmt.Println("📦 Generating items.csv...")
	}
	if err := cmd.generateItems(tree); err != nil {
		return fmt.Errorf("failed to generate items: %w", err)
	}

	// Generate bom.csv
	if cmd.config.Verbose {
		fmt.Println("🔗 Generating bom.csv...")
	}
	if err := cmd.generateBOM(tree); err != nil {
		return fmt.Errorf("failed to generate BOM: %w", err)
	}

	// Generate demands.csv
	if cmd.config.Verbose {
		fmt.Println("📋 Generating demands.csv...")
	}
	if err := cmd.generateDemands(tree); err != nil {
		return fmt.Errorf("failed to generate demands: %w", err)
	}

	// Generate inventory.csv
	if cmd.config.Verbose {
		fmt.Println("📦 Generating inventory.csv...")
	}
	if err := cmd.generateInventory(tree); err != nil {
		return fmt.Errorf("failed to generate inventory: %w", err)
	}

	if cmd.config.Verbose {
		fmt.Printf("✅ Scenario generated successfully in %s\n", cmd.config.OutputDir)
	}

	return nil
}

// generateBOMTree creates a realistic BOM tree with shared components
func (cmd *GenerateCommand) generateBOMTree() (map[string]*BOMNode, error) {
	nodes := make(map[string]*BOMNode)
	var roots []*BOMNode

	// Calculate number of root nodes (about 1-3% of total items)
	numRoots := max(1, cmd.config.Items/50+cmd.rand.Intn(3))

	// Create root nodes
	for i := 0; i < numRoots; i++ {
		partNum := fmt.Sprintf("ROOT_ASSEMBLY_%03d", i+1)
		node := &BOMNode{
			PartNumber: partNum,
			Level:      0,
			IsRoot:     true,
			Children:   make([]*BOMNode, 0),
			Parents:    make([]*BOMNode, 0),
		}
		nodes[partNum] = node
		roots = append(roots, node)
	}

	itemsGenerated := numRoots

	// Generate tree level by level
	currentLevel := roots
	level := 0

	for level < cmd.config.MaxDepth && itemsGenerated < cmd.config.Items {
		level++
		var nextLevel []*BOMNode

		for _, parent := range currentLevel {
			// Each parent gets 2-8 children
			numChildren := 2 + cmd.rand.Intn(7)

			for child := 0; child < numChildren && itemsGenerated < cmd.config.Items; child++ {
				// 20% chance to reuse existing part from this level or lower levels
				var childNode *BOMNode
				if level > 1 && cmd.rand.Float64() < 0.2 {
					// Try to find an existing part to share
					candidates := cmd.findShareableParts(nodes, level, parent)
					if len(candidates) > 0 {
						existing := candidates[cmd.rand.Intn(len(candidates))]
						childNode = existing
						childNode.IsShared = true
					}
				}

				// Create new part if not sharing
				if childNode == nil {
					partNum := fmt.Sprintf("PART_L%d_%04d", level, itemsGenerated)
					childNode = &BOMNode{
						PartNumber: partNum,
						Level:      level,
						Children:   make([]*BOMNode, 0),
						Parents:    make([]*BOMNode, 0),
					}
					nodes[partNum] = childNode
					nextLevel = append(nextLevel, childNode)
					itemsGenerated++
				}

				// Add parent-child relationship
				parent.Children = append(parent.Children, childNode)
				childNode.Parents = append(childNode.Parents, parent)

				// Set quantity (1-10, higher quantities for lower levels)
				baseQty := 1 + cmd.rand.Intn(5)
				if level > 2 {
					baseQty += cmd.rand.Intn(5) // 1-10 for lower levels
				}
				childNode.Quantity = entities.Quantity(baseQty)
			}
		}

		if len(nextLevel) == 0 {
			break
		}
		currentLevel = nextLevel
	}

	// Fill remaining items as leaf components if needed
	for itemsGenerated < cmd.config.Items {
		partNum := fmt.Sprintf("COMPONENT_%04d", itemsGenerated)
		node := &BOMNode{
			PartNumber: partNum,
			Level:      level + 1,
			Children:   make([]*BOMNode, 0),
			Parents:    make([]*BOMNode, 0),
		}
		nodes[partNum] = node

		// Attach to random existing parent
		if len(currentLevel) > 0 {
			parent := currentLevel[cmd.rand.Intn(len(currentLevel))]
			parent.Children = append(parent.Children, node)
			node.Parents = append(node.Parents, parent)
			node.Quantity = entities.Quantity(1 + cmd.rand.Intn(10))
		}

		itemsGenerated++
	}

	return nodes, nil
}

// findShareableParts finds existing parts that can be shared, avoiding circular references
func (cmd *GenerateCommand) findShareableParts(
	nodes map[string]*BOMNode,
	maxLevel int,
	parent *BOMNode,
) []*BOMNode {
	var candidates []*BOMNode
	for _, node := range nodes {
		if node.Level >= maxLevel-1 && len(node.Parents) < 3 { // Don't over-share
			// Check for circular reference - node cannot be an ancestor of parent
			if !cmd.isAncestor(node, parent) && node != parent {
				candidates = append(candidates, node)
			}
		}
	}
	return candidates
}

// isAncestor checks if candidate is an ancestor of node (would create circular reference)
func (cmd *GenerateCommand) isAncestor(candidate, node *BOMNode) bool {
	visited := make(map[string]bool)
	return cmd.isAncestorHelper(candidate, node, visited)
}

// isAncestorHelper recursively checks for ancestry with cycle detection
func (cmd *GenerateCommand) isAncestorHelper(
	candidate, node *BOMNode,
	visited map[string]bool,
) bool {
	if visited[node.PartNumber] {
		return false // Avoid infinite loops
	}
	visited[node.PartNumber] = true

	for _, parent := range node.Parents {
		if parent.PartNumber == candidate.PartNumber {
			return true
		}
		if cmd.isAncestorHelper(candidate, parent, visited) {
			return true
		}
	}
	return false
}

// generateItems creates the items.csv file
func (cmd *GenerateCommand) generateItems(nodes map[string]*BOMNode) error {
	filePath := filepath.Join(cmd.config.OutputDir, "items.csv")
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header
	fmt.Fprintln(
		file,
		"part_number,description,lead_time_days,lot_size_rule,min_order_qty,max_order_qty,safety_stock,unit_of_measure,make_buy_code",
	)

	// Generate items
	for _, node := range nodes {
		desc := cmd.generateDescription(node)
		leadTime := cmd.generateLeadTime(node)
		lotRule, minQty, safetyStock := cmd.generateLotSizing(node)
		maxQty := cmd.generateMaxOrderQty(node, minQty)
		makeBuyCode := cmd.generateMakeBuyCode(node)

		fmt.Fprintf(file, "%s,%s,%d,%s,%d,%d,%d,EA,%s\n",
			node.PartNumber, desc, leadTime, lotRule, minQty, maxQty, safetyStock, makeBuyCode)
	}

	return nil
}

// generateDescription creates a realistic description
func (cmd *GenerateCommand) generateDescription(node *BOMNode) string {
	if node.IsRoot {
		return fmt.Sprintf("%s Complete Assembly", node.PartNumber)
	}

	if node.Level <= 2 {
		return fmt.Sprintf("%s Subassembly", node.PartNumber)
	}

	componentTypes := []string{"Component", "Module", "Unit", "Assembly", "Block", "Element"}
	return fmt.Sprintf("%s %s", node.PartNumber, componentTypes[cmd.rand.Intn(len(componentTypes))])
}

// generateLeadTime creates realistic lead times based on level
func (cmd *GenerateCommand) generateLeadTime(node *BOMNode) int {
	// Higher level = longer lead time, with randomness
	baseTime := 0
	switch {
	case node.IsRoot:
		baseTime = 300 + cmd.rand.Intn(150) // 300-450 days
	case node.Level <= 1:
		baseTime = 180 + cmd.rand.Intn(120) // 180-300 days
	case node.Level <= 2:
		baseTime = 90 + cmd.rand.Intn(90) // 90-180 days
	case node.Level <= 3:
		baseTime = 30 + cmd.rand.Intn(60) // 30-90 days
	default:
		baseTime = 7 + cmd.rand.Intn(23) // 7-30 days
	}
	return baseTime
}

// generateLotSizing creates varied lot sizing rules
func (cmd *GenerateCommand) generateLotSizing(node *BOMNode) (string, int, int) {
	if node.IsRoot || node.Level <= 2 {
		return "LotForLot", 1, 0
	}

	roll := cmd.rand.Float64()
	switch {
	case roll < 0.6:
		return "LotForLot", 1, cmd.rand.Intn(3)
	case roll < 0.8:
		minQty := 5 + cmd.rand.Intn(15)
		return "MinimumQty", minQty, cmd.rand.Intn(5)
	default:
		packSize := 10 + cmd.rand.Intn(90)
		return "StandardPack", packSize, packSize / 10
	}
}

// generateMaxOrderQty creates realistic maximum order quantities
func (cmd *GenerateCommand) generateMaxOrderQty(node *BOMNode, minQty int) int {
	baseMax := minQty

	switch {
	case node.IsRoot:
		// Root assemblies: small batches (1-10)
		baseMax = minQty + cmd.rand.Intn(10)
	case node.Level <= 2:
		// Subassemblies: medium batches (minQty * 2-20)
		multiplier := 2 + cmd.rand.Intn(19) // 2-20x
		baseMax = minQty * multiplier
	case node.Level <= 4:
		// Components: larger batches (minQty * 5-50)
		multiplier := 5 + cmd.rand.Intn(46) // 5-50x
		baseMax = minQty * multiplier
	default:
		// Raw materials: very large batches (minQty * 10-200)
		multiplier := 10 + cmd.rand.Intn(191) // 10-200x
		baseMax = minQty * multiplier
	}

	// Ensure minimum of minQty
	if baseMax < minQty {
		baseMax = minQty
	}

	return baseMax
}

// generateMakeBuyCode determines if an item should be made or bought
func (cmd *GenerateCommand) generateMakeBuyCode(node *BOMNode) string {
	// Business logic for make vs buy decisions
	switch {
	case node.IsRoot:
		// Root assemblies are typically made (final assembly)
		return "Make"
	case node.Level <= 2:
		// Subassemblies: 70% make, 30% buy
		if cmd.rand.Float32() < 0.7 {
			return "Make"
		}
		return "Buy"
	case node.Level <= 4:
		// Components: 50% make, 50% buy
		if cmd.rand.Float32() < 0.5 {
			return "Make"
		}
		return "Buy"
	default:
		// Raw materials and basic parts: mostly buy (80% buy)
		if cmd.rand.Float32() < 0.8 {
			return "Buy"
		}
		return "Make"
	}
}

// generateBOM creates the bom.csv file
func (cmd *GenerateCommand) generateBOM(nodes map[string]*BOMNode) error {
	filePath := filepath.Join(cmd.config.OutputDir, "bom.csv")
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header
	fmt.Fprintln(file, "parent_pn,child_pn,qty_per,find_number,from_serial,to_serial,priority")

	findNum := 100
	for _, parent := range nodes {
		for _, child := range parent.Children {
			fmt.Fprintf(file, "%s,%s,%d,%d,SN001,,0\n",
				parent.PartNumber, child.PartNumber, child.Quantity, findNum)
			findNum += 100
		}
	}

	return nil
}

// generateDemands creates the demands.csv file
func (cmd *GenerateCommand) generateDemands(nodes map[string]*BOMNode) error {
	filePath := filepath.Join(cmd.config.OutputDir, "demands.csv")
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header
	fmt.Fprintln(file, "part_number,quantity,need_date,demand_source,location,target_serial")

	// Find root nodes
	var roots []*BOMNode
	for _, node := range nodes {
		if node.IsRoot {
			roots = append(roots, node)
		}
	}

	// Generate demands
	baseDate := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < cmd.config.Demands; i++ {
		root := roots[cmd.rand.Intn(len(roots))]
		qty := 1 + cmd.rand.Intn(5) // 1-5 units

		// Vary need dates
		daysOffset := cmd.rand.Intn(365) // Within 1 year
		needDate := baseDate.AddDate(0, 0, daysOffset)

		serial := fmt.Sprintf("SN%03d", i+1)
		source := fmt.Sprintf("DEMAND_%03d", i+1)
		location := cmd.generateLocation()

		fmt.Fprintf(file, "%s,%d,%s,%s,%s,%s\n",
			root.PartNumber, qty, needDate.Format("2006-01-02"), source, location, serial)
	}

	return nil
}

// generateInventory creates the inventory.csv file
func (cmd *GenerateCommand) generateInventory(nodes map[string]*BOMNode) error {
	filePath := filepath.Join(cmd.config.OutputDir, "inventory.csv")
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header
	fmt.Fprintln(file, "part_number,type,identifier,location,quantity,receipt_date,status")

	// Calculate total parts needed for one complete assembly
	partCounts := cmd.calculatePartCounts(nodes)

	// Generate inventory based on multiplier
	baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	lotNum := 1
	serialNum := 1

	for partNum, totalNeeded := range partCounts {
		node := nodes[partNum]
		inventoryQty := int(float64(totalNeeded) * cmd.config.Inventory)

		if inventoryQty <= 0 {
			continue
		}

		location := cmd.generateLocation()
		receiptDate := baseDate.AddDate(0, 0, cmd.rand.Intn(365))

		// High-level parts get serial tracking, low-level get lot tracking
		if node.Level <= 2 || node.IsRoot {
			// Serial inventory
			for i := 0; i < inventoryQty; i++ {
				identifier := fmt.Sprintf("SER_%06d", serialNum)
				fmt.Fprintf(file, "%s,serial,%s,%s,1,%s,Available\n",
					partNum, identifier, location, receiptDate.Format("2006-01-02"))
				serialNum++
			}
		} else {
			// Lot inventory
			identifier := fmt.Sprintf("LOT_%06d", lotNum)
			fmt.Fprintf(file, "%s,lot,%s,%s,%d,%s,Available\n",
				partNum, identifier, location, inventoryQty, receiptDate.Format("2006-01-02"))
			lotNum++
		}
	}

	return nil
}

// calculatePartCounts calculates how many of each part is needed for one complete assembly
func (cmd *GenerateCommand) calculatePartCounts(nodes map[string]*BOMNode) map[string]int {
	counts := make(map[string]int)

	// Find a root and calculate explosion
	for _, node := range nodes {
		if node.IsRoot {
			cmd.explodePart(node, 1, counts, make(map[string]bool))
			break // Just use one root for calculation
		}
	}

	return counts
}

// explodePart recursively calculates part requirements with better cycle detection
func (cmd *GenerateCommand) explodePart(
	node *BOMNode,
	qty int,
	counts map[string]int,
	visited map[string]bool,
) {
	// Avoid infinite loops in shared parts - just use part number for visited check
	if visited[node.PartNumber] {
		return
	}
	visited[node.PartNumber] = true

	counts[node.PartNumber] += qty

	for _, child := range node.Children {
		childQty := qty * int(child.Quantity)
		// Create a new visited map for each child path to allow proper shared component handling
		childVisited := make(map[string]bool)
		for k, v := range visited {
			childVisited[k] = v
		}
		cmd.explodePart(child, childQty, counts, childVisited)
	}
}

// generateLocation creates realistic location names
func (cmd *GenerateCommand) generateLocation() string {
	locations := []string{
		"FACTORY_A",
		"FACTORY_B",
		"WAREHOUSE_1",
		"WAREHOUSE_2",
		"PLANT_NORTH",
		"PLANT_SOUTH",
	}
	return locations[cmd.rand.Intn(len(locations))]
}

// printHelp shows usage information
func (cmd *GenerateCommand) printHelp() {
	fmt.Println(`MRP Scenario Generator

USAGE:
    mrp generate [OPTIONS]

OPTIONS:
    --items <N>         Number of items to generate (required)
    --max-depth <N>     Maximum depth of BOM tree (required)
    --demands <N>       Number of demand lines to generate (required)
    --inventory <F>     Inventory multiplier (e.g., 0.5 = half coverage, 4.0 = 4x coverage) (required)
    --output <DIR>      Output directory for generated files (required)
    --seed <N>          Random seed for reproducible generation (optional)
    --verbose           Enable verbose output
    --help              Show this help message

EXAMPLES:
    # Generate small test scenario
    mrp generate --items 100 --max-depth 5 --demands 10 --inventory 0.5 --output ./test_scenario

    # Generate large performance test scenario  
    mrp generate --items 30000 --max-depth 8 --demands 50 --inventory 1.2 --output ./large_scenario --verbose

    # Generate reproducible scenario
    mrp generate --items 1000 --max-depth 6 --demands 20 --inventory 0.8 --output ./repro_scenario --seed 12345`)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
