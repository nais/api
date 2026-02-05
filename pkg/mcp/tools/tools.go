package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// ToolDefinition describes an MCP tool.
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema,omitempty"` // JSON schema for input
}

// ParameterInfo describes a parameter extracted from a tool's input schema.
// This is a generic representation that can be used by different LLM clients.
type ParameterInfo struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

// GetParameters extracts parameter information from the tool's InputSchema.
// This parses the JSON schema and returns a slice of ParameterInfo.
func (t ToolDefinition) GetParameters() []ParameterInfo {
	if t.InputSchema == nil {
		return nil
	}

	schemaMap, ok := t.InputSchema.(map[string]any)
	if !ok {
		// Try mcp.ToolInputSchema
		if inputSchema, ok := t.InputSchema.(mcp.ToolInputSchema); ok {
			return extractParametersFromToolInputSchema(inputSchema)
		}
		return nil
	}

	return extractParametersFromMap(schemaMap)
}

// extractParametersFromMap extracts parameters from a JSON schema map.
func extractParametersFromMap(schemaMap map[string]any) []ParameterInfo {
	properties, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		return nil
	}

	// Get required fields
	requiredSet := make(map[string]bool)
	if required, ok := schemaMap["required"].([]any); ok {
		for _, r := range required {
			if name, ok := r.(string); ok {
				requiredSet[name] = true
			}
		}
	}
	// Also handle []string for required
	if required, ok := schemaMap["required"].([]string); ok {
		for _, name := range required {
			requiredSet[name] = true
		}
	}

	params := make([]ParameterInfo, 0, len(properties))
	for name, propValue := range properties {
		prop, ok := propValue.(map[string]any)
		if !ok {
			continue
		}

		param := ParameterInfo{
			Name:     name,
			Required: requiredSet[name],
		}

		if t, ok := prop["type"].(string); ok {
			param.Type = t
		}
		if d, ok := prop["description"].(string); ok {
			param.Description = d
		}

		params = append(params, param)
	}

	return params
}

// extractParametersFromToolInputSchema extracts parameters from mcp.ToolInputSchema.
func extractParametersFromToolInputSchema(schema mcp.ToolInputSchema) []ParameterInfo {
	if schema.Properties == nil {
		return nil
	}

	// Get required fields
	requiredSet := make(map[string]bool)
	for _, name := range schema.Required {
		requiredSet[name] = true
	}

	params := make([]ParameterInfo, 0, len(schema.Properties))
	for name, propValue := range schema.Properties {
		prop, ok := propValue.(map[string]any)
		if !ok {
			continue
		}

		param := ParameterInfo{
			Name:     name,
			Required: requiredSet[name],
		}

		if t, ok := prop["type"].(string); ok {
			param.Type = t
		}
		if d, ok := prop["description"].(string); ok {
			param.Description = d
		}

		params = append(params, param)
	}

	return params
}
