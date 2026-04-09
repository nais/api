package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nais/api/internal/agent/chat"
	"github.com/sirupsen/logrus"
)

// executeTool executes a tool call using the registry.
// Returns the result string and an optional ChartData if this was a render_chart call.
func (o *Orchestrator) executeTool(ctx context.Context, toolCall chat.ToolCall) (string, *ChartData, error) {
	o.log.WithFields(logrus.Fields{
		"tool": toolCall.Name,
		"args": toolCall.Arguments,
	}).Debug("executing tool")

	result, err := o.registry.Execute(ctx, toolCall.Name, toolCall.Arguments)
	if err != nil {
		return "", nil, fmt.Errorf("tool %s failed: %w", toolCall.Name, err)
	}

	// Check if the result is ChartData (handled specially for visualization)
	if chart, ok := result.(*ChartData); ok {
		return "Chart rendered successfully. The user can now see the visualization.", chart, nil
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

// getToolDefinitions returns the tool definitions for the LLM.
func (o *Orchestrator) getToolDefinitions() []chat.ToolDefinition {
	registeredTools := o.registry.ListTools()
	result := make([]chat.ToolDefinition, 0, len(registeredTools))

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

		result = append(result, chat.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  chatParams,
		})
	}

	return result
}
