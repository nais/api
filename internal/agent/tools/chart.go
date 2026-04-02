// Package tools provides tool definitions and execution for the agent.
package tools

import (
	"context"
	"fmt"
)

// ChartTools provides tools for chart visualization.
type ChartTools struct{}

// NewChartTools creates a new ChartTools instance.
func NewChartTools() *ChartTools {
	return &ChartTools{}
}

// RenderChart validates chart parameters and prepares the chart data.
// Note: The actual rendering happens on the client side using the returned ChartData.
func (t *ChartTools) RenderChart(ctx context.Context, input ChartData) (*ChartData, error) {
	// Validate required fields
	if input.ChartType == "" {
		return nil, fmt.Errorf("chart_type is required")
	}
	if input.ChartType != "line" {
		return nil, fmt.Errorf("unsupported chart_type: %s (only 'line' is supported)", input.ChartType)
	}

	if input.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	if input.Environment == "" {
		return nil, fmt.Errorf("environment is required")
	}

	if input.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Validate optional fields
	if input.Interval != "" {
		validIntervals := map[string]bool{"1h": true, "6h": true, "1d": true, "7d": true, "30d": true}
		if !validIntervals[input.Interval] {
			return nil, fmt.Errorf("invalid interval: %s (valid values: 1h, 6h, 1d, 7d, 30d)", input.Interval)
		}
	}

	if input.YFormat != "" {
		validFormats := map[string]bool{"number": true, "percentage": true, "bytes": true, "cpu_cores": true, "duration": true}
		if !validFormats[input.YFormat] {
			return nil, fmt.Errorf("invalid y_format: %s (valid values: number, percentage, bytes, cpu_cores, duration)", input.YFormat)
		}
	}

	return &input, nil
}
