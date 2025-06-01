package output

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/vsinha/mrp/pkg/application/dto"
	"github.com/vsinha/mrp/pkg/domain/entities"
)

// GanttChart represents a Gantt chart for MRP results
type GanttChart struct {
	Width        int
	Height       int
	MarginLeft   int
	MarginTop    int
	MarginRight  int
	MarginBottom int
	RowHeight    int
	TimeScale    time.Duration
	StartTime    time.Time
	EndTime      time.Time
}

// GanttBar represents a single bar in the Gantt chart
type GanttBar struct {
	PartNumber entities.PartNumber
	OrderType  entities.OrderType
	Quantity   entities.Quantity
	StartDate  time.Time
	DueDate    time.Time
	X          int
	Y          int
	Width      int
	Color      string
	Split      int // Which split order this is (0 for non-split orders)
}

// NewGanttChart creates a new Gantt chart for MRP results
func NewGanttChart(result *dto.MRPResult) *GanttChart {
	if len(result.PlannedOrders) == 0 {
		return &GanttChart{
			Width:        800,
			Height:       200,
			MarginLeft:   150,
			MarginTop:    50,
			MarginRight:  50,
			MarginBottom: 50,
			RowHeight:    25,
		}
	}

	// Find time bounds
	startTime := result.PlannedOrders[0].StartDate
	endTime := result.PlannedOrders[0].DueDate

	for _, order := range result.PlannedOrders {
		if order.StartDate.Before(startTime) {
			startTime = order.StartDate
		}
		if order.DueDate.After(endTime) {
			endTime = order.DueDate
		}
	}

	// Add some padding to the time range
	totalDuration := endTime.Sub(startTime)
	padding := time.Duration(float64(totalDuration) * 0.1) // 10% padding
	startTime = startTime.Add(-padding)
	endTime = endTime.Add(padding)

	// Calculate dimensions
	uniqueParts := make(map[entities.PartNumber]int)
	for _, order := range result.PlannedOrders {
		uniqueParts[order.PartNumber]++
	}

	rowHeight := 30
	height := len(uniqueParts)*rowHeight + 100 // Extra space for headers and margins

	return &GanttChart{
		Width:        1200,
		Height:       height,
		MarginLeft:   200,
		MarginTop:    60,
		MarginRight:  100,
		MarginBottom: 80,
		RowHeight:    rowHeight,
		StartTime:    startTime,
		EndTime:      endTime,
	}
}

// GenerateSVG creates an SVG representation of the Gantt chart
func (gc *GanttChart) GenerateSVG(result *dto.MRPResult) string {
	if len(result.PlannedOrders) == 0 {
		return gc.generateEmptyChart()
	}

	var svg strings.Builder

	// SVG header
	svg.WriteString(fmt.Sprintf(`<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">`, gc.Width, gc.Height))
	svg.WriteString(`<defs>`)
	svg.WriteString(`<style>`)
	svg.WriteString(`.part-label { font-family: Arial, sans-serif; font-size: 12px; fill: #333; }`)
	svg.WriteString(`.time-label { font-family: Arial, sans-serif; font-size: 10px; fill: #666; }`)
	svg.WriteString(`.title { font-family: Arial, sans-serif; font-size: 16px; font-weight: bold; fill: #333; }`)
	svg.WriteString(`.grid-line { stroke: #e0e0e0; stroke-width: 1; }`)
	svg.WriteString(`.order-bar { stroke: #333; stroke-width: 1; }`)
	svg.WriteString(`.order-text { font-family: Arial, sans-serif; font-size: 9px; fill: white; }`)
	svg.WriteString(`</style>`)
	svg.WriteString(`</defs>`)

	// Background
	svg.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, gc.Width, gc.Height))

	// Title
	svg.WriteString(fmt.Sprintf(`<text x="%d" y="30" class="title">MRP Production Schedule - Forward Scheduling</text>`, gc.Width/2))

	// Create bars and organize by part
	bars := gc.createBars(result.PlannedOrders)
	partRows := gc.organizeBars(bars)

	// Draw time grid and axis
	gc.drawTimeAxis(&svg)
	gc.drawTimeGrid(&svg, len(partRows))

	// Draw part labels and bars
	gc.drawPartRows(&svg, partRows)

	// Legend
	gc.drawLegend(&svg)

	svg.WriteString(`</svg>`)
	return svg.String()
}

// createBars converts planned orders to Gantt bars
func (gc *GanttChart) createBars(orders []entities.PlannedOrder) []GanttBar {
	var bars []GanttBar
	chartWidth := gc.Width - gc.MarginLeft - gc.MarginRight
	totalDuration := gc.EndTime.Sub(gc.StartTime)

	for _, order := range orders {
		// Determine split number from demand trace
		split := 0
		if strings.Contains(order.DemandTrace, "Split") {
			fmt.Sscanf(order.DemandTrace, "%*s (Split %d)", &split)
		}

		// Calculate position and size
		startOffset := order.StartDate.Sub(gc.StartTime)
		duration := order.DueDate.Sub(order.StartDate)

		x := gc.MarginLeft + int(float64(startOffset)/float64(totalDuration)*float64(chartWidth))
		width := int(float64(duration) / float64(totalDuration) * float64(chartWidth))
		if width < 2 {
			width = 2 // Minimum width for visibility
		}

		// Color based on order type and split
		color := gc.getBarColor(order.OrderType, split)

		bar := GanttBar{
			PartNumber: order.PartNumber,
			OrderType:  order.OrderType,
			Quantity:   order.Quantity,
			StartDate:  order.StartDate,
			DueDate:    order.DueDate,
			X:          x,
			Width:      width,
			Color:      color,
			Split:      split,
		}

		bars = append(bars, bar)
	}

	return bars
}

