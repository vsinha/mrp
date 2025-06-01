# MRP Engine Example Scenarios

This directory contains example scenarios demonstrating different aspects of the MRP engine for aerospace manufacturing using Apollo program data.

## Scenario Overview

### 1. `apollo_saturn_v/`
**Basic Saturn V mission scenario**
- Single vehicle demand for Apollo 11 mission
- Serial effectivity between engine variants (F-1 turbopump versions)
- Mixed inventory types (serial engines, lot fasteners)
- Demonstrates: Basic BOM explosion, serial effectivity, inventory allocation

### 2. `apollo_csm/`
**Apollo Command/Service Module scenario**
- Simpler vehicle with Command and Service modules
- Multiple unit demand for Apollo missions
- Good for learning and testing
- Demonstrates: Multi-unit explosion, basic inventory management

### 3. `apollo_engine_refurb/`
**Apollo engine refurbishment operations**
- Multiple demands for F-1 and J-2 engine refurb and spares
- Different target serials for Saturn V missions (SA509+)
- Heavy use of consumable parts (seals, gaskets, bolts)
- Demonstrates: Multi-serial demands, consumable planning, refurb logistics

### 4. `apollo_saturn_v_stack/`
**Complete Saturn V with Apollo spacecraft**
- Full Saturn V stack including CSM and LM
- Complex multi-level BOMs (S-IC, S-II, S-IVB stages)
- Complete Apollo 11 mission configuration
- Demonstrates: Large BOM handling, complex structures, full mission planning

## CSV File Format

Each scenario contains four CSV files:

### `items.csv` - Item Master Data
```csv
part_number,description,lead_time_days,lot_size_rule,min_order_qty,safety_stock,unit_of_measure
F1_ENGINE,F-1 Engine,180,LotForLot,1,2,EA
```

### `bom.csv` - Bill of Materials
```csv
parent_pn,child_pn,qty_per,find_number,from_serial,to_serial
F1_ENGINE,F1_TURBOPUMP_V1,1,100,SA501,SA507
F1_ENGINE,F1_TURBOPUMP_V2,1,100,SA508,
```

### `inventory.csv` - Available Inventory
```csv
part_number,type,identifier,location,quantity,receipt_date,status
F1_ENGINE,serial,F1_001,MICHOUD,1,1968-09-15,Available
SEAL_KIT,lot,SEAL_LOT_001,KENNEDY,150,1968-04-01,Available
```

### `demands.csv` - Demand Requirements
```csv
part_number,quantity,need_date,demand_source,location,target_serial
SATURN_V_FIRST_STAGE,1,1969-07-04,APOLLO_11,KENNEDY,SA506
```

## Running Examples

### Basic Usage
```bash
# Run Saturn V scenario
mrp -scenario examples/apollo_saturn_v -verbose

# Run with JSON output
mrp -scenario examples/apollo_saturn_v_stack -format json -output results/

# Use optimized engine for large scenarios
mrp -scenario examples/apollo_saturn_v_stack -optimize -verbose
```

### Individual Files
```bash
mrp -bom examples/apollo_saturn_v/bom.csv \
    -items examples/apollo_saturn_v/items.csv \
    -inventory examples/apollo_saturn_v/inventory.csv \
    -demands examples/apollo_saturn_v/demands.csv \
    -verbose
```

### CSV Output
```bash
mrp -scenario examples/apollo_engine_refurb -format csv -output results/
# Creates: results/planned_orders.csv, results/allocations.csv, results/shortages.csv
```

## Expected Results

### Apollo Saturn V
- **Planned Orders**: ~20-25 orders for F-1 engines, J-2 engines, tanks, structures
- **Serial Effectivity**: Uses F1_TURBOPUMP_V1 for SA506 (early Saturn V)
- **Inventory Usage**: Allocates available engines and components from Michoud, Canoga Park

### Apollo CSM
- **Multi-Unit**: Plans for 2 Command/Service Modules with shared components
- **Simple Structure**: 3-level BOM explosion for CSM systems
- **Good Performance**: Fast execution for learning Apollo systems

### Apollo Engine Refurbishment
- **Multi-Serial**: Different BOMs for SA501-SA507 vs SA508+ configurations
- **Consumables**: High quantities of seals, gaskets, bolts for engine overhaul
- **Mixed Demands**: F-1 and J-2 engine refurb for later missions

### Apollo Saturn V Stack
- **Large Scale**: 100+ planned orders for complete vehicle
- **Complex Structure**: Full Saturn V with all three stages plus Apollo spacecraft
- **Deep BOMs**: 5-level explosion from vehicle to fasteners
- **Performance Test**: Good benchmark for optimization with realistic Apollo data

## Key Learning Points

1. **Serial Effectivity**: See how different Saturn V serials (SA501-SA507 vs SA508+) resolve to different BOMs
2. **Inventory Allocation**: FIFO allocation across NASA facilities (Michoud, Canoga Park, Kennedy)
3. **Multi-Unit Planning**: How quantities multiply through BOM levels for Apollo missions
4. **Lead Time Offsetting**: Planned order timing based on Apollo program schedules
5. **Mixed Inventory**: Serialized engines vs lot-controlled consumables
6. **Demand Traceability**: Full trace from Apollo mission to individual components

## Historical Context

These scenarios are based on the actual Apollo program:
- **Manufacturing Locations**: Michoud (S-IC), Canoga Park (J-2), Downey (CSM), Grumman (LM)
- **Serial Numbers**: SA501-SA517 for Saturn V vehicles, CSM001-CSM119 for spacecraft
- **Mission Timeline**: Apollo 11 launch date (July 16, 1969) and preparation schedules
- **Engine Variants**: F-1 turbopump improvements between early and late Saturn V missions