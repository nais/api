package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

// SchemaTools provides schema exploration functionality.
type SchemaTools struct {
	schema *ast.Schema
}

// NewSchemaTools creates a new SchemaTools instance.
func NewSchemaTools(schema *ast.Schema) *SchemaTools {
	return &SchemaTools{schema: schema}
}

// ListTypes lists all types in the schema, grouped by kind.
func (s *SchemaTools) ListTypes(ctx context.Context, input SchemaListTypesInput) (SchemaListTypesOutput, error) {
	kind := strings.ToUpper(input.Kind)
	if kind == "" {
		kind = "ALL"
	}
	search := strings.ToLower(input.Search)

	var output SchemaListTypesOutput

	for name, def := range s.schema.Types {
		// Skip built-in types
		if strings.HasPrefix(name, "__") {
			continue
		}

		// Filter by kind
		defKind := string(def.Kind)
		if kind != "ALL" && defKind != kind {
			continue
		}

		// Filter by search
		if search != "" && !strings.Contains(strings.ToLower(name), search) {
			continue
		}

		switch def.Kind {
		case ast.Object:
			output.Objects = append(output.Objects, name)
		case ast.Interface:
			output.Interfaces = append(output.Interfaces, name)
		case ast.Enum:
			output.Enums = append(output.Enums, name)
		case ast.Union:
			output.Unions = append(output.Unions, name)
		case ast.InputObject:
			output.InputObjects = append(output.InputObjects, name)
		case ast.Scalar:
			output.Scalars = append(output.Scalars, name)
		}
	}

	// Sort all lists
	sort.Strings(output.Objects)
	sort.Strings(output.Interfaces)
	sort.Strings(output.Enums)
	sort.Strings(output.Unions)
	sort.Strings(output.InputObjects)
	sort.Strings(output.Scalars)

	return output, nil
}

// GetType returns details about a specific type.
func (s *SchemaTools) GetType(ctx context.Context, input SchemaGetTypeInput) (SchemaGetTypeOutput, error) {
	def, ok := s.schema.Types[input.Name]
	if !ok {
		return SchemaGetTypeOutput{}, fmt.Errorf("type %q not found", input.Name)
	}

	output := SchemaGetTypeOutput{
		Name:        def.Name,
		Kind:        string(def.Kind),
		Description: def.Description,
	}

	// Add interfaces this type implements
	if len(def.Interfaces) > 0 {
		output.Implements = append([]string(nil), def.Interfaces...)
	}

	// Add fields for OBJECT, INTERFACE, INPUT_OBJECT
	if def.Kind == ast.Object || def.Kind == ast.Interface || def.Kind == ast.InputObject {
		output.Fields = formatFields(def.Fields)
	}

	// Add enum values for ENUM
	if def.Kind == ast.Enum {
		for _, v := range def.EnumValues {
			value := SchemaEnumValue{
				Name:        v.Name,
				Description: v.Description,
			}
			if dep := v.Directives.ForName("deprecated"); dep != nil {
				if reason := dep.Arguments.ForName("reason"); reason != nil {
					value.Deprecated = reason.Value.Raw
				} else {
					value.Deprecated = true
				}
			}
			output.Values = append(output.Values, value)
		}
	}

	// Add types for UNION
	if def.Kind == ast.Union {
		output.Types = append([]string(nil), def.Types...)
	}

	// Add implementedBy for INTERFACE
	if def.Kind == ast.Interface {
		var implementedBy []string
		for typeName, typeDef := range s.schema.Types {
			for _, iface := range typeDef.Interfaces {
				if iface == input.Name {
					implementedBy = append(implementedBy, typeName)
					break
				}
			}
		}
		sort.Strings(implementedBy)
		if len(implementedBy) > 0 {
			output.ImplementedBy = implementedBy
		}
	}

	return output, nil
}

