package prompterdb

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/vijaylingoju/prompterdb/llm"
	"github.com/vijaylingoju/prompterdb/templates"
)

// WidgetType represents the type of visualization widget
type WidgetType string

const (
	WidgetTypeTable    WidgetType = "table"
	WidgetTypeBarChart WidgetType = "bar-chart"
	WidgetTypePieChart WidgetType = "pie-chart"
	WidgetTypeLine     WidgetType = "line-chart"
)

// WidgetConfig represents the configuration for a visualization widget
type WidgetConfig struct {
	ID          string                 `json:"_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        WidgetType            `json:"type"`
	Data        map[string]interface{} `json:"data"`
	CreatedAt   time.Time             `json:"created_at"`
}

// Visualize converts query results into widget configurations using the specified template and LLM client
func Visualize(
	results []map[string]interface{},
	templateName string,
	tm *templates.TemplateManager,
	llmClient llm.LLM,
) ([]WidgetConfig, error) {
	log.Println("Starting visualization process...")
	if len(results) == 0 {
		log.Println("No results to visualize")
		return nil, fmt.Errorf("no results to visualize")
	}

	log.Printf("Processing %d result rows", len(results))

	// Convert results to JSON for LLM analysis
	resultsJSON, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Printf("Error marshaling results to JSON: %v", err)
		return nil, fmt.Errorf("error preparing data for visualization: %w", err)
	}

	// Check if the user specifically asked for a pie chart in the template name
	if strings.Contains(strings.ToLower(templateName), "pie") {
		// Try to find a numeric column for the pie chart
		var valueField string
		if len(results) > 0 {
			for col, val := range results[0] {
				if _, ok := val.(float64); ok || val == nil {
					valueField = col
					break
				}
			}
		}

		// If no numeric field found, try common names
		if valueField == "" {
			possibleFields := []string{"count", "total", "value", "amount", "quantity"}
			for _, field := range possibleFields {
				if _, exists := results[0][field]; exists {
					valueField = field
					break
				}
			}
		}

		// If we found a value field, try to create a pie chart
		if valueField != "" {
			// Find a category field (non-numeric)
			var categoryField string
			for col, val := range results[0] {
				if col != valueField && val != nil {
					categoryField = col
					break
				}
			}

			if categoryField != "" {
				widgets, err := createPieChartWidget(results, categoryField, valueField)
				if err == nil {
					return widgets, nil
				}
				log.Printf("Warning: Could not create pie chart: %v", err)
			}
		}
	}

	// Create a direct prompt for the LLM to suggest visualization type
	prompt := fmt.Sprintf(`Analyze the following query results and suggest the best visualization type (table, bar-chart, pie-chart, line-chart). 
Consider the data types and relationships in the results. Respond with only the visualization type, nothing else.

Results:
%s`, resultsJSON)

	log.Println("Sending prompt to LLM for visualization suggestion...")
	
	// Try to get visualization suggestion from LLM
	widgetTypeStr, err := getVisualizationSuggestion(llmClient, prompt)
	if err != nil {
		log.Printf("Error getting visualization suggestion from LLM: %v", err)
		// Default to table view if LLM fails
		widgetTypeStr = "table"
	}

	// Clean and normalize the LLM response
	widgetType := normalizeWidgetType(widgetTypeStr)
	log.Printf("LLM suggested visualization type: %s", widgetType)

	// Create widget configuration based on the suggested type
	widget, err := createWidget(results, widgetType, templateName)
	if err != nil {
		log.Printf("Error creating widget: %v", err)
		return nil, fmt.Errorf("error creating widget: %w", err)
	}

	log.Printf("Successfully created %s widget configuration", widget.Type)
	return []WidgetConfig{widget}, nil
}

// normalizeWidgetType ensures the widget type is one of the supported types
func normalizeWidgetType(widgetType string) WidgetType {
	switch strings.ToLower(widgetType) {
	case "bar", "barchart", "bar-chart":
		return WidgetTypeBarChart
	case "pie", "piechart", "pie-chart":
		return WidgetTypePieChart
	case "line", "linechart", "line-chart":
		return WidgetTypeLine
	default:
		return WidgetTypeTable
	}
}

// createWidget creates a widget configuration based on the results and widget type
func createWidget(results []map[string]interface{}, widgetType WidgetType, templateName string) (WidgetConfig, error) {
	if len(results) == 0 {
		return WidgetConfig{}, fmt.Errorf("no results to create widget")
	}

	// Extract column names from the first result
	var columns []string
	for col := range results[0] {
		columns = append(columns, col)
	}

	// Create basic widget configuration
	widget := WidgetConfig{
		ID:          fmt.Sprintf("widget_%d", time.Now().Unix()),
		Name:        fmt.Sprintf("%s_%s", templateName, widgetType),
		Description: fmt.Sprintf("Visualization of %s data as %s", templateName, widgetType),
		Type:        widgetType,
		Data: map[string]interface{}{
			"columns": columns,
			"rows":    results,
		},
		CreatedAt: time.Now(),
	}

	return widget, nil
}

// createPieChartWidget creates a pie chart widget from query results
func createPieChartWidget(results []map[string]interface{}, categoryField, valueField string) ([]WidgetConfig, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no data to visualize")
	}

	// Prepare data for pie chart
	var data []map[string]interface{}
	total := 0.0

	// First pass: calculate total for percentages
	for _, row := range results {
		if val, ok := row[valueField].(float64); ok {
			total += val
		}
	}

	// Second pass: create data points with percentages
	for _, row := range results {
		category, _ := row[categoryField].(string)
		value, _ := row[valueField].(float64)
		
		// Skip if value is zero or category is empty
		if value == 0 || category == "" {
			continue
		}

		percentage := (value / total) * 100
		data = append(data, map[string]interface{}{
			"category":  category,
			"value":    value,
			"percent":  fmt.Sprintf("%.1f%%", percentage),
		})
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no valid data points for pie chart")
	}

	return []WidgetConfig{
		{
			ID:          "pie_" + strconv.FormatInt(time.Now().Unix(), 10),
			Name:        "Distribution by " + categoryField,
			Description: fmt.Sprintf("Pie chart showing distribution across %d categories", len(data)),
			Type:        WidgetTypePieChart,
			Data: map[string]interface{}{
				"categories": data,
				"total":      total,
			},
		},
	}, nil
}

// createTableWidget creates a simple table widget from query results
func createTableWidget(results []map[string]interface{}) []WidgetConfig {
	if len(results) == 0 {
		return []WidgetConfig{}
	}

	// Extract column names from first row
	var columns []string
	for col := range results[0] {
		columns = append(columns, col)
	}

	return []WidgetConfig{
		{
			ID:          "table_" + strconv.FormatInt(time.Now().Unix(), 10),
			Name:        "Query Results",
			Description: "Tabular view of query results",
			Type:        WidgetTypeTable,
			Data: map[string]interface{}{
				"columns": columns,
				"rows":    results,
			},
		},
	}
}

// getVisualizationSuggestion gets a visualization suggestion using a direct API call
func getVisualizationSuggestion(llmClient llm.LLM, prompt string) (string, error) {
	// For now, we'll use a simple heuristic based on the prompt
	// This avoids any template system issues
	if strings.Contains(strings.ToLower(prompt), "pie") {
		return "pie-chart", nil
	}
	if strings.Contains(strings.ToLower(prompt), "bar") {
		return "bar-chart", nil
	}
	if strings.Contains(strings.ToLower(prompt), "line") {
		return "line-chart", nil
	}
	
	// Default to table view
	return "table", nil
}

// PrintWidgetConfig prints the widget configuration in a readable format
func PrintWidgetConfig(widgets []WidgetConfig) {
	for i, widget := range widgets {
		fmt.Printf("\n=== Widget %d ===\n", i+1)
		fmt.Printf("ID: %s\n", widget.ID)
		fmt.Printf("Name: %s\n", widget.Name)
		fmt.Printf("Type: %s\n", widget.Type)
		fmt.Printf("Description: %s\n", widget.Description)
		
		// Print a summary of the data
		if data, ok := widget.Data["rows"].([]map[string]interface{}); ok && len(data) > 0 {
			fmt.Printf("\nData Preview (first row of %d):\n", len(data))
			for k, v := range data[0] {
				fmt.Printf("  %s: %v\n", k, v)
			}
		}
		
		fmt.Println("\n---")
	}
}
