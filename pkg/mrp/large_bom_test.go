package mrp

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"testing"
	"time"
)

// LargeBOMConfig defines parameters for generating realistic large BOMs
type LargeBOMConfig struct {
	TotalParts         int     // Target total number of parts (e.g., 30000)
	MaxLevels          int     // Maximum BOM depth (e.g., 10)
	AvgChildrenPerPart float64 // Average number of children per parent part
	SerialRanges       int     // Number of different serial effectivity ranges
	InventoryRatio     float64 // Fraction of parts that have inventory
	LeadTimeVariation  int     // Max lead time in days
}

// LargeBOMSynthesizer creates realistic large-scale BOMs for testing
type LargeBOMSynthesizer struct {
	config LargeBOMConfig
	rng    *rand.Rand
}

// NewLargeBOMSynthesizer creates a new BOM synthesizer with the given configuration
func NewLargeBOMSynthesizer(config LargeBOMConfig) *LargeBOMSynthesizer {
	return &LargeBOMSynthesizer{
		config: config,
		rng:    rand.New(rand.NewSource(42)), // Fixed seed for reproducible tests
	}
}

// SynthesizeAerospaceBOM creates a realistic aerospace BOM structure
func (s *LargeBOMSynthesizer) SynthesizeAerospaceBOM() (*BOMRepository, *InventoryRepository) {
	bomRepo := NewBOMRepository(s.config.TotalParts, s.config.TotalParts*2) // Conservative estimate for BOM lines
	inventoryRepo := NewInMemoryInventoryRepository()
	
	fmt.Printf("🏭 Synthesizing aerospace BOM with %d parts, %d levels...\n", 
		s.config.TotalParts, s.config.MaxLevels)
	
	// Calculate distribution of parts across levels
	levelsDistribution := s.calculateLevelDistribution()
	
	// Generate parts by level
	var allParts []PartInfo
	partsByLevel := make([][]PartInfo, s.config.MaxLevels)
	
	for level := 0; level < s.config.MaxLevels; level++ {
		levelParts := s.generatePartsForLevel(level, levelsDistribution[level])
		partsByLevel[level] = levelParts
		allParts = append(allParts, levelParts...)
		
		// Add items to repository
		for _, part := range levelParts {
			bomRepo.AddItem(part.Item)
		}
	}
	
	fmt.Printf("  Generated %d total parts across %d levels\n", len(allParts), s.config.MaxLevels)
	
	// Generate BOM relationships
	bomLineCount := s.generateBOMRelationships(bomRepo, partsByLevel)
	fmt.Printf("  Created %d BOM relationships\n", bomLineCount)
	
	// Generate inventory for some parts
	inventoryCount := s.generateInventory(inventoryRepo, allParts)
	fmt.Printf("  Generated inventory for %d parts\n", inventoryCount)
	
	return bomRepo, inventoryRepo
}

// PartInfo holds information about a generated part
type PartInfo struct {
	Item        Item
	Level       int
	PartType    PartType
	Criticality CriticalityLevel
}

type PartType int
const (
	Assembly PartType = iota
	Subassembly
	Component
	RawMaterial
)

type CriticalityLevel int
const (
	Critical CriticalityLevel = iota
	Important
	Standard
)

// calculateLevelDistribution determines how many parts should be at each level
func (s *LargeBOMSynthesizer) calculateLevelDistribution() []int {
	distribution := make([]int, s.config.MaxLevels)
	remaining := s.config.TotalParts
	
	// Use exponential distribution - fewer parts at top levels, more at bottom
	for level := 0; level < s.config.MaxLevels-1; level++ {
		// Top levels have fewer parts, bottom levels have more
		ratio := float64(level+1) / float64(s.config.MaxLevels)
		levelParts := int(float64(s.config.TotalParts) * ratio * 0.15)
		
		if levelParts > remaining {
			levelParts = remaining
		}
		
		distribution[level] = levelParts
		remaining -= levelParts
	}
	
	// Assign remaining parts to the last level (leaf nodes)
	distribution[s.config.MaxLevels-1] = remaining
	
	return distribution
}