// organizeBars groups bars by part number and sorts them
func (gc *GanttChart) organizeBars(bars []GanttBar) map[entities.PartNumber][]GanttBar {
	partRows := make(map[entities.PartNumber][]GanttBar)

	// Group by part number
	for _, bar := range bars {
		partRows[bar.PartNumber] = append(partRows[bar.PartNumber], bar)
	}

	// Sort bars within each part by start date
	for partNumber := range partRows {
		sort.Slice(partRows[partNumber], func(i, j int) bool {
			return partRows[partNumber][i].StartDate.Before(partRows[partNumber][j].StartDate)
		})
	}

	return partRows
}

// drawTimeAxis draws the time axis labels
func (gc *GanttChart) drawTimeAxis(svg *strings.Builder) {
	chartWidth := gc.Width - gc.MarginLeft - gc.MarginRight
	totalDuration := gc.EndTime.Sub(gc.StartTime)

	// Calculate appropriate time intervals
	days := int(math.Ceil(totalDuration.Hours() / 24))
	var interval time.Duration
	var labelFormat string

	if days <= 30 {
		interval = 24 * time.Hour // Daily
		labelFormat = "Jan 2"
	} else if days <= 180 {
		interval = 7 * 24 * time.Hour // Weekly
		labelFormat = "Jan 2"
	} else {
		interval = 30 * 24 * time.Hour // Monthly
		labelFormat = "Jan 2006"
	}

	// Draw time labels
	for t := gc.StartTime.Truncate(interval); t.Before(gc.EndTime); t = t.Add(interval) {
		offset := t.Sub(gc.StartTime)
		x := gc.MarginLeft + int(float64(offset)/float64(totalDuration)*float64(chartWidth))

		if x >= gc.MarginLeft && x <= gc.Width-gc.MarginRight {
			svg.WriteString(fmt.Sprintf(`<text x="%d" y="%d" class="time-label" text-anchor="middle">%s</text>`,
				x, gc.Height-gc.MarginBottom+15, t.Format(labelFormat)))
		}
	}

	// Time axis line
	svg.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" class="grid-line"/>`,
		gc.MarginLeft, gc.Height-gc.MarginBottom, gc.Width-gc.MarginRight, gc.Height-gc.MarginBottom))
}

// FIXED: drawTimeGrid to use consistent row calculation with proper bounds
func (gc *GanttChart) drawTimeGrid(svg *strings.Builder, numRows int) {
	chartWidth := gc.Width - gc.MarginLeft - gc.MarginRight
	totalDuration := gc.EndTime.Sub(gc.StartTime)
	gridTop := gc.MarginTop

	// FIXED: Calculate proper grid bottom to match drawPartRows logic
	maxRowY := gc.Height - gc.MarginBottom - 30 // Leave space above time axis
	availableHeight := maxRowY - gc.MarginTop
	adjustedRowHeight := availableHeight / numRows
	if adjustedRowHeight > gc.RowHeight {
		adjustedRowHeight = gc.RowHeight
	}
	gridBottom := gc.MarginTop + numRows*adjustedRowHeight

	// Calculate grid interval
	days := int(math.Ceil(totalDuration.Hours() / 24))
	var interval time.Duration

	if days <= 30 {
		interval = 24 * time.Hour // Daily
	} else if days <= 180 {
		interval = 7 * 24 * time.Hour // Weekly
	} else {
		interval = 30 * 24 * time.Hour // Monthly
	}

	// Draw vertical grid lines
	for t := gc.StartTime.Truncate(interval); t.Before(gc.EndTime); t = t.Add(interval) {
		offset := t.Sub(gc.StartTime)
		x := gc.MarginLeft + int(float64(offset)/float64(totalDuration)*float64(chartWidth))

		if x >= gc.MarginLeft && x <= gc.Width-gc.MarginRight {
			svg.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" class="grid-line"/>`,
				x, gridTop, x, gridBottom))
		}
	}
}