// ListQueries lists all available query operations.
func (s *SchemaTools) ListQueries(ctx context.Context, input SchemaListQueriesInput) ([]SchemaOperationInfo, error) {
	search := strings.ToLower(input.Search)

	queryType := s.schema.Query
	if queryType == nil {
		return nil, fmt.Errorf("query type not found in schema")
	}

	var queries []SchemaOperationInfo
	for _, field := range queryType.Fields {
		if search == "" || strings.Contains(strings.ToLower(field.Name), search) || strings.Contains(strings.ToLower(field.Description), search) {
			queries = append(queries, SchemaOperationInfo{
				Name:        field.Name,
				ReturnType:  field.Type.String(),
				Description: truncate(field.Description, 150),
				ArgCount:    len(field.Arguments),
			})
		}
	}

	// Sort by name
	sort.Slice(queries, func(i, j int) bool {
		return queries[i].Name < queries[j].Name
	})

	return queries, nil
}

// ListMutations lists all available mutation operations.
func (s *SchemaTools) ListMutations(ctx context.Context, input SchemaListMutationsInput) ([]SchemaOperationInfo, error) {
	search := strings.ToLower(input.Search)

	mutationType := s.schema.Mutation
	if mutationType == nil {
		return []SchemaOperationInfo{}, nil
	}

	var mutations []SchemaOperationInfo
	for _, field := range mutationType.Fields {
		if search == "" || strings.Contains(strings.ToLower(field.Name), search) || strings.Contains(strings.ToLower(field.Description), search) {
			mutations = append(mutations, SchemaOperationInfo{
				Name:        field.Name,
				ReturnType:  field.Type.String(),
				Description: truncate(field.Description, 150),
				ArgCount:    len(field.Arguments),
			})
		}
	}

	// Sort by name
	sort.Slice(mutations, func(i, j int) bool {
		return mutations[i].Name < mutations[j].Name
	})

	return mutations, nil
}

// GetField returns details about a specific field on a type.
func (s *SchemaTools) GetField(ctx context.Context, input SchemaGetFieldInput) (SchemaGetFieldOutput, error) {
	typeDef, ok := s.schema.Types[input.Type]
	if !ok {
		return SchemaGetFieldOutput{}, fmt.Errorf("type %q not found", input.Type)
	}

	var field *ast.FieldDefinition
	for _, f := range typeDef.Fields {
		if f.Name == input.Field {
			field = f
			break
		}
	}

	if field == nil {
		return SchemaGetFieldOutput{}, fmt.Errorf("field %q not found on type %q", input.Field, input.Type)
	}

	output := SchemaGetFieldOutput{
		Name:        field.Name,
		Type:        field.Type.String(),
		Description: field.Description,
	}

	// Check for deprecation
	if dep := field.Directives.ForName("deprecated"); dep != nil {
		if reason := dep.Arguments.ForName("reason"); reason != nil {
			output.Deprecated = reason.Value.Raw
		} else {
			output.Deprecated = true
		}
	}

	// Add arguments
	if len(field.Arguments) > 0 {
		for _, arg := range field.Arguments {
			argInfo := SchemaArgumentInfo{
				Name:        arg.Name,
				Type:        arg.Type.String(),
				Description: arg.Description,
			}
			if arg.DefaultValue != nil {
				argInfo.Default = arg.DefaultValue.String()
			}
			output.Args = append(output.Args, argInfo)
		}
	}

	return output, nil
}

// GetEnum returns details about an enum type.
func (s *SchemaTools) GetEnum(ctx context.Context, input SchemaGetEnumInput) (SchemaGetEnumOutput, error) {
	def, ok := s.schema.Types[input.Name]
	if !ok || def.Kind != ast.Enum {
		return SchemaGetEnumOutput{}, fmt.Errorf("enum %q not found", input.Name)
	}

	output := SchemaGetEnumOutput{
		Name:        def.Name,
		Description: def.Description,
	}

	for _, v := range def.EnumValues {
		value := SchemaEnumValue{
			Name:        v.Name,
			Description: v.Description,
		}
		if dep := v.Directives.ForName("deprecated"); dep != nil {
			if reason := dep.Arguments.ForName("reason"); reason != nil {
				value.Deprecated = reason.Value.Raw
			} else {
				value.Deprecated = true
			}
		}
		output.Values = append(output.Values, value)
	}

	return output, nil
}

