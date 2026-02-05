package mcp

import (
	"bytes"
	"sync"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
)

// SchemaProvider provides access to the GraphQL schema for MCP tools.
// Implementations can provide the schema from various sources, such as
// introspection queries, local files, or embedded schema definitions.
type SchemaProvider interface {
	// GetSchema returns the parsed GraphQL schema.
	// This is used for schema exploration and query validation.
	GetSchema() *ast.Schema

	// GetSchemaSDL returns the schema as an SDL (Schema Definition Language) string.
	// This is used for MCP resources that expose the raw schema.
	GetSchemaSDL() string
}

// StaticSchemaProvider wraps an ast.Schema and provides it as a SchemaProvider.
// Use this when you have direct access to the schema (e.g., from gqlgen's
// generated ExecutableSchema).
type StaticSchemaProvider struct {
	schema *ast.Schema

	// Lazy-initialized SDL representation
	sdlOnce sync.Once
	sdl     string
}

// NewStaticSchemaProvider creates a SchemaProvider from an existing ast.Schema.
// This is typically used with gqlgen's generated schema:
//
//	schema := gengql.NewExecutableSchema(gengql.Config{}).Schema()
//	provider := mcp.NewStaticSchemaProvider(schema)
func NewStaticSchemaProvider(schema *ast.Schema) *StaticSchemaProvider {
	return &StaticSchemaProvider{
		schema: schema,
	}
}

// GetSchema returns the underlying ast.Schema.
func (p *StaticSchemaProvider) GetSchema() *ast.Schema {
	return p.schema
}

// GetSchemaSDL returns the schema formatted as SDL.
// The SDL is generated lazily on first access and cached.
func (p *StaticSchemaProvider) GetSchemaSDL() string {
	p.sdlOnce.Do(func() {
		var buf bytes.Buffer
		f := formatter.NewFormatter(&buf, formatter.WithComments())
		f.FormatSchema(p.schema)
		p.sdl = buf.String()
	})
	return p.sdl
}

// removeBuiltinScalars removes scalar definitions for built-in GraphQL types
// (Boolean, String, Int, Float, ID) that gqlparser already defines internally.
// These redeclarations in external schemas cause "Cannot redeclare type" errors.
func removeBuiltinScalars(schema string) string {
	// This is used when parsing schemas from external sources that may
	// include built-in scalar definitions.
	builtins := map[string]bool{
		"Boolean": true,
		"String":  true,
		"Int":     true,
		"Float":   true,
		"ID":      true,
	}

	var result bytes.Buffer
	lines := bytes.Split([]byte(schema), []byte("\n"))

	var descriptionLines [][]byte
	inDescription := false

	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)

		// Track description blocks (""" ... """)
		if bytes.HasPrefix(trimmed, []byte(`"""`)) {
			if !inDescription {
				inDescription = true
				descriptionLines = [][]byte{line}
				// Check if it ends on the same line (single-line description)
				if len(trimmed) > 6 && bytes.HasSuffix(trimmed, []byte(`"""`)) {
					inDescription = false
				}
				continue
			} else {
				inDescription = false
				descriptionLines = append(descriptionLines, line)
				continue
			}
		}

		if inDescription {
			descriptionLines = append(descriptionLines, line)
			continue
		}

		// Check if this is a scalar line for a builtin type
		if bytes.HasPrefix(trimmed, []byte("scalar ")) {
			parts := bytes.Fields(trimmed)
			if len(parts) >= 2 {
				scalarName := string(parts[1])
				if builtins[scalarName] {
					// Skip this scalar and the description we just collected
					descriptionLines = nil
					continue
				}
			}
		}

		// If we have pending description lines, write them now
		if len(descriptionLines) > 0 {
			for _, descLine := range descriptionLines {
				result.Write(descLine)
				result.WriteByte('\n')
			}
			descriptionLines = nil
		}

		result.Write(line)
		result.WriteByte('\n')
	}

	return result.String()
}