// generatePartsForLevel creates parts for a specific BOM level
func (s *LargeBOMSynthesizer) generatePartsForLevel(level, count int) []PartInfo {
	parts := make([]PartInfo, count)
	
	for i := 0; i < count; i++ {
		partType := s.determinePartType(level)
		criticality := s.determineCriticality()
		
		partNumber := PartNumber(fmt.Sprintf("L%d_%s_%06d", level, s.getPartTypePrefix(partType), i))
		
		parts[i] = PartInfo{
			Item: Item{
				PartNumber:      partNumber,
				Description:     s.generatePartDescription(partType, level, i),
				LeadTimeDays:    s.generateLeadTime(partType, criticality),
				LotSizeRule:     s.generateLotSizeRule(partType),
				MinOrderQty:     s.generateMinOrderQty(partType),
				SafetyStock:     s.generateSafetyStock(criticality),
				UnitOfMeasure:   s.generateUnitOfMeasure(partType),
			},
			Level:       level,
			PartType:    partType,
			Criticality: criticality,
		}
	}
	
	return parts
}

// generateBOMRelationships creates parent-child relationships between parts
func (s *LargeBOMSynthesizer) generateBOMRelationships(bomRepo *BOMRepository, partsByLevel [][]PartInfo) int {
	bomLineCount := 0
	
	// Create relationships from each level to the next
	for level := 0; level < len(partsByLevel)-1; level++ {
		parents := partsByLevel[level]
		children := partsByLevel[level+1]
		
		if len(children) == 0 {
			continue
		}
		
		for _, parent := range parents {
			childCount := s.generateChildCount(parent.PartType)
			
			// Select random children for this parent
			selectedChildren := s.selectRandomChildren(children, childCount)
			
			for j, child := range selectedChildren {
				// Generate serial effectivity ranges
				effectivity := s.generateSerialEffectivity()
				
				bomLine := BOMLine{
					ParentPN:     parent.Item.PartNumber,
					ChildPN:      child.Item.PartNumber,
					QtyPer:       s.generateQtyPer(parent.PartType, child.PartType),
					FindNumber:   (j + 1) * 10, // 10, 20, 30, etc.
					Effectivity:  effectivity,
				}
				
				bomRepo.AddBOMLine(bomLine)
				bomLineCount++
			}
		}
	}
	
	return bomLineCount
}

// generateInventory creates inventory records for a subset of parts
func (s *LargeBOMSynthesizer) generateInventory(inventoryRepo *InventoryRepository, allParts []PartInfo) int {
	inventoryCount := 0
	baseDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	
	for _, part := range allParts {
		// Only create inventory for some parts
		if s.rng.Float64() > s.config.InventoryRatio {
			continue
		}
		
		// Determine if this should be serialized or lot-controlled inventory
		if s.shouldBeSerialized(part.PartType, part.Criticality) {
			// Create serialized inventory
			serialCount := s.rng.Intn(5) + 1 // 1-5 serialized items
			for i := 0; i < serialCount; i++ {
				serialNum := fmt.Sprintf("%s_S%03d", part.Item.PartNumber, i+1)
				receiptDate := baseDate.Add(time.Duration(s.rng.Intn(90)) * 24 * time.Hour)
				
				inventoryRepo.AddSerializedInventory(SerializedInventory{
					PartNumber:   part.Item.PartNumber,
					SerialNumber: serialNum,
					Location:     s.generateLocation(),
					Status:       Available,
					ReceiptDate:  receiptDate,
				})
				inventoryCount++
			}
		} else {
			// Create lot inventory
			lotCount := s.rng.Intn(3) + 1 // 1-3 lots
			for i := 0; i < lotCount; i++ {
				lotNum := fmt.Sprintf("LOT_%s_%03d", part.Item.PartNumber, i+1)
				quantity := s.generateLotQuantity(part.PartType)
				receiptDate := baseDate.Add(time.Duration(s.rng.Intn(90)) * 24 * time.Hour)
				
				inventoryRepo.AddLotInventory(InventoryLot{
					PartNumber:   part.Item.PartNumber,
					LotNumber:    lotNum,
					Location:     s.generateLocation(),
					Quantity:     quantity,
					ReceiptDate:  receiptDate,
					Status:       Available,
				})
				inventoryCount++
			}
		}
	}
	
	return inventoryCount
}

