# MRP Planning System

A high-performance Material Requirements Planning (MRP) engine designed for complex manufacturing scenarios with support for multi-level BOMs, serial effectivity, and critical path analysis.

## Overview

This MRP system provides enterprise-grade planning capabilities including:

- **Multi-level BOM explosion** with shared components and alternates
- **Serial effectivity** for configuration management
- **Critical path analysis** for production scheduling
- **Inventory allocation** with FIFO and serial tracking
- **Performance optimization** for large-scale manufacturing
- **Scenario generation** for testing and validation

## System Architecture

### Core Components

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI Interface                        │
├─────────────────────────────────────────────────────────────┤
│                    Application Layer                        │
│  ┌─────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │     MRP     │  │ Critical Path   │  │ Orchestration   │  │
│  │   Service   │  │    Service      │  │    Service      │  │
│  └─────────────┘  └─────────────────┘  └─────────────────┘  │
│           │              │                      │            │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │                Shared Services                          │ │
│  │  • BOM Traverser • Allocation Context • Alternates     │ │
│  └─────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                      Domain Layer                           │
│  ┌─────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │  Entities   │  │  Repositories   │  │  Domain         │  │
│  │ (BOMs, Items│  │  (Interfaces)   │  │  Services       │  │
│  │  Orders)    │  │                 │  │                 │  │
│  └─────────────┘  └─────────────────┘  └─────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                 Infrastructure Layer                        │
│  ┌─────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │  Memory     │  │      CSV        │  │    Testing      │  │
│  │ Repositories│  │    Loaders      │  │   Helpers       │  │
│  └─────────────┘  └─────────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Key Features

- **Clean Architecture**: Domain-driven design with clear separation of concerns
- **Functional Approach**: Explicit dependency injection and immutable operations
- **Performance Optimized**: Efficient algorithms and memory management for large BOMs
- **Extensible**: Plugin architecture for custom business logic
- **Testable**: Comprehensive test coverage with realistic scenarios

## Getting Started

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd mrp

