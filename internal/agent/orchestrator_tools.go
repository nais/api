package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nais/api/internal/agent/chat"
	"github.com/sirupsen/logrus"
)

// executeTool executes a tool call using the tool integration.
// Returns the result string and an optional ChartData if this was a render_chart call.
func (o *Orchestrator) executeTool(ctx context.Context, toolCall chat.ToolCall) (string, *ChartData, error) {
	o.log.WithFields(logrus.Fields{
		"tool": toolCall.Name,
		"args": toolCall.Arguments,
	}).Debug("executing tool")

	// Special handling for render_chart tool
	if toolCall.Name == renderChartToolName {
		chart, err := o.parseChartToolCall(toolCall)
		if err != nil {
			return "", nil, fmt.Errorf("invalid chart parameters: %w", err)
		}
		// Return success message to the LLM and the chart data for the client
		return "Chart rendered successfully. The user can now see the visualization.", chart, nil
	}

	// Execute the tool via tool integration
	result, err := o.toolIntegration.ExecuteTool(ctx, toolCall.Name, toolCall.Arguments)
	if err != nil {
		return "", nil, fmt.Errorf("tool %s failed: %w", toolCall.Name, err)
	}

	// Convert result to JSON string for LLM consumption
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal tool result: %w", err)
	}

	output := string(resultJSON)

	// Truncate large outputs to prevent context window exhaustion
	if len(output) > maxToolOutputChars {
		o.log.WithFields(logrus.Fields{
			"tool":            toolCall.Name,
			"original_length": len(output),
			"truncated_to":    maxToolOutputChars,
		}).Warn("truncating large tool output")
		output = output[:maxToolOutputChars] + "\n\n[Output truncated due to size. Please refine your query to get more specific results.]"
	}

	return output, nil, nil
}

// parseChartToolCall extracts and validates ChartData from a render_chart tool call.
func (o *Orchestrator) parseChartToolCall(toolCall chat.ToolCall) (*ChartData, error) {
	args := toolCall.Arguments

	// Required fields
	chartType, ok := args["chart_type"].(string)
	if !ok || chartType == "" {
		return nil, fmt.Errorf("chart_type is required")
	}
	if chartType != "line" {
		return nil, fmt.Errorf("unsupported chart_type: %s (only 'line' is supported)", chartType)
	}

	title, ok := args["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("title is required")
	}

	environment, ok := args["environment"].(string)
	if !ok || environment == "" {
		return nil, fmt.Errorf("environment is required")
	}

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query is required")
	}

	chart := &ChartData{
		ChartType:   chartType,
		Title:       title,
		Environment: environment,
		Query:       query,
	}

	// Optional fields
	if interval, ok := args["interval"].(string); ok && interval != "" {
		validIntervals := map[string]bool{"1h": true, "6h": true, "1d": true, "7d": true, "30d": true}
		if !validIntervals[interval] {
			return nil, fmt.Errorf("invalid interval: %s (valid values: 1h, 6h, 1d, 7d, 30d)", interval)
		}
		chart.Interval = interval
	}

	if yFormat, ok := args["y_format"].(string); ok && yFormat != "" {
		validFormats := map[string]bool{"number": true, "percentage": true, "bytes": true, "cpu_cores": true, "duration": true}
		if !validFormats[yFormat] {
			return nil, fmt.Errorf("invalid y_format: %s (valid values: number, percentage, bytes, cpu_cores, duration)", yFormat)
		}
		chart.YFormat = yFormat
	}

	if labelTemplate, ok := args["label_template"].(string); ok {
		chart.LabelTemplate = labelTemplate
	}

	return chart, nil
}

// getToolDefinitions returns the tool definitions for the LLM.
func (o *Orchestrator) getToolDefinitions() []chat.ToolDefinition {
	registeredTools := o.toolIntegration.ListTools()
	result := make([]chat.ToolDefinition, 0, len(registeredTools)+1)

	for _, tool := range registeredTools {
		chatParams := make([]chat.ParameterDefinition, len(tool.Parameters))
		for i, p := range tool.Parameters {
			chatParams[i] = chat.ParameterDefinition{
				Name:        p.Name,
				Type:        p.Type,
				Description: p.Description,
				Required:    p.Required,
			}
		}

		chatTool := chat.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  chatParams,
		}
		result = append(result, chatTool)
	}

	// Add the render_chart tool for displaying metrics visualizations
	result = append(result, chat.ToolDefinition{
		Name: renderChartToolName,
		Description: `Render a Prometheus metrics chart in the chat. Use this tool when the user asks about metrics, resource usage, trends, or any data that would be better visualized as a chart rather than described in text.

Currently only line charts are supported. The chart will be rendered by the client using the provided Prometheus query.

Guidelines for when to use this tool:
- CPU, memory, or network usage over time
- Request rates, error rates, or latency trends
- Any time-series metrics the user wants to visualize
- When comparing metrics across pods or containers

Do NOT use this tool for:
- Simple numeric values that don't need visualization
- Non-metrics questions
- When the user explicitly asks for text/numbers only`,
		Parameters: []chat.ParameterDefinition{
			{
				Name:        "chart_type",
				Type:        "string",
				Description: "The type of chart to render. Currently only 'line' is supported.",
				Required:    true,
			},
			{
				Name:        "title",
				Type:        "string",
				Description: "A human-readable title for the chart, e.g., 'CPU Usage for my-app'",
				Required:    true,
			},
			{
				Name:        "environment",
				Type:        "string",
				Description: "The environment to query metrics from (e.g., 'dev', 'prod'). Use the environment from the current context if available.",
				Required:    true,
			},
			{
				Name:        "query",
				Type:        "string",
				Description: "The Prometheus query to execute. Must be a valid PromQL query.",
				Required:    true,
			},
			{
				Name:        "interval",
				Type:        "string",
				Description: "Time interval for the query. Valid values: '1h' (1 hour), '6h' (6 hours), '1d' (1 day), '7d' (7 days), '30d' (30 days). Defaults to '1h'.",
				Required:    false,
			},
			{
				Name:        "y_format",
				Type:        "string",
				Description: "Format type for Y-axis values. Valid values: 'number', 'percentage', 'bytes', 'cpu_cores', 'duration'. Helps the client format the values appropriately.",
				Required:    false,
			},
			{
				Name:        "label_template",
				Type:        "string",
				Description: "Template string for formatting series labels. Use {label_name} syntax, e.g., '{pod}' or '{pod}/{container}'. If not provided, default label formatting is used.",
				Required:    false,
			},
		},
	})

	return result
}
