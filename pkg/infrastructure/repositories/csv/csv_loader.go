package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vsinha/mrp/pkg/domain/entities"
)

// Loader handles loading MRP data from CSV files
type Loader struct{}

// NewLoader creates a new CSV loader
func NewLoader() *Loader {
	return &Loader{}
}

// LoadItems loads items from a CSV file
func (l *Loader) LoadItems(filename string) ([]*entities.Item, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open items file %s: %w", filename, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read items CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("items CSV must have header and at least one data row")
	}

	// Validate header
	expectedHeader := []string{
		"part_number",
		"description",
		"lead_time_days",
		"lot_size_rule",
		"min_order_qty",
		"max_order_qty",
		"safety_stock",
		"unit_of_measure",
		"make_buy_code",
	}
	header := records[0]
	if !validateHeader(header, expectedHeader) {
		return nil, fmt.Errorf(
			"items CSV header mismatch. Expected: %v, Got: %v",
			expectedHeader,
			header,
		)
	}

	var items []*entities.Item
	for i, record := range records[1:] {
		if len(record) != len(expectedHeader) {
			return nil, fmt.Errorf(
				"items CSV row %d: expected %d columns, got %d",
				i+2,
				len(expectedHeader),
				len(record),
			)
		}

		item, err := parseItem(record)
		if err != nil {
			return nil, fmt.Errorf("items CSV row %d: %w", i+2, err)
		}

		items = append(items, &item)
	}

	return items, nil
}

// LoadBOM loads BOM lines from a CSV file
func (l *Loader) LoadBOM(filename string) ([]*entities.BOMLine, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open BOM file %s: %w", filename, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read BOM CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("BOM CSV must have header and at least one data row")
	}

	// Validate header - support both old format (without priority) and new format (with priority)
	expectedHeaderOld := []string{
		"parent_pn",
		"child_pn",
		"qty_per",
		"find_number",
		"from_serial",
		"to_serial",
	}
	expectedHeaderNew := []string{
		"parent_pn",
		"child_pn",
		"qty_per",
		"find_number",
		"from_serial",
		"to_serial",
		"priority",
	}
	header := records[0]

	var hasPriority bool
	if validateHeader(header, expectedHeaderNew) {
		hasPriority = true
	} else if validateHeader(header, expectedHeaderOld) {
		hasPriority = false
	} else {
		return nil, fmt.Errorf("BOM CSV header mismatch. Expected: %v or %v, Got: %v", expectedHeaderOld, expectedHeaderNew, header)
	}

	var bomLines []*entities.BOMLine
	for i, record := range records[1:] {
		expectedCols := len(expectedHeaderOld)
		if hasPriority {
			expectedCols = len(expectedHeaderNew)
		}

		if len(record) != expectedCols {
			return nil, fmt.Errorf(
				"BOM CSV row %d: expected %d columns, got %d",
				i+2,
				expectedCols,
				len(record),
			)
		}

		bomLine, err := parseBOMLineWithPriority(record, hasPriority)
		if err != nil {
			return nil, fmt.Errorf("BOM CSV row %d: %w", i+2, err)
		}

		bomLines = append(bomLines, &bomLine)
	}

	return bomLines, nil
}