# Build the application
go build -o ./bin/mrp cmd/mrp/*.go

# Verify installation
./bin/mrp help
```

### Quick Start

#### 1. Run an Existing Scenario

```bash
# Run a basic Apollo Saturn V scenario
./bin/mrp run --scenario ./examples/apollo_saturn_v --verbose

# Run with critical path analysis
./bin/mrp run --scenario ./examples/apollo_saturn_v --critical-path --top-paths 5

# Generate JSON output
./bin/mrp run --scenario ./examples/constellation_program --format json --output ./results
```

#### 2. Generate Test Scenarios

```bash
# Generate a small test scenario
./bin/mrp generate --items 100 --max-depth 5 --demands 10 --inventory 0.5 --output ./test_scenario

# Generate a large performance test scenario  
./bin/mrp generate --items 30000 --max-depth 8 --demands 50 --inventory 1.2 --output ./large_scenario --verbose

# Generate a reproducible scenario
./bin/mrp generate --items 1000 --max-depth 6 --demands 20 --inventory 0.8 --output ./repro_scenario --seed 12345
```

#### 3. Run Analysis on Generated Scenario

```bash
# Run MRP analysis on generated scenario
./bin/mrp run --scenario ./test_scenario --verbose

# Include critical path analysis
./bin/mrp run --scenario ./test_scenario --critical-path --top-paths 3
```

## Commands

### `mrp run` - Execute MRP Analysis

Run MRP planning on existing scenarios.

**Options:**
- `--scenario <dir>`: Path to scenario directory containing CSV files
- `--bom <file>`: Path to BOM CSV file (alternative to scenario)
- `--items <file>`: Path to items CSV file
- `--inventory <file>`: Path to inventory CSV file  
- `--demands <file>`: Path to demands CSV file
- `--output <dir>`: Output directory for results
- `--format <fmt>`: Output format (text, json, csv)
- `--critical-path`: Perform critical path analysis
- `--top-paths <n>`: Number of top critical paths to analyze (default: 3)
- `--verbose`: Enable detailed output

**Examples:**
```bash
# Basic MRP run
./bin/mrp run --scenario ./examples/apollo_saturn_v

# With critical path analysis
./bin/mrp run --scenario ./examples/constellation_program --critical-path --top-paths 5 --verbose

# Custom file inputs
./bin/mrp run --bom data/bom.csv --items data/items.csv --inventory data/inventory.csv --demands data/demands.csv
```

### `mrp generate` - Create Test Scenarios

Generate realistic test scenarios for MRP analysis.

**Options:**
- `--items <n>`: Number of items to generate (required)
- `--max-depth <n>`: Maximum depth of BOM tree (required)  
- `--demands <n>`: Number of demand lines to generate (required)
- `--inventory <f>`: Inventory multiplier - 0.5 = half coverage, 4.0 = 4x coverage (required)
- `--output <dir>`: Output directory for generated files (required)
- `--seed <n>`: Random seed for reproducible generation
- `--verbose`: Enable detailed output

**Examples:**
```bash
# Small test scenario
./bin/mrp generate --items 100 --max-depth 5 --demands 10 --inventory 0.5 --output ./test_scenario

# Large enterprise scenario
./bin/mrp generate --items 30000 --max-depth 8 --demands 100 --inventory 0.8 --output ./enterprise_test --verbose

# Reproducible scenario for testing
./bin/mrp generate --items 1000 --max-depth 6 --demands 25 --inventory 1.0 --output ./consistent_test --seed 42
```

**Generated Scenario Features:**
- **Realistic BOM structures** with random branching and shared components
- **Serial effectivity** with proper configuration management
- **Varied lead times** based on BOM level (7-450 days)
- **Mixed lot sizing rules** (LotForLot, MinimumQty, StandardPack)
- **Multiple locations** and inventory types
- **Proportional inventory** with configurable coverage levels

## Input File Formats

### 1. `items.csv` - Item Master Data

```csv
part_number,description,lead_time_days,lot_size_rule,min_order_qty,safety_stock,unit_of_measure
SATURN_V,Saturn V Launch Vehicle,365,LotForLot,1,0,EA
F1_ENGINE,F-1 Engine,180,LotForLot,1,1,EA
VALVE_MAIN,Main Engine Valve,45,MinimumQty,10,5,EA
FASTENER_KIT,Fastener Kit,14,StandardPack,100,20,EA
```

### 2. `bom.csv` - Bill of Materials

```csv
parent_pn,child_pn,qty_per,find_number,from_serial,to_serial,priority
SATURN_V,F1_ENGINE,5,100,SN001,,0
SATURN_V,J2_ENGINE_V1,6,200,SN001,SN506,0
SATURN_V,J2_ENGINE_V2,6,200,SN507,,0
F1_ENGINE,F1_TURBOPUMP_V1,1,100,SN001,SN505,0
F1_ENGINE,F1_TURBOPUMP_V2,1,100,SN506,,1
```

### 3. `inventory.csv` - Available Inventory

```csv
part_number,type,identifier,location,quantity,receipt_date,status
F1_ENGINE,serial,F1_001,STENNIS,1,2024-01-15,Available
VALVE_MAIN,lot,VALVE_LOT_001,STENNIS,50,2024-01-10,Available
```

### 4. `demands.csv` - Demand Requirements

```csv
part_number,quantity,need_date,demand_source,location,target_serial
SATURN_V,1,2025-07-16,APOLLO_11,KENNEDY,SN506
F1_ENGINE,5,2025-06-01,REFURB_PROGRAM,STENNIS,SN502
```

## Example Scenarios

The system includes several pre-built scenarios:

### Apollo Saturn V (`./examples/apollo_saturn_v/`)
- **Purpose**: Basic MRP demonstration with serial effectivity
- **Scale**: 15 items, 25 BOM relationships
- **Features**: Multi-engine configuration, serial-specific BOMs

### Constellation Program (`./examples/constellation_program/`)  
- **Purpose**: Large-scale enterprise scenario
- **Scale**: 107 items, 241 BOM relationships, 22 demand lines
- **Features**: Multiple vehicle programs, complex supply chains, severe shortages

### Apollo CSM (`./examples/apollo_csm/`)
- **Purpose**: Educational and testing
- **Scale**: Smaller, focused scenario
- **Features**: Command/Service Module specific planning

## Performance

The system is optimized for enterprise-scale manufacturing:

- **Large BOMs**: Tested with 30,000+ items
- **Fast Execution**: Microsecond explosion times for typical scenarios  
- **Memory Efficient**: Optimized data structures and GC tuning
- **Scalable**: Linear performance scaling with BOM size

### Performance Benchmarks

| Scenario Size | Items | BOM Lines | Explosion Time | Memory Usage |
|---------------|-------|-----------|----------------|--------------|
| Small         | 50    | 100       | < 1ms          | < 10MB       |
| Medium        | 500   | 1,000     | < 5ms          | < 50MB       |
| Large         | 5,000 | 10,000    | < 50ms         | < 200MB      |
| Enterprise    | 30,000| 60,000    | < 500ms        | < 1GB        |

## Output Formats

### Text (Default)
Human-readable formatted output with planning summary, orders, allocations, and shortages.

### JSON
Structured output for integration with other systems:
```json
{
  "metadata": {
    "explosion_time": "7.293787ms",
    "generated_at": "2025-06-01T13:53:42Z"
  },
  "summary": {
    "planned_orders_count": 2513,
    "allocations_count": 283,
    "shortages_count": 0
  },
  "planned_orders": [...],
  "allocations": [...],
  "shortages": [...]
}
```

### CSV
Separate CSV files for each output type suitable for further analysis in Excel, pandas, etc.

## Advanced Features

### Critical Path Analysis
Identifies the longest lead time paths through complex BOMs:

```bash
./bin/mrp run --scenario ./examples/constellation_program --critical-path --top-paths 5
```

### Serial Effectivity
Supports configuration-specific BOMs based on serial number ranges:

```csv
parent_pn,child_pn,qty_per,find_number,from_serial,to_serial,priority
F1_ENGINE,F1_TURBOPUMP_V1,1,100,AS501,AS505,0
F1_ENGINE,F1_TURBOPUMP_V2,1,100,AS506,,0
```

### Shared Components
Handles parts used across multiple assemblies with proper allocation logic.

### Alternate Parts
Supports alternate components with priority-based selection.

## Integration

### Batch Processing
```bash
#!/bin/bash
for scenario in scenarios/*/; do
    ./bin/mrp run --scenario "$scenario" --format json --output "results/$(basename $scenario)"