// Helper methods for generating realistic data

func (s *LargeBOMSynthesizer) determinePartType(level int) PartType {
	switch {
	case level == 0:
		return Assembly
	case level <= 2:
		return Subassembly
	case level <= 6:
		return Component
	default:
		return RawMaterial
	}
}

func (s *LargeBOMSynthesizer) determineCriticality() CriticalityLevel {
	roll := s.rng.Float64()
	switch {
	case roll < 0.1:
		return Critical
	case roll < 0.3:
		return Important
	default:
		return Standard
	}
}

func (s *LargeBOMSynthesizer) getPartTypePrefix(partType PartType) string {
	switch partType {
	case Assembly:
		return "ASM"
	case Subassembly:
		return "SUB"
	case Component:
		return "CMP"
	case RawMaterial:
		return "RAW"
	default:
		return "UNK"
	}
}

func (s *LargeBOMSynthesizer) generatePartDescription(partType PartType, level, index int) string {
	typeNames := map[PartType]string{
		Assembly:    "Assembly",
		Subassembly: "Subassembly",
		Component:   "Component",
		RawMaterial: "Raw Material",
	}
	
	return fmt.Sprintf("%s Level %d #%d", typeNames[partType], level, index)
}

func (s *LargeBOMSynthesizer) generateLeadTime(partType PartType, criticality CriticalityLevel) int {
	baseLeadTime := map[PartType]int{
		Assembly:    120,
		Subassembly: 90,
		Component:   60,
		RawMaterial: 30,
	}
	
	base := baseLeadTime[partType]
	
	// Critical parts take longer
	if criticality == Critical {
		base = int(float64(base) * 1.5)
	}
	
	// Add some variation
	variation := s.rng.Intn(s.config.LeadTimeVariation) - s.config.LeadTimeVariation/2
	result := base + variation
	
	if result < 1 {
		result = 1
	}
	
	return result
}

func (s *LargeBOMSynthesizer) generateLotSizeRule(partType PartType) LotSizeRule {
	switch partType {
	case Assembly, Subassembly:
		return LotForLot
	case Component:
		if s.rng.Float64() < 0.3 {
			return MinimumQty
		}
		return LotForLot
	case RawMaterial:
		if s.rng.Float64() < 0.6 {
			return StandardPack
		}
		return MinimumQty
	default:
		return LotForLot
	}
}

func (s *LargeBOMSynthesizer) generateMinOrderQty(partType PartType) Quantity {
	switch partType {
	case Assembly, Subassembly:
		return Quantity(1)
	case Component:
		return Quantity(s.rng.Intn(10) + 1)
	case RawMaterial:
		return Quantity(s.rng.Intn(100) + 1)
	default:
		return Quantity(1)
	}
}

func (s *LargeBOMSynthesizer) generateSafetyStock(criticality CriticalityLevel) Quantity {
	switch criticality {
	case Critical:
		return Quantity(s.rng.Intn(10) + 5)
	case Important:
		return Quantity(s.rng.Intn(5) + 1)
	default:
		return Quantity(0)
	}
}

func (s *LargeBOMSynthesizer) generateUnitOfMeasure(partType PartType) string {
	switch partType {
	case Assembly, Subassembly, Component:
		return "EA"
	case RawMaterial:
		units := []string{"KG", "M", "L", "EA", "M2"}
		return units[s.rng.Intn(len(units))]
	default:
		return "EA"
	}
}

func (s *LargeBOMSynthesizer) generateChildCount(partType PartType) int {
	base := int(s.config.AvgChildrenPerPart)
	
	switch partType {
	case Assembly:
		// Assemblies have more children
		return s.rng.Intn(base*2) + base/2
	case Subassembly:
		return s.rng.Intn(base) + base/2
	case Component:
		return s.rng.Intn(base/2 + 1) + 1
	default:
		return s.rng.Intn(3) + 1
	}
}