// LoadInventory loads inventory from a CSV file
func (l *Loader) LoadInventory(
	filename string,
) ([]*entities.InventoryLot, []*entities.SerializedInventory, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open inventory file %s: %w", filename, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read inventory CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, nil, fmt.Errorf("inventory CSV must have header and at least one data row")
	}

	// Validate header
	expectedHeader := []string{
		"part_number",
		"type",
		"identifier",
		"location",
		"quantity",
		"receipt_date",
		"status",
	}
	header := records[0]
	if !validateHeader(header, expectedHeader) {
		return nil, nil, fmt.Errorf(
			"inventory CSV header mismatch. Expected: %v, Got: %v",
			expectedHeader,
			header,
		)
	}

	var lotInventory []*entities.InventoryLot
	var serialInventory []*entities.SerializedInventory

	for i, record := range records[1:] {
		if len(record) != len(expectedHeader) {
			return nil, nil, fmt.Errorf(
				"inventory CSV row %d: expected %d columns, got %d",
				i+2,
				len(expectedHeader),
				len(record),
			)
		}

		partNumber := entities.PartNumber(record[0])
		invType := strings.ToLower(record[1])
		identifier := record[2]
		location := record[3]
		quantityStr := record[4]
		receiptDateStr := record[5]
		statusStr := record[6]

		// Parse common fields
		receiptDate, err := time.Parse("2006-01-02", receiptDateStr)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"invalid receipt_date format in row %d: %s (expected YYYY-MM-DD)",
				i+2,
				receiptDateStr,
			)
		}

		status, err := parseInventoryStatus(statusStr)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid status in row %d: %w", i+2, err)
		}

		switch invType {
		case "lot":
			quantity, err := strconv.ParseInt(quantityStr, 10, 64)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid quantity in row %d: %s", i+2, quantityStr)
			}

			lot, err := entities.NewInventoryLot(
				partNumber,
				identifier,
				location,
				entities.Quantity(quantity),
				receiptDate,
				status,
			)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid inventory lot in row %d: %w", i+2, err)
			}
			lotInventory = append(lotInventory, lot)

		case "serial":
			// For serialized inventory, quantity should be 1 (ignore CSV value)
			serial, err := entities.NewSerializedInventory(
				partNumber,
				identifier,
				location,
				status,
				receiptDate,
			)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid serialized inventory in row %d: %w", i+2, err)
			}
			serialInventory = append(serialInventory, serial)

		default:
			return nil, nil, fmt.Errorf(
				"invalid inventory type in row %d: %s (expected 'lot' or 'serial')",
				i+2,
				invType,
			)
		}
	}

	return lotInventory, serialInventory, nil
}

// LoadDemands loads demand requirements from a CSV file
func (l *Loader) LoadDemands(filename string) ([]*entities.DemandRequirement, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open demands file %s: %w", filename, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read demands CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("demands CSV must have header and at least one data row")
	}

	// Validate header
	expectedHeader := []string{
		"part_number",
		"quantity",
		"need_date",
		"demand_source",
		"location",
		"target_serial",
	}
	header := records[0]
	if !validateHeader(header, expectedHeader) {
		return nil, fmt.Errorf(
			"demands CSV header mismatch. Expected: %v, Got: %v",
			expectedHeader,
			header,
		)
	}

	var demands []*entities.DemandRequirement
	for i, record := range records[1:] {
		if len(record) != len(expectedHeader) {
			return nil, fmt.Errorf(
				"demands CSV row %d: expected %d columns, got %d",
				i+2,
				len(expectedHeader),
				len(record),
			)
		}

		demand, err := parseDemand(record)
		if err != nil {
			return nil, fmt.Errorf("demands CSV row %d: %w", i+2, err)
		}

		demands = append(demands, &demand)
	}

	return demands, nil
}

// Helper functions for parsing CSV records

func validateHeader(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}

	for i, col := range expected {
		if strings.ToLower(strings.TrimSpace(actual[i])) != col {
			return false
		}
	}

	return true
}

func parseItem(record []string) (entities.Item, error) {
	partNumber := entities.PartNumber(record[0])
	description := record[1]

	leadTimeDays, err := strconv.Atoi(record[2])
	if err != nil {
		return entities.Item{}, fmt.Errorf("invalid lead_time_days: %s", record[2])
	}

	lotSizeRule, err := parseLotSizeRule(record[3])
	if err != nil {
		return entities.Item{}, err
	}

	minOrderQty, err := strconv.ParseInt(record[4], 10, 64)
	if err != nil {
		return entities.Item{}, fmt.Errorf("invalid min_order_qty: %s", record[4])
	}

	maxOrderQty, err := strconv.ParseInt(record[5], 10, 64)
	if err != nil {
		return entities.Item{}, fmt.Errorf("invalid max_order_qty: %s", record[5])
	}

	safetyStock, err := strconv.ParseInt(record[6], 10, 64)
	if err != nil {
		return entities.Item{}, fmt.Errorf("invalid safety_stock: %s", record[6])
	}

	unitOfMeasure := record[7]

	makeBuyCode, err := parseMakeBuyCode(record[8])
	if err != nil {
		return entities.Item{}, err
	}

	item, err := entities.NewItem(
		partNumber,
		description,
		leadTimeDays,
		lotSizeRule,
		entities.Quantity(minOrderQty),
		entities.Quantity(maxOrderQty),
		entities.Quantity(safetyStock),
		unitOfMeasure,
		makeBuyCode,
	)
	if err != nil {
		return entities.Item{}, fmt.Errorf("invalid item: %w", err)
	}
	return *item, nil
}