done
```

### CI/CD Pipeline
```yaml
- name: Run MRP Analysis
  run: |
    ./bin/mrp run --scenario production_scenario --format json --output artifacts/
    ./bin/mrp run --scenario production_scenario --critical-path --output artifacts/
```

## Troubleshooting

### Common Issues

**File Format Errors**
- Ensure CSV headers match expected format exactly
- Check for special characters in part numbers
- Verify date formats (YYYY-MM-DD)

**Performance Issues**  
- Use `--verbose` to monitor execution time
- Consider breaking very large scenarios into batches
- Ensure adequate memory for large BOMs

**Logic Errors**
- Validate BOM parent-child relationships
- Check serial effectivity ranges for overlaps
- Verify inventory locations match demand locations

### Getting Help

```bash
# Command help
./bin/mrp help
./bin/mrp run --help  
./bin/mrp generate --help

# Verbose output for debugging
./bin/mrp run --scenario ./examples/apollo_saturn_v --verbose
```

## Contributing

This system follows clean architecture principles with comprehensive test coverage. Key development practices:

- **Domain-driven design** with clear boundaries
- **Functional approach** with explicit dependencies  
- **Performance testing** with realistic scenarios
- **Comprehensive validation** of business logic

For detailed architecture information, see the source code documentation in `/pkg/` directories.