// FIXED: drawPartRows with proper spacing calculation to avoid time axis overlap
func (gc *GanttChart) drawPartRows(svg *strings.Builder, partRows map[entities.PartNumber][]GanttBar) {
	// Sort parts by earliest start time for better visualization
	var parts []entities.PartNumber
	for part := range partRows {
		parts = append(parts, part)
	}
	sort.Slice(parts, func(i, j int) bool {
		// Find earliest start time for each part
		iEarliest := partRows[parts[i]][0].StartDate
		jEarliest := partRows[parts[j]][0].StartDate

		for _, bar := range partRows[parts[i]] {
			if bar.StartDate.Before(iEarliest) {
				iEarliest = bar.StartDate
			}
		}
		for _, bar := range partRows[parts[j]] {
			if bar.StartDate.Before(jEarliest) {
				jEarliest = bar.StartDate
			}
		}

		return iEarliest.Before(jEarliest)
	})

	// FIXED: Calculate proper chart bounds to avoid time axis overlap
	maxRowY := gc.Height - gc.MarginBottom - 30 // Leave 30px buffer above time axis
	availableHeight := maxRowY - gc.MarginTop
	adjustedRowHeight := availableHeight / len(parts)
	if adjustedRowHeight > gc.RowHeight {
		adjustedRowHeight = gc.RowHeight // Don't make rows unnecessarily tall
	}

	// Draw each part row
	for i, part := range parts {
		y := gc.MarginTop + i*adjustedRowHeight // Use adjusted height
		bars := partRows[part]

		// Part label - positioned further left to avoid overlap
		svg.WriteString(fmt.Sprintf(`<text x="%d" y="%d" class="part-label" text-anchor="end">%s</text>`,
			gc.MarginLeft-15, y+adjustedRowHeight/2+4, string(part)))

		// Horizontal row line - FIXED: position properly with adjusted height
		svg.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" class="grid-line"/>`,
			gc.MarginLeft, y+adjustedRowHeight, gc.Width-gc.MarginRight, y+adjustedRowHeight))

		// Draw bars for this part
		for _, bar := range bars {
			gc.drawBar(svg, bar, y, adjustedRowHeight)
		}
	}
}

// FIXED: drawBar with proper height parameter to use adjusted row height
func (gc *GanttChart) drawBar(svg *strings.Builder, bar GanttBar, rowY int, rowHeight int) {
	barHeight := rowHeight - 4
	barY := rowY + 2

	// Draw bar rectangle
	svg.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="%s" class="order-bar"/>`,
		bar.X, barY, bar.Width, barHeight, bar.Color))

	// Add text if bar is wide enough
	if bar.Width > 40 {
		textX := bar.X + bar.Width/2
		textY := barY + barHeight/2 + 3

		// Show quantity and split info
		text := fmt.Sprintf("Qty: %d", bar.Quantity)
		if bar.Split > 0 {
			text = fmt.Sprintf("Split %d: %d", bar.Split, bar.Quantity)
		}

		svg.WriteString(fmt.Sprintf(`<text x="%d" y="%d" class="order-text" text-anchor="middle">%s</text>`,
			textX, textY, text))
	}

	// Tooltip (SVG title element)
	tooltipText := fmt.Sprintf("Part: %s, Qty: %d, Start: %s, Due: %s, Type: %s",
		bar.PartNumber, bar.Quantity,
		bar.StartDate.Format("2006-01-02"),
		bar.DueDate.Format("2006-01-02"),
		bar.OrderType)

	svg.WriteString(fmt.Sprintf(`<title>%s</title>`, tooltipText))
}

// drawLegend draws a legend explaining the colors
func (gc *GanttChart) drawLegend(svg *strings.Builder) {
	legendX := gc.Width - gc.MarginRight - 200
	legendY := 50

	// Legend background
	svg.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="180" height="60" fill="white" stroke="#ccc" stroke-width="1"/>`,
		legendX, legendY))

	// Legend title
	svg.WriteString(fmt.Sprintf(`<text x="%d" y="%d" class="part-label" font-weight="bold">Legend</text>`,
		legendX+10, legendY+15))

	// Legend items
	items := []struct {
		color string
		label string
	}{
		{"#4CAF50", "Make Orders"},
		{"#2196F3", "Buy Orders"},
		{"#FF9800", "Split Orders"},
	}

	for i, item := range items {
		itemY := legendY + 25 + i*12
		svg.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="12" height="8" fill="%s"/>`,
			legendX+10, itemY, item.color))
		svg.WriteString(fmt.Sprintf(`<text x="%d" y="%d" class="time-label">%s</text>`,
			legendX+30, itemY+6, item.label))
	}
}

// getBarColor returns color based on order type and split status
func (gc *GanttChart) getBarColor(orderType entities.OrderType, split int) string {
	if split > 0 {
		return "#FF9800" // Orange for split orders
	}
	switch orderType {
	case entities.Make:
		return "#4CAF50" // Green for make orders
	case entities.Buy:
		return "#2196F3" // Blue for buy orders
	default:
		return "#9E9E9E" // Gray for unknown
	}
}

// generateEmptyChart creates an empty chart when no orders exist
func (gc *GanttChart) generateEmptyChart() string {
	return fmt.Sprintf(`<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">
		<rect width="%d" height="%d" fill="white"/>
		<text x="%d" y="%d" class="title" text-anchor="middle">No Production Orders Found</text>
		<style>
			.title { font-family: Arial, sans-serif; font-size: 16px; fill: #666; }
		</style>
	</svg>`, gc.Width, gc.Height, gc.Width, gc.Height, gc.Width/2, gc.Height/2)
}