func parseBOMLine(record []string) (entities.BOMLine, error) {
	parentPN := entities.PartNumber(record[0])
	childPN := entities.PartNumber(record[1])

	qtyPer, err := strconv.ParseInt(record[2], 10, 64)
	if err != nil {
		return entities.BOMLine{}, fmt.Errorf("invalid qty_per: %s", record[2])
	}

	findNumber, err := strconv.Atoi(record[3])
	if err != nil {
		return entities.BOMLine{}, fmt.Errorf("invalid find_number: %s", record[3])
	}

	fromSerial := record[4]
	toSerial := record[5]

	effectivity, err := entities.NewSerialEffectivity(fromSerial, toSerial)
	if err != nil {
		return entities.BOMLine{}, fmt.Errorf("invalid serial effectivity: %w", err)
	}

	bomLine, err := entities.NewBOMLine(
		parentPN,
		childPN,
		entities.Quantity(qtyPer),
		findNumber,
		*effectivity,
		0,
	)
	if err != nil {
		return entities.BOMLine{}, fmt.Errorf("invalid BOM line: %w", err)
	}
	return *bomLine, nil
}

func parseBOMLineWithPriority(record []string, hasPriority bool) (entities.BOMLine, error) {
	parentPN := entities.PartNumber(record[0])
	childPN := entities.PartNumber(record[1])

	qtyPer, err := strconv.ParseInt(record[2], 10, 64)
	if err != nil {
		return entities.BOMLine{}, fmt.Errorf("invalid qty_per: %s", record[2])
	}

	findNumber, err := strconv.Atoi(record[3])
	if err != nil {
		return entities.BOMLine{}, fmt.Errorf("invalid find_number: %s", record[3])
	}

	fromSerial := record[4]
	toSerial := record[5]

	effectivity, err := entities.NewSerialEffectivity(fromSerial, toSerial)
	if err != nil {
		return entities.BOMLine{}, fmt.Errorf("invalid serial effectivity: %w", err)
	}

	// Default priority is 0 (standard/primary)
	priority := 0
	if hasPriority && len(record) > 6 {
		priority, err = strconv.Atoi(record[6])
		if err != nil {
			return entities.BOMLine{}, fmt.Errorf("invalid priority: %s", record[6])
		}
	}

	bomLine, err := entities.NewBOMLine(
		parentPN,
		childPN,
		entities.Quantity(qtyPer),
		findNumber,
		*effectivity,
		priority,
	)
	if err != nil {
		return entities.BOMLine{}, fmt.Errorf("invalid BOM line: %w", err)
	}
	return *bomLine, nil
}

func parseDemand(record []string) (entities.DemandRequirement, error) {
	partNumber := entities.PartNumber(record[0])

	quantity, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return entities.DemandRequirement{}, fmt.Errorf("invalid quantity: %s", record[1])
	}

	needDate, err := time.Parse("2006-01-02", record[2])
	if err != nil {
		return entities.DemandRequirement{}, fmt.Errorf(
			"invalid need_date format: %s (expected YYYY-MM-DD)",
			record[2],
		)
	}

	demandSource := record[3]
	location := record[4]
	targetSerial := record[5]

	return entities.DemandRequirement{
		PartNumber:   partNumber,
		Quantity:     entities.Quantity(quantity),
		NeedDate:     needDate,
		DemandSource: demandSource,
		Location:     location,
		TargetSerial: targetSerial,
	}, nil
}

func parseLotSizeRule(s string) (entities.LotSizeRule, error) {
	switch strings.ToLower(s) {
	case "lotforlot":
		return entities.LotForLot, nil
	case "minimumqty":
		return entities.MinimumQty, nil
	case "standardpack":
		return entities.StandardPack, nil
	default:
		return entities.LotForLot, fmt.Errorf(
			"invalid lot_size_rule: %s (expected: LotForLot, MinimumQty, or StandardPack)",
			s,
		)
	}
}

func parseInventoryStatus(s string) (entities.InventoryStatus, error) {
	switch strings.ToLower(s) {
	case "available":
		return entities.Available, nil
	case "allocated":
		return entities.Allocated, nil
	case "quarantine":
		return entities.Quarantine, nil
	default:
		return entities.Available, fmt.Errorf(
			"invalid status: %s (expected: Available, Allocated, or Quarantine)",
			s,
		)
	}
}

func parseMakeBuyCode(s string) (entities.MakeBuyCode, error) {
	switch strings.ToLower(s) {
	case "make":
		return entities.MakeBuyMake, nil
	case "buy":
		return entities.MakeBuyBuy, nil
	default:
		return entities.MakeBuyMake, fmt.Errorf(
			"invalid make_buy_code: %s (expected: Make or Buy)",
			s,
		)
	}
}
