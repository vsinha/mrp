# MRP Engine CLI Usage Guide

## Overview

The MRP Engine CLI provides a command-line interface for running Material Requirements Planning analysis on aerospace manufacturing BOMs using CSV input files.

## Installation

```bash
# Build the CLI
go build -o mrp cmd/mrp/*.go

# Run from any directory
./mrp -help
```

## Quick Start

```bash
# Run a basic aerospace scenario
./mrp -scenario examples/aerospace_basic -verbose

# Generate JSON output
./mrp -scenario examples/complex_vehicle -format json -output results/

# Use optimized engine for large BOMs
./mrp -scenario examples/complex_vehicle -optimize -verbose
```

## Command Line Options

| Option | Description | Example |
|--------|-------------|---------|
| `-scenario <dir>` | Use scenario directory with CSV files | `-scenario examples/aerospace_basic` |
| `-bom <file>` | Path to BOM CSV file | `-bom data/bom.csv` |
| `-items <file>` | Path to items CSV file | `-items data/items.csv` |
| `-inventory <file>` | Path to inventory CSV file | `-inventory data/inventory.csv` |
| `-demands <file>` | Path to demands CSV file | `-demands data/demands.csv` |
| `-output <dir>` | Output directory for results | `-output results/` |
| `-format <fmt>` | Output format: text, json, csv | `-format json` |
| `-verbose` | Enable detailed progress output | `-verbose` |
| `-help` | Show help message | `-help` |

## Input CSV Formats

### 1. `items.csv` - Item Master Data
Defines parts with their planning parameters.

```csv
part_number,description,lead_time_days,lot_size_rule,min_order_qty,safety_stock,unit_of_measure
SATURN_V,Saturn V Launch Vehicle,180,LotForLot,1,0,EA
F1_ENGINE,F-1 Engine,120,LotForLot,1,2,EA
VALVE_MAIN,Main Engine Valve,30,MinimumQty,10,5,EA
```

**Fields:**
- `part_number`: Unique part identifier
- `description`: Human-readable description
- `lead_time_days`: Manufacturing/procurement lead time
- `lot_size_rule`: LotForLot, MinimumQty, or StandardPack
- `min_order_qty`: Minimum order quantity
- `safety_stock`: Safety stock quantity
- `unit_of_measure`: Unit (EA, KG, M, etc.)

### 2. `bom.csv` - Bill of Materials
Defines parent-child relationships with serial effectivity.

```csv
parent_pn,child_pn,qty_per,find_number,from_serial,to_serial
SATURN_V,F1_ENGINE,5,100,AS501,
SATURN_V,J2_ENGINE_V1,6,200,AS501,AS506
SATURN_V,J2_ENGINE_V2,6,200,AS507,
F1_ENGINE,F1_TURBOPUMP_V1,1,100,AS501,AS505
F1_ENGINE,F1_TURBOPUMP_V2,1,100,AS506,
```

**Fields:**
- `parent_pn`: Parent part number
- `child_pn`: Child part number
- `qty_per`: Quantity of child per parent
- `find_number`: Reference designator/position
- `from_serial`: Starting serial for effectivity
- `to_serial`: Ending serial (empty = open-ended)

### 3. `inventory.csv` - Available Inventory
Defines available inventory by location.

```csv
part_number,type,identifier,location,quantity,receipt_date,status
MERLIN_1D,serial,E1001,HAWTHORNE,1,2025-03-01,Available
VALVE_MAIN,lot,VALVE_LOT_001,HAWTHORNE,50,2025-01-20,Available
```

**Fields:**
- `part_number`: Part number
- `type`: `serial` or `lot`
- `identifier`: Serial number or lot number
- `location`: Storage location
- `quantity`: Available quantity (always 1 for serial)
- `receipt_date`: Date received (YYYY-MM-DD)
- `status`: Available, Allocated, or Quarantine

### 4. `demands.csv` - Demand Requirements
Defines what needs to be produced and when.

```csv
part_number,quantity,need_date,demand_source,location,target_serial
SATURN_V,1,1969-07-16,APOLLO_11,KENNEDY,AS506
F1_ENGINE,5,1969-06-01,REFURB_AS502,STENNIS,AS502
```

**Fields:**
- `part_number`: Part number needed
- `quantity`: Quantity required
- `need_date`: Date needed (YYYY-MM-DD)
- `demand_source`: Source/reason for demand
- `location`: Required location
- `target_serial`: Target serial for BOM effectivity

## Output Formats

### Text Format (Default)
Human-readable formatted output with sections for:
- Summary statistics
- Planned orders (sorted by due date)
- Inventory allocations (with FIFO details)
- Material shortages
- Cache statistics (if verbose)

```bash
./mrp -scenario examples/aerospace_basic -verbose
```

### JSON Format
Structured JSON output suitable for integration:

```bash
./mrp -scenario examples/aerospace_basic -format json -output results/
# Creates: results/mrp_results.json
```

