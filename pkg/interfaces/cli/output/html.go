package output

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/vsinha/mrp/pkg/application/dto"
	"github.com/vsinha/mrp/pkg/domain/entities"
)

//go:embed templates/*.html
var templateFS embed.FS

// HTMLVisualization generates an interactive HTML D3.js visualization
type HTMLVisualization struct {
	Width  int
	Height int
}

// NetworkNode represents a node in the dependency network
type NetworkNode struct {
	ID         string              `json:"id"`
	PartNumber entities.PartNumber `json:"partNumber"`
	OrderType  string              `json:"orderType"`
	Quantity   entities.Quantity   `json:"quantity"`
	StartDate  time.Time           `json:"startDate"`
	DueDate    time.Time           `json:"dueDate"`
	Location   string              `json:"location"`
	Level      int                 `json:"level"` // BOM level for positioning
}

// NetworkLink represents a dependency relationship
type NetworkLink struct {
	Source   string            `json:"source"`
	Target   string            `json:"target"`
	QtyPer   entities.Quantity `json:"qtyPer"`
	FindNum  int               `json:"findNumber"`
	Priority int               `json:"priority"`
}

// TimelineBar represents a bar in the timeline chart
type TimelineBar struct {
	ID         string              `json:"id"`
	PartNumber entities.PartNumber `json:"partNumber"`
	OrderType  string              `json:"orderType"`
	Quantity   entities.Quantity   `json:"quantity"`
	StartDate  time.Time           `json:"startDate"`
	DueDate    time.Time           `json:"dueDate"`
	Location   string              `json:"location"`
	Split      int                 `json:"split"`
	Color      string              `json:"color"`
}

// VisualizationData contains all data needed for the HTML visualization
type VisualizationData struct {
	Nodes         []NetworkNode               `json:"nodes"`
	Links         []NetworkLink               `json:"links"`
	TimelineBars  []TimelineBar               `json:"timelineBars"`
	Allocations   []entities.AllocationResult `json:"allocations"`
	Shortages     []entities.Shortage         `json:"shortages"`
	ExplosionTime time.Duration               `json:"explosionTime"`
	StartDate     time.Time                   `json:"startDate"`
	EndDate       time.Time                   `json:"endDate"`
	Statistics    struct {
		TotalOrders    int `json:"totalOrders"`
		TotalParts     int `json:"totalParts"`
		TotalShortages int `json:"totalShortages"`
		MaxBOMLevel    int `json:"maxBOMLevel"`
	} `json:"statistics"`
}

// TemplateData contains all data for rendering the HTML template
type TemplateData struct {
	*VisualizationData
	DataJSON               template.JS
	ExplosionTimeFormatted string
	GeneratedAt            string
}

// NewHTMLVisualization creates a new HTML visualization generator
func NewHTMLVisualization() *HTMLVisualization {
	return &HTMLVisualization{
		Width:  1400,
		Height: 800,
	}
}