func (s *LargeBOMSynthesizer) selectRandomChildren(children []PartInfo, count int) []PartInfo {
	if count >= len(children) {
		return children
	}
	
	// Shuffle and take first 'count' elements
	shuffled := make([]PartInfo, len(children))
	copy(shuffled, children)
	
	for i := len(shuffled) - 1; i > 0; i-- {
		j := s.rng.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	
	return shuffled[:count]
}

func (s *LargeBOMSynthesizer) generateSerialEffectivity() SerialEffectivity {
	if s.rng.Float64() < 0.7 {
		// 70% of parts are effective for all serials
		return SerialEffectivity{FromSerial: "SN001", ToSerial: ""}
	}
	
	// 30% have limited ranges
	rangeStart := s.rng.Intn(s.config.SerialRanges) * 50 + 1
	rangeEnd := rangeStart + s.rng.Intn(100) + 25
	
	fromSerial := fmt.Sprintf("SN%03d", rangeStart)
	toSerial := ""
	
	if s.rng.Float64() < 0.5 {
		toSerial = fmt.Sprintf("SN%03d", rangeEnd)
	}
	
	return SerialEffectivity{FromSerial: fromSerial, ToSerial: toSerial}
}

func (s *LargeBOMSynthesizer) generateQtyPer(parentType, childType PartType) Quantity {
	switch {
	case childType == RawMaterial:
		// Raw materials often used in bulk
		qty := s.rng.Intn(50) + 1
		return Quantity(qty)
	case childType == Component:
		// Components typically used in small quantities
		qty := s.rng.Intn(10) + 1
		return Quantity(qty)
	default:
		// Assemblies and subassemblies typically 1-3 per parent
		qty := s.rng.Intn(3) + 1
		return Quantity(qty)
	}
}

func (s *LargeBOMSynthesizer) shouldBeSerialized(partType PartType, criticality CriticalityLevel) bool {
	switch partType {
	case Assembly, Subassembly:
		return true
	case Component:
		return criticality == Critical
	default:
		return false
	}
}

func (s *LargeBOMSynthesizer) generateLocation() string {
	locations := []string{"HUNTSVILLE", "STENNIS", "KENNEDY", "MICHOUD", "WALLOPS"}
	return locations[s.rng.Intn(len(locations))]
}

func (s *LargeBOMSynthesizer) generateLotQuantity(partType PartType) Quantity {
	switch partType {
	case RawMaterial:
		qty := s.rng.Intn(1000) + 100
		return Quantity(qty)
	case Component:
		qty := s.rng.Intn(100) + 10
		return Quantity(qty)
	default:
		qty := s.rng.Intn(10) + 1
		return Quantity(qty)
	}
}

// Large scale benchmarks

func BenchmarkMRPEngine_30KParts_10Levels(b *testing.B) {
	ctx := context.Background()
	
	config := LargeBOMConfig{
		TotalParts:         30000,
		MaxLevels:          10,
		AvgChildrenPerPart: 8.0,
		SerialRanges:       20,
		InventoryRatio:     0.15, // 15% of parts have inventory
		LeadTimeVariation:  30,
	}
	
	synthesizer := NewLargeBOMSynthesizer(config)
	bomRepo, inventoryRepo := synthesizer.SynthesizeAerospaceBOM()
	
	engine := NewTestEngine(bomRepo, inventoryRepo)
	
	// Create demand for top-level assembly
	demands := []DemandRequirement{
		{
			PartNumber:   "L0_ASM_000000", // First top-level assembly
			Quantity:     Quantity(1),
			NeedDate:     time.Now().Add(180 * 24 * time.Hour),
			DemandSource: "LARGE_SCALE_BENCHMARK",
			Location:     "KENNEDY",
			TargetSerial: "SN100",
		},
	}
	
	// Warm up the cache with one run
	_, err := engine.ExplodeDemand(ctx, demands)
	if err != nil {
		b.Fatalf("Warmup run failed: %v", err)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		result, err := engine.ExplodeDemand(ctx, demands)
		if err != nil {
			b.Fatalf("ExplodeDemand failed: %v", err)
		}
		
		// Prevent compiler optimization
		_ = result
	}
}

func BenchmarkMRPEngine_50KParts_12Levels(b *testing.B) {
	ctx := context.Background()
	
	config := LargeBOMConfig{
		TotalParts:         50000,
		MaxLevels:          12,
		AvgChildrenPerPart: 10.0,
		SerialRanges:       25,
		InventoryRatio:     0.12,
		LeadTimeVariation:  45,
	}
	
	synthesizer := NewLargeBOMSynthesizer(config)
	bomRepo, inventoryRepo := synthesizer.SynthesizeAerospaceBOM()
	
	engine := NewTestEngine(bomRepo, inventoryRepo)
	
	demands := []DemandRequirement{
		{
			PartNumber:   "L0_ASM_000000",
			Quantity:     Quantity(1),
			NeedDate:     time.Now().Add(200 * 24 * time.Hour),
			DemandSource: "EXTREME_SCALE_BENCHMARK",
			Location:     "KENNEDY",
			TargetSerial: "SN200",
		},
	}
	
	// Warm up
	_, err := engine.ExplodeDemand(ctx, demands)
	if err != nil {
		b.Fatalf("Warmup run failed: %v", err)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		result, err := engine.ExplodeDemand(ctx, demands)
		if err != nil {
			b.Fatalf("ExplodeDemand failed: %v", err)
		}
		_ = result
	}
}

func TestLargeBOMSynthesis(t *testing.T) {
	config := LargeBOMConfig{
		TotalParts:         1000, // Smaller for test
		MaxLevels:          5,
		AvgChildrenPerPart: 4.0,
		SerialRanges:       5,
		InventoryRatio:     0.2,
		LeadTimeVariation:  20,
	}
	
	synthesizer := NewLargeBOMSynthesizer(config)
	bomRepo, inventoryRepo := synthesizer.SynthesizeAerospaceBOM()
	
	// Verify basic structure
	ctx := context.Background()
	allItems, err := bomRepo.GetAllItems(ctx)
	if err != nil {
		t.Fatalf("Failed to get all items: %v", err)
	}
	
	if len(allItems) != config.TotalParts {
		t.Errorf("Expected %d parts, got %d", config.TotalParts, len(allItems))
	}
	
	allBOMLines, err := bomRepo.GetAllBOMLines(ctx)
	if err != nil {
		t.Fatalf("Failed to get all BOM lines: %v", err)
	}
	
	if len(allBOMLines) == 0 {
		t.Error("Expected BOM lines to be generated")
	}
	
	t.Logf("Generated BOM with %d parts and %d BOM lines", len(allItems), len(allBOMLines))
	
	// Test a small MRP run
	engine := NewTestEngine(bomRepo, inventoryRepo)
	demands := []DemandRequirement{
		{
			PartNumber:   "L0_ASM_000000",
			Quantity:     Quantity(1),
			NeedDate:     time.Now().Add(60 * 24 * time.Hour),
			DemandSource: "TEST_SYNTHESIS",
			Location:     "KENNEDY",
			TargetSerial: "SN001",
		},
	}
	
	result, err := engine.ExplodeDemand(ctx, demands)
	if err != nil {
		t.Fatalf("MRP explosion failed: %v", err)
	}
	
	t.Logf("MRP Results: %d planned orders, %d allocations, %d cache entries",
		len(result.PlannedOrders), len(result.Allocations), len(result.ExplosionCache))
}

func BenchmarkOptimizedMRPEngine_30KParts(b *testing.B) {
	ctx := context.Background()
	
	config := LargeBOMConfig{
		TotalParts:         30000,
		MaxLevels:          10,
		AvgChildrenPerPart: 8.0,
		SerialRanges:       20,
		InventoryRatio:     0.15,
		LeadTimeVariation:  30,
	}
	
	synthesizer := NewLargeBOMSynthesizer(config)
	bomRepo, inventoryRepo := synthesizer.SynthesizeAerospaceBOM()
	
	engineConfig := EngineConfig{
		EnableGCPacing:  true,
		MaxCacheEntries: 5000,
	}
	
	engine := NewEngineWithConfig(bomRepo, inventoryRepo, engineConfig)
	
	demands := []DemandRequirement{
		{
			PartNumber:   "L0_ASM_000000",
			Quantity:     Quantity(1),
			NeedDate:     time.Now().Add(180 * 24 * time.Hour),
			DemandSource: "OPTIMIZED_BENCHMARK",
			Location:     "KENNEDY",
			TargetSerial: "SN100",
		},
	}
	
	// Warm up
	_, err := engine.ExplodeDemand(ctx, demands)
	if err != nil {
		b.Fatalf("Warmup run failed: %v", err)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		result, err := engine.ExplodeDemand(ctx, demands)
		if err != nil {
			b.Fatalf("ExplodeDemand failed: %v", err)
		}
		_ = result
	}
}

func BenchmarkMemoryUsage_30KParts(b *testing.B) {
	ctx := context.Background()
	
	config := LargeBOMConfig{
		TotalParts:         30000,
		MaxLevels:          10,
		AvgChildrenPerPart: 8.0,
		SerialRanges:       20,
		InventoryRatio:     0.15,
		LeadTimeVariation:  30,
	}
	
	synthesizer := NewLargeBOMSynthesizer(config)
	
	b.Run("Standard_Engine", func(b *testing.B) {
		bomRepo, inventoryRepo := synthesizer.SynthesizeAerospaceBOM()
		engine := NewTestEngine(bomRepo, inventoryRepo)
		
		demands := []DemandRequirement{
			{
				PartNumber:   "L0_ASM_000000",
				Quantity:     Quantity(1),
				NeedDate:     time.Now().Add(180 * 24 * time.Hour),
				DemandSource: "MEMORY_BENCHMARK_STD",
				Location:     "KENNEDY",
				TargetSerial: "SN100",
			},
		}
		
		// Measure memory before
		runtime.GC()
		statsBefore := GetMemoryStats()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := engine.ExplodeDemand(ctx, demands)
			if err != nil {
				b.Fatalf("ExplodeDemand failed: %v", err)
			}
			_ = result
		}
		b.StopTimer()
		
		// Measure memory after
		runtime.GC()
		statsAfter := GetMemoryStats()
		
		b.Logf("Memory usage - Before: %s, After: %s, Increase: %s",
			FormatBytes(statsBefore.AllocBytes),
			FormatBytes(statsAfter.AllocBytes),
			FormatBytes(statsAfter.AllocBytes-statsBefore.AllocBytes))
	})
	
	b.Run("Optimized_Engine", func(b *testing.B) {
		bomRepo, inventoryRepo := synthesizer.SynthesizeAerospaceBOM()
		
		engineConfig := EngineConfig{
			EnableGCPacing:  true,
			MaxCacheEntries: 5000,
		}
		
		engine := NewEngineWithConfig(bomRepo, inventoryRepo, engineConfig)
		
		demands := []DemandRequirement{
			{
				PartNumber:   "L0_ASM_000000",
				Quantity:     Quantity(1),
				NeedDate:     time.Now().Add(180 * 24 * time.Hour),
				DemandSource: "MEMORY_BENCHMARK_OPT",
				Location:     "KENNEDY",
				TargetSerial: "SN100",
			},
		}
		
		// Measure memory before
		runtime.GC()
		statsBefore := GetMemoryStats()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := engine.ExplodeDemand(ctx, demands)
			if err != nil {
				b.Fatalf("ExplodeDemand failed: %v", err)
			}
			_ = result
		}
		b.StopTimer()
		
		// Measure memory after
		runtime.GC()
		statsAfter := GetMemoryStats()
		
		b.Logf("Memory usage - Before: %s, After: %s, Increase: %s",
			FormatBytes(statsBefore.AllocBytes),
			FormatBytes(statsAfter.AllocBytes),
			FormatBytes(statsAfter.AllocBytes-statsBefore.AllocBytes))
	})
}