JSON structure:
```json
{
  "metadata": {
    "explosion_time": "73.208µs",
    "generated_at": "2025-06-01T07:07:51-07:00",
    "input_files": {...}
  },
  "summary": {
    "planned_orders_count": 8,
    "allocations_count": 12,
    "shortages_count": 8,
    "cache_entries_count": 13
  },
  "planned_orders": [...],
  "allocations": [...],
  "shortages": [...]
}
```

### CSV Format
Separate CSV files for each output type:

```bash
./mrp -scenario examples/refurbishment_scenario -format csv -output results/
# Creates: 
#   results/planned_orders.csv
#   results/allocations.csv  
#   results/shortages.csv
```

## Example Scenarios

### Apollo Saturn V
Simple Saturn V scenario demonstrating serial effectivity:
```bash
./mrp -scenario examples/apollo_saturn_v -verbose
```
- **Features**: Serial effectivity, mixed inventory, basic BOM
- **Parts**: 15 items, 25 BOM lines
- **Performance**: ~70µs explosion time

### Apollo CSM
Educational scenario with simpler structure:
```bash
./mrp -scenario examples/apollo_csm -verbose
```
- **Features**: Multi-unit demand, 3-level BOM
- **Parts**: 15 items, 18 BOM lines
- **Use Case**: Learning and testing

### Refurbishment Scenario
Engine refurbishment with multi-serial demands:
```bash
./mrp -scenario examples/refurbishment_scenario -format csv -output results/
```
- **Features**: Multi-serial demands, consumable parts
- **Parts**: 15 items, 30 BOM lines
- **Use Case**: Refurb operations planning

### Apollo Saturn V Stack
Large-scale complete Saturn V scenario:
```bash
./mrp -scenario examples/apollo_saturn_v_stack -format json -output results/
```
- **Features**: Large BOMs, high quantities, optimization
- **Parts**: 40+ items, 85+ BOM lines
- **Use Case**: Performance testing, large-scale planning

## Performance Optimization

### Automatic Optimization
The CLI automatically uses optimized algorithms for all datasets:
- Uses compact storage repositories
- Always uses optimized engine
- Memory management and GC tuning for all operations

### High Performance
```bash
# All runs are optimized by default
./mrp -scenario examples/apollo_saturn_v_stack -verbose

# Monitor performance with verbose output
./mrp -scenario examples/aerospace_basic -verbose
```

## Integration Examples

### Batch Processing
```bash
#!/bin/bash
for scenario in examples/*/; do
    echo "Processing $scenario..."
    ./mrp -scenario "$scenario" -format json -output "results/$(basename $scenario)"
done
```

### CI/CD Pipeline
```yaml
- name: Run MRP Analysis
  run: |
    ./mrp -scenario scenarios/production -format json -output artifacts/
    # Upload artifacts/mrp_results.json to analysis system
```

### Data Processing
```bash
# Generate CSV for further analysis
./mrp -scenario examples/complex_vehicle -format csv -output analysis/
# Process with tools like pandas, R, Excel, etc.
```

## Error Handling

The CLI provides detailed error messages for common issues:

### File Errors
```bash
Error: BOM file not found: missing_bom.csv
Error: Items CSV header mismatch. Expected: [part_number,description,...], Got: [...]
```

### Data Validation
```bash
Error: Invalid quantity in row 5: not-a-number
Error: Invalid need_date format: 2025-13-45 (expected YYYY-MM-DD)
Error: Invalid lot_size_rule: InvalidRule (expected: LotForLot, MinimumQty, or StandardPack)
```

### MRP Logic
```bash
Error: Circular BOM reference detected: PART_A -> PART_B -> PART_A
Error: Item not found: UNKNOWN_PART
```

## Tips and Best Practices

### File Organization
```
project/
├── scenarios/
│   ├── production/
│   │   ├── items.csv
│   │   ├── bom.csv
│   │   ├── inventory.csv
│   │   └── demands.csv
│   └── test/
└── results/
```

### Serial Effectivity
- Use consistent serial number formats (e.g., SN001, SN002, ...)
- Ensure no overlapping effectivity ranges for same parent/child
- Leave `to_serial` empty for open-ended ranges

### Inventory Management
- Use `serial` type for unique, trackable items (engines, major assemblies)
- Use `lot` type for bulk items (fasteners, fluids, consumables)
- Maintain FIFO with consistent receipt dates

### Performance
- Optimized performance for all BOM sizes
- Consider breaking very large scenarios into smaller runs
- Use `-verbose` to monitor performance and cache effectiveness

## Troubleshooting

### Common Issues

**Slow Performance**
```bash
# Solution: Use optimization
./mrp -scenario large_bom -verbose
```

**Memory Issues**
```bash
# Solution: Process in smaller batches or use optimization
# Check available memory before running large scenarios
```

**Incorrect Results**
```bash
# Solution: Validate input data
# Check serial effectivity ranges
# Verify BOM parent-child relationships
# Ensure inventory locations match demand locations
```

**File Format Errors**
```bash
# Solution: Validate CSV headers and data types
# Use example scenarios as templates
# Check for special characters in part numbers
```