// GenerateHTML creates an interactive HTML D3.js visualization
func (hv *HTMLVisualization) GenerateHTML(result *dto.MRPResult, config Config) (string, error) {
	if config.Verbose {
		fmt.Printf(
			"    📊 Processing %d planned orders for visualization...\n",
			len(result.PlannedOrders),
		)
	}

	// Build visualization data
	vizData := hv.buildVisualizationData(result, config)

	if config.Verbose {
		fmt.Printf("    🔗 Built network with %d nodes and %d links\n",
			len(vizData.Nodes), len(vizData.Links))
		fmt.Printf("    📅 Timeline spans from %s to %s\n",
			vizData.StartDate.Format("2006-01-02"), vizData.EndDate.Format("2006-01-02"))
	}

	// Convert to JSON for JavaScript embedding
	jsonData, err := json.Marshal(vizData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal visualization data: %w", err)
	}

	if config.Verbose {
		fmt.Printf("    📄 Serialized %d bytes of visualization data\n", len(jsonData))
	}

	// Create template data
	templateData := &TemplateData{
		VisualizationData:      vizData,
		DataJSON:               template.JS(jsonData),
		ExplosionTimeFormatted: hv.formatDuration(config.ExplosionTime),
		GeneratedAt:            time.Now().Format("2006-01-02 15:04:05"),
	}

	if config.Verbose {
		fmt.Printf("    🎨 Loading HTML template...\n")
	}

	// Parse and execute template
	tmpl, err := template.ParseFS(templateFS, "templates/interactive_viz.html")
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	if config.Verbose {
		fmt.Printf("    🔄 Rendering template with data...\n")
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, templateData)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// buildVisualizationData converts MRP results into visualization-ready data
func (hv *HTMLVisualization) buildVisualizationData(
	result *dto.MRPResult,
	config Config,
) *VisualizationData {
	vizData := &VisualizationData{
		Allocations:   result.Allocations,
		Shortages:     result.ShortageReport,
		ExplosionTime: config.ExplosionTime,
	}

	fmt.Printf("Buiding visualization data...\n")

	// Track unique parts and build network nodes
	partMap := make(map[entities.PartNumber]*NetworkNode)

	// Find time bounds
	if len(result.PlannedOrders) > 0 {
		vizData.StartDate = result.PlannedOrders[0].StartDate
		vizData.EndDate = result.PlannedOrders[0].DueDate

		for _, order := range result.PlannedOrders {
			if order.StartDate.Before(vizData.StartDate) {
				vizData.StartDate = order.StartDate
			}
			if order.DueDate.After(vizData.EndDate) {
				vizData.EndDate = order.DueDate
			}
		}
	}

	// Build nodes and timeline bars from planned orders
	fmt.Printf("Processing %d planned orders...\n", len(result.PlannedOrders))
	for i, order := range result.PlannedOrders {
		nodeID := fmt.Sprintf("part_%s_%d", order.PartNumber, i)

		// Create network node
		node := &NetworkNode{
			ID:         nodeID,
			PartNumber: order.PartNumber,
			OrderType:  order.OrderType.String(),
			Quantity:   order.Quantity,
			StartDate:  order.StartDate,
			DueDate:    order.DueDate,
			Location:   order.Location,
			Level:      0, // Will be calculated from BOM relationships
		}

		partMap[order.PartNumber] = node
		vizData.Nodes = append(vizData.Nodes, *node)

		// Create timeline bar
		split := 0
		if strings.Contains(order.DemandTrace, "Split") {
			fmt.Sscanf(order.DemandTrace, "%*s (Split %d)", &split)
		}

		bar := TimelineBar{
			ID:         nodeID,
			PartNumber: order.PartNumber,
			OrderType:  order.OrderType.String(),
			Quantity:   order.Quantity,
			StartDate:  order.StartDate,
			DueDate:    order.DueDate,
			Location:   order.Location,
			Split:      split,
			Color:      hv.getBarColor(order.OrderType, split),
		}

		vizData.TimelineBars = append(vizData.TimelineBars, bar)
	}

	// Build links from BOM relationships (would need BOM access)
	// For now, we'll create links based on demand traces
	fmt.Printf("Building links from demand traces...\n")
	vizData.Links = hv.buildLinksFromDemandTraces(result.PlannedOrders, partMap)

	// Calculate statistics
	vizData.Statistics.TotalOrders = len(result.PlannedOrders)
	vizData.Statistics.TotalParts = len(partMap)
	vizData.Statistics.TotalShortages = len(result.ShortageReport)

	return vizData
}

// buildLinksFromDemandTraces attempts to infer relationships from demand traces
func (hv *HTMLVisualization) buildLinksFromDemandTraces(
	orders []entities.PlannedOrder,
	partMap map[entities.PartNumber]*NetworkNode,
) []NetworkLink {
	var links []NetworkLink

	// Safety limit to prevent memory exhaustion
	const maxOrdersForLinkGeneration = 1000
	const maxLinksGenerated = 50000

	numOrders := len(orders)

	// Add memory monitoring
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	fmt.Printf("  📊 Memory before link generation: %d MB allocated, %d MB in use\n",
		memStats.Alloc/1024/1024, memStats.Sys/1024/1024)

	if numOrders > maxOrdersForLinkGeneration {
		fmt.Printf(
			"  ⚠️  WARNING: %d orders exceeds safe limit of %d for link generation\n",
			numOrders,
			maxOrdersForLinkGeneration,
		)
		fmt.Printf(
			"  🔄 Processing only first %d orders to prevent memory exhaustion\n",
			maxOrdersForLinkGeneration,
		)
		orders = orders[:maxOrdersForLinkGeneration]
		numOrders = maxOrdersForLinkGeneration
	}

	fmt.Printf(
		"  🔄 Processing %d orders (O(n²) = %d comparisons)\n",
		numOrders,
		numOrders*numOrders,
	)

	// This is a simplified approach - in practice you'd want access to the BOM
	// to build proper parent-child relationships
	for i, order := range orders {
		if i%100 == 0 {
			fmt.Printf(
				"  📈 Processing order %d/%d (%0.1f%%)...\n",
				i,
				numOrders,
				float64(i)/float64(numOrders)*100,
			)

			// Check memory usage periodically
			runtime.ReadMemStats(&memStats)
			if memStats.Alloc > 2*1024*1024*1024 { // 2GB limit
				fmt.Printf(
					"  🛑 MEMORY LIMIT REACHED: %d MB allocated, stopping link generation\n",
					memStats.Alloc/1024/1024,
				)
				break
			}
		}

		// Look for other orders that might be parents (crude heuristic)
		for j, potentialParent := range orders {
			if len(links) >= maxLinksGenerated {
				fmt.Printf(
					"  🛑 LINK LIMIT REACHED: Generated %d links, stopping to prevent memory exhaustion\n",
					maxLinksGenerated,
				)
				runtime.ReadMemStats(&memStats)
				fmt.Printf("  📊 Memory at link limit: %d MB allocated, %d MB in use\n",
					memStats.Alloc/1024/1024, memStats.Sys/1024/1024)
				return links
			}

			if i != j && order.DueDate.Before(potentialParent.StartDate) {
				// This order finishes before the other starts - possible dependency
				link := NetworkLink{
					Source:   fmt.Sprintf("part_%s_%d", order.PartNumber, i),
					Target:   fmt.Sprintf("part_%s_%d", potentialParent.PartNumber, j),
					QtyPer:   1, // Would come from BOM
					FindNum:  0, // Would come from BOM
					Priority: 0, // Would come from BOM
				}
				links = append(links, link)
			}
		}
	}

	runtime.ReadMemStats(&memStats)
	fmt.Printf("  📊 Memory after link generation: %d MB allocated, %d MB in use\n",
		memStats.Alloc/1024/1024, memStats.Sys/1024/1024)
	fmt.Printf("  🔗 Found %d links based on demand traces\n", len(links))
	return links
}

// getBarColor returns color based on order type and split status
func (hv *HTMLVisualization) getBarColor(orderType entities.OrderType, split int) string {
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

// formatDuration formats a time duration into human-readable format
func (hv *HTMLVisualization) formatDuration(d time.Duration) string {
	if d < time.Second {
		return "< 1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// generateHTMLOutput creates HTML output file
func generateHTMLOutput(result *dto.MRPResult, config Config) error {
	if config.Verbose {
		fmt.Printf("  🔄 Building visualization data structure...\n")
	}

	viz := NewHTMLVisualization()
	html, err := viz.GenerateHTML(result, config)
	if err != nil {
		return fmt.Errorf("failed to generate HTML visualization: %w", err)
	}

	if config.Verbose {
		fmt.Printf("  📝 Generated HTML document (%d bytes)\n", len(html))
	}

	// Write HTML to file
	filename := config.SVGOutput // Reuse SVGOutput path but change extension
	if strings.HasSuffix(filename, ".svg") {
		filename = strings.TrimSuffix(filename, ".svg") + ".html"
	} else if !strings.HasSuffix(filename, ".html") {
		filename += ".html"
	}

	if config.Verbose {
		fmt.Printf("  💾 Writing HTML file to: %s\n", filename)
	}

	err = os.WriteFile(filename, []byte(html), 0644)
	if err != nil {
		return fmt.Errorf("failed to write HTML file: %w", err)
	}

	if config.Verbose {
		fmt.Printf("🌐 Interactive HTML visualization saved to: %s\n", filename)
	}

	return nil
}
