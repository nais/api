package agent

import (
	"fmt"
	"strings"

	"github.com/nais/api/internal/agent/chat"
	"github.com/nais/api/internal/agent/rag"
)

// buildSystemPrompt creates the system prompt for the LLM.
func (o *Orchestrator) buildSystemPrompt(ctx *ChatContext, docs []rag.Document, tools []chat.ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString(`You are a helpful assistant for the Nais platform, a Kubernetes-based application platform. You help users understand and troubleshoot their applications.

## Current Context
`)

	if ctx != nil {
		if ctx.Path != "" {
			sb.WriteString(fmt.Sprintf("- User is viewing: %s\n", ctx.Path))
		}
		if ctx.Team != "" {
			sb.WriteString(fmt.Sprintf("- Team: %s\n", ctx.Team))
		}
		if ctx.App != "" {
			sb.WriteString(fmt.Sprintf("- Application: %s\n", ctx.App))
		}
		if ctx.Env != "" {
			sb.WriteString(fmt.Sprintf("- Environment: %s\n", ctx.Env))
		}
	}

	sb.WriteString(`
## Available Tools

You have access to the following tools to help answer user questions:

`)

	// Dynamically list available tools
	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", tool.Name, firstSentence(tool.Description)))
	}

	sb.WriteString(`
## Guidelines
1. **Start with context**: Use get_nais_context first to understand the user and their teams.
2. **Explore before querying**: Use schema exploration tools to understand the API structure before executing queries.
3. **Use specific queries**: Construct targeted GraphQL queries based on what the user needs.
4. **Provide actionable advice**: When possible, include links to relevant console pages using the URL patterns from get_nais_context.
5. **Handle errors gracefully**: If a tool returns an error, explain it clearly to the user and suggest alternatives.
6. **Use documentation**: For general questions about Nais features, refer to the documentation provided below.

## Source Citation Guidelines
When documentation is provided below, you MUST follow these rules:
1. **Use the documentation**: Base your answers on the documentation provided when it is relevant to the user's question.
2. **Cite your sources**: When you use information from the documentation, naturally reference it in your response (e.g., "According to the Nais documentation on X..." or "The documentation explains that...").
3. **Only cite what you use**: Do NOT reference sources that you did not actually use to formulate your answer. If the documentation provided is not relevant to the question, simply answer without citing it.
4. **Be specific**: When citing, be specific about which documentation you're referencing so users can find more details.
5. **Combine sources**: If multiple documentation sources contribute to your answer, reference each one appropriately.
`)

	if len(docs) > 0 {
		sb.WriteString("\n## Documentation\n")
		for _, doc := range docs {
			sb.WriteString(fmt.Sprintf("\n### %s\n%s\nSource: %s\n", doc.Title, doc.Content, doc.URL))
		}
	}

	return sb.String()
}

// firstSentence extracts the first sentence from a string for concise tool descriptions.
func firstSentence(s string) string {
	// Find the first period followed by a space or end of string
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '.' && (i+1 >= len(s) || s[i+1] == ' ' || s[i+1] == '\n') {
			return s[:i+1]
		}
	}
	// If no sentence ending found, return the whole string (truncated if too long)
	if len(s) > 150 {
		return s[:147] + "..."
	}
	return s
}