// Search searches across all schema types, fields, and enum values.
func (s *SchemaTools) Search(ctx context.Context, input SchemaSearchInput) (SchemaSearchOutput, error) {
	query := strings.ToLower(input.Query)
	var results []SchemaSearchResult

	// Search types
	for name, def := range s.schema.Types {
		// Skip built-in types
		if strings.HasPrefix(name, "__") {
			continue
		}

		// Match type name or description
		if strings.Contains(strings.ToLower(name), query) || strings.Contains(strings.ToLower(def.Description), query) {
			results = append(results, SchemaSearchResult{
				Kind:        strings.ToLower(string(def.Kind)),
				Name:        name,
				Description: truncate(def.Description, 100),
			})
		}

		// Search fields
		for _, field := range def.Fields {
			if strings.Contains(strings.ToLower(field.Name), query) || strings.Contains(strings.ToLower(field.Description), query) {
				results = append(results, SchemaSearchResult{
					Kind:        "field",
					Type:        name,
					Name:        field.Name,
					FieldType:   field.Type.String(),
					Description: truncate(field.Description, 100),
				})
			}
		}

		// Search enum values
		for _, v := range def.EnumValues {
			if strings.Contains(strings.ToLower(v.Name), query) || strings.Contains(strings.ToLower(v.Description), query) {
				results = append(results, SchemaSearchResult{
					Kind:        "enum_value",
					Enum:        name,
					Name:        v.Name,
					Description: truncate(v.Description, 100),
				})
			}
		}
	}

	// Sort results by kind then name
	sort.Slice(results, func(i, j int) bool {
		if results[i].Kind != results[j].Kind {
			return results[i].Kind < results[j].Kind
		}
		return results[i].Name < results[j].Name
	})

	// Limit results
	if len(results) > 50 {
		results = results[:50]
	}

	return SchemaSearchOutput{
		TotalMatches: len(results),
		Results:      results,
	}, nil
}

// GetImplementors returns all types that implement a given interface.
func (s *SchemaTools) GetImplementors(ctx context.Context, input SchemaGetImplementorsInput) (SchemaGetImplementorsOutput, error) {
	// Verify the interface exists
	def, ok := s.schema.Types[input.Interface]
	if !ok || def.Kind != ast.Interface {
		return SchemaGetImplementorsOutput{}, fmt.Errorf("interface %q not found", input.Interface)
	}

	var implementors []SchemaImplementorInfo
	for typeName, typeDef := range s.schema.Types {
		for _, iface := range typeDef.Interfaces {
			if iface == input.Interface {
				implementors = append(implementors, SchemaImplementorInfo{
					Name:        typeName,
					Description: truncate(typeDef.Description, 100),
				})
				break
			}
		}
	}

	// Sort by name
	sort.Slice(implementors, func(i, j int) bool {
		return implementors[i].Name < implementors[j].Name
	})

	return SchemaGetImplementorsOutput{
		Interface:    input.Interface,
		Description:  def.Description,
		Implementors: implementors,
		Count:        len(implementors),
	}, nil
}

// GetUnionTypes returns all member types of a union.
func (s *SchemaTools) GetUnionTypes(ctx context.Context, input SchemaGetUnionTypesInput) (SchemaGetUnionTypesOutput, error) {
	def, ok := s.schema.Types[input.Union]
	if !ok || def.Kind != ast.Union {
		return SchemaGetUnionTypesOutput{}, fmt.Errorf("union %q not found", input.Union)
	}

	var types []SchemaUnionMember
	for _, typeName := range def.Types {
		typeDef := s.schema.Types[typeName]
		member := SchemaUnionMember{
			Name: typeName,
		}
		if typeDef != nil {
			member.Description = truncate(typeDef.Description, 100)
		}
		types = append(types, member)
	}

	return SchemaGetUnionTypesOutput{
		Union:       input.Union,
		Description: def.Description,
		Types:       types,
		Count:       len(types),
	}, nil
}

// formatFields converts AST fields to SchemaFieldInfo slice.
func formatFields(fields ast.FieldList) []SchemaFieldInfo {
	var result []SchemaFieldInfo
	for _, f := range fields {
		field := SchemaFieldInfo{
			Name:        f.Name,
			Type:        f.Type.String(),
			Description: truncate(f.Description, 150),
		}
		if dep := f.Directives.ForName("deprecated"); dep != nil {
			if reason := dep.Arguments.ForName("reason"); reason != nil {
				field.Deprecated = reason.Value.Raw
			} else {
				field.Deprecated = true
			}
		}
		result = append(result, field)
	}
	return result
}

// truncate truncates a string to the specified length.
func truncate(s string, length int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}
