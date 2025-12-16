//go:build integration

// This file contains integration tests that validate our Go structs against the Bifrost Swagger spec.
// These tests require network access and are skipped during normal test runs.
//
// Run these tests with:
//   go test -v -tags=integration ./internal/unleash/
//
// What these tests validate:
// - Field existence: Every field in our Go structs exists in the Bifrost API
// - Field types: Types match between Go structs and Swagger spec (string, int, bool, etc.)
// - Breaking changes: Tests fail if Bifrost removes/renames fields we depend on
// - Additive changes: Tests pass (log only) if Bifrost adds new optional fields
//
// These tests are resilient to:
// ✅ New optional fields added to Bifrost API
// ✅ New endpoints added to Bifrost
// ✅ Fields in Swagger that we don't use
//
// These tests will fail on:
// ❌ Fields we use are removed from Bifrost
// ❌ Fields we use are renamed in Bifrost
// ❌ Field types change in incompatible ways

package unleash

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bifrostSwaggerURL = "https://raw.githubusercontent.com/nais/bifrost/main/docs/swagger.json"

// SwaggerSpec represents the top-level Swagger/OpenAPI 2.0 structure
type SwaggerSpec struct {
	Swagger     string                       `json:"swagger"`
	Info        SwaggerInfo                  `json:"info"`
	BasePath    string                       `json:"basePath"`
	Paths       map[string]SwaggerPath       `json:"paths"`
	Definitions map[string]SwaggerDefinition `json:"definitions"`
}

type SwaggerInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type SwaggerPath struct {
	Get    *SwaggerOperation `json:"get,omitempty"`
	Post   *SwaggerOperation `json:"post,omitempty"`
	Put    *SwaggerOperation `json:"put,omitempty"`
	Delete *SwaggerOperation `json:"delete,omitempty"`
}

type SwaggerOperation struct {
	Summary     string                     `json:"summary"`
	Description string                     `json:"description"`
	Parameters  []SwaggerParameter         `json:"parameters"`
	Responses   map[string]SwaggerResponse `json:"responses"`
}

type SwaggerParameter struct {
	Name        string            `json:"name"`
	In          string            `json:"in"`
	Required    bool              `json:"required"`
	Type        string            `json:"type,omitempty"`
	Description string            `json:"description"`
	Schema      *SwaggerSchemaRef `json:"schema,omitempty"`
}

type SwaggerResponse struct {
	Description string            `json:"description"`
	Schema      *SwaggerSchemaRef `json:"schema,omitempty"`
}

type SwaggerSchemaRef struct {
	Ref   string            `json:"$ref,omitempty"`
	Type  string            `json:"type,omitempty"`
	Items *SwaggerSchemaRef `json:"items,omitempty"`
}

type SwaggerDefinition struct {
	Type                 string                     `json:"type"`
	Properties           map[string]SwaggerProperty `json:"properties,omitempty"`
	Required             []string                   `json:"required,omitempty"`
	AdditionalProperties *SwaggerProperty           `json:"additionalProperties,omitempty"`
}

type SwaggerProperty struct {
	Type                 string             `json:"type,omitempty"`
	Description          string             `json:"description,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
	Items                *SwaggerProperty   `json:"items,omitempty"`
	AdditionalProperties *SwaggerProperty   `json:"additionalProperties,omitempty"`
	AllOf                []SwaggerSchemaRef `json:"allOf,omitempty"`
}

// fetchSwaggerSpec fetches the Swagger spec from the Bifrost repository
func fetchSwaggerSpec(t *testing.T) *SwaggerSpec {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(bifrostSwaggerURL)
	require.NoError(t, err, "Failed to fetch swagger spec")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Unexpected status code fetching swagger spec")

	var spec SwaggerSpec
	err = json.NewDecoder(resp.Body).Decode(&spec)
	require.NoError(t, err, "Failed to decode swagger spec")

	return &spec
}

// getDefinitionName extracts the definition name from a $ref like "#/definitions/dto.UnleashConfigRequest"
func getDefinitionName(ref string) string {
	parts := strings.Split(ref, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ref
}

// getJSONTagName extracts the JSON field name from a struct field tag
func getJSONTagName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" || tag == "-" {
		return ""
	}
	// Handle tags like "name,omitempty"
	parts := strings.Split(tag, ",")
	return parts[0]
}

// getStructJSONFields returns a map of JSON field names to their types for a given struct
func getStructJSONFields(t reflect.Type) map[string]reflect.StructField {
	fields := make(map[string]reflect.StructField)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonName := getJSONTagName(field)
		if jsonName != "" {
			fields[jsonName] = field
		}
	}
	return fields
}

// validateFieldType validates that a Go field type is compatible with a Swagger property type
func validateFieldType(t *testing.T, fieldName string, goField reflect.StructField, swaggerProp SwaggerProperty) {
	goType := goField.Type

	// Handle pointer types
	if goType.Kind() == reflect.Ptr {
		goType = goType.Elem()
	}

	// Map Go types to Swagger types
	var expectedSwaggerTypes []string
	switch goType.Kind() {
	case reflect.String:
		expectedSwaggerTypes = []string{"string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		expectedSwaggerTypes = []string{"integer", "number"}
	case reflect.Float32, reflect.Float64:
		expectedSwaggerTypes = []string{"number"}
	case reflect.Bool:
		expectedSwaggerTypes = []string{"boolean"}
	case reflect.Map:
		expectedSwaggerTypes = []string{"object"}
	case reflect.Slice, reflect.Array:
		expectedSwaggerTypes = []string{"array"}
	default:
		// For complex types (structs, interfaces), we can't easily validate
		return
	}

	// Check if the swagger type matches
	typeMatches := false
	for _, expected := range expectedSwaggerTypes {
		if swaggerProp.Type == expected {
			typeMatches = true
			break
		}
	}

	if !typeMatches && swaggerProp.Type != "" {
		t.Errorf("Field %q type mismatch: Go type %v, Swagger type %q (expected one of %v)",
			fieldName, goField.Type, swaggerProp.Type, expectedSwaggerTypes)
	}
}

// TestBifrostV1CreateRequestMatchesSwagger validates BifrostV1CreateRequest against dto.UnleashConfigRequest
func TestBifrostV1CreateRequestMatchesSwagger(t *testing.T) {
	spec := fetchSwaggerSpec(t)

	// Get the dto.UnleashConfigRequest definition from swagger
	def, ok := spec.Definitions["dto.UnleashConfigRequest"]
	require.True(t, ok, "dto.UnleashConfigRequest not found in swagger definitions")

	// Get the fields from our Go struct
	goFields := getStructJSONFields(reflect.TypeOf(BifrostV1CreateRequest{}))

	// Track which swagger fields we've matched
	matchedSwaggerFields := make(map[string]bool)

	// Validate each Go struct field exists in swagger spec and has correct type
	for jsonName, goField := range goFields {
		swaggerProp, exists := def.Properties[jsonName]
		assert.True(t, exists, "BifrostV1CreateRequest field %q not found in swagger dto.UnleashConfigRequest", jsonName)
		if exists {
			validateFieldType(t, jsonName, goField, swaggerProp)
		}
		matchedSwaggerFields[jsonName] = true
	}

	// Report any swagger fields that are not in our Go struct (informational, not errors)
	// These might be optional fields we don't use
	for swaggerFieldName := range def.Properties {
		if !matchedSwaggerFields[swaggerFieldName] {
			t.Logf("INFO: Swagger field %q exists in dto.UnleashConfigRequest but not in BifrostV1CreateRequest (may be intentionally omitted)", swaggerFieldName)
		}
	}

	// Verify expected fields exist with correct JSON names
	expectedFields := []string{
		"name",
		"enable_federation",
		"allowed_teams",
		"allowed_clusters",
		"log_level",
		"database_pool_max",
		"database_pool_idle_timeout_ms",
		"release_channel_name",
	}

	for _, field := range expectedFields {
		_, existsInSwagger := def.Properties[field]
		_, existsInGo := goFields[field]
		assert.True(t, existsInSwagger, "Expected field %q not found in swagger spec", field)
		assert.True(t, existsInGo, "Expected field %q not found in BifrostV1CreateRequest", field)
	}
}

// TestBifrostV1UpdateRequestMatchesSwagger validates BifrostV1UpdateRequest against dto.UnleashConfigRequest
func TestBifrostV1UpdateRequestMatchesSwagger(t *testing.T) {
	spec := fetchSwaggerSpec(t)

	// Get the dto.UnleashConfigRequest definition from swagger
	def, ok := spec.Definitions["dto.UnleashConfigRequest"]
	require.True(t, ok, "dto.UnleashConfigRequest not found in swagger definitions")

	// Get the fields from our Go struct
	goFields := getStructJSONFields(reflect.TypeOf(BifrostV1UpdateRequest{}))

	// Validate each Go struct field exists in swagger spec and has correct type
	for jsonName, goField := range goFields {
		swaggerProp, exists := def.Properties[jsonName]
		assert.True(t, exists, "BifrostV1UpdateRequest field %q not found in swagger dto.UnleashConfigRequest", jsonName)
		if exists {
			validateFieldType(t, jsonName, goField, swaggerProp)
		}
	}

	// Update request should have these fields (subset of create request)
	expectedFields := []string{
		"allowed_teams",
		"release_channel_name",
	}

	for _, field := range expectedFields {
		_, existsInSwagger := def.Properties[field]
		_, existsInGo := goFields[field]
		assert.True(t, existsInSwagger, "Expected field %q not found in swagger spec", field)
		assert.True(t, existsInGo, "Expected field %q not found in BifrostV1UpdateRequest", field)
	}
}

// TestBifrostV1ErrorResponseMatchesSwagger validates BifrostV1ErrorResponse against handlers.ErrorResponse
func TestBifrostV1ErrorResponseMatchesSwagger(t *testing.T) {
	spec := fetchSwaggerSpec(t)

	// Get the handlers.ErrorResponse definition from swagger
	def, ok := spec.Definitions["handlers.ErrorResponse"]
	require.True(t, ok, "handlers.ErrorResponse not found in swagger definitions")

	// Get the fields from our Go struct
	goFields := getStructJSONFields(reflect.TypeOf(BifrostV1ErrorResponse{}))

	// Validate each Go struct field exists in swagger spec and has correct type
	// This catches: Bifrost removed a field we depend on or changed its type
	for jsonName, goField := range goFields {
		swaggerProp, exists := def.Properties[jsonName]
		assert.True(t, exists, "BifrostV1ErrorResponse field %q not found in swagger handlers.ErrorResponse - Bifrost may have removed this field", jsonName)
		if exists {
			validateFieldType(t, jsonName, goField, swaggerProp)
		}
	}

	// Log new swagger fields we don't have (informational only)
	// Go's JSON decoder ignores unknown fields, so this is not a breaking change
	for swaggerFieldName := range def.Properties {
		if _, existsInGo := goFields[swaggerFieldName]; !existsInGo {
			t.Logf("INFO: Swagger field %q exists in handlers.ErrorResponse but not in BifrostV1ErrorResponse (additive change, not breaking)", swaggerFieldName)
		}
	}

	// Verify expected fields exist - these are the fields we actually use
	expectedFields := []string{
		"error",
		"message",
		"details",
		"status_code",
	}

	for _, field := range expectedFields {
		_, existsInSwagger := def.Properties[field]
		_, existsInGo := goFields[field]
		assert.True(t, existsInSwagger, "Expected field %q not found in swagger spec", field)
		assert.True(t, existsInGo, "Expected field %q not found in BifrostV1ErrorResponse", field)
	}
}

// TestBifrostV1ReleaseChannelResponseMatchesSwagger validates BifrostV1ReleaseChannelResponse against handlers.ReleaseChannelResponse
func TestBifrostV1ReleaseChannelResponseMatchesSwagger(t *testing.T) {
	spec := fetchSwaggerSpec(t)

	// Get the handlers.ReleaseChannelResponse definition from swagger
	def, ok := spec.Definitions["handlers.ReleaseChannelResponse"]
	require.True(t, ok, "handlers.ReleaseChannelResponse not found in swagger definitions")

	// Get the fields from our Go struct
	goFields := getStructJSONFields(reflect.TypeOf(BifrostV1ReleaseChannelResponse{}))

	// Validate each Go struct field exists in swagger spec and has correct type
	// This catches: Bifrost removed a field we depend on or changed its type
	for jsonName, goField := range goFields {
		swaggerProp, exists := def.Properties[jsonName]
		assert.True(t, exists, "BifrostV1ReleaseChannelResponse field %q not found in swagger handlers.ReleaseChannelResponse - Bifrost may have removed this field", jsonName)
		if exists {
			validateFieldType(t, jsonName, goField, swaggerProp)
		}
	}

	// Log new swagger fields we don't have (informational only)
	// Go's JSON decoder ignores unknown fields, so this is not a breaking change
	for swaggerFieldName := range def.Properties {
		if _, existsInGo := goFields[swaggerFieldName]; !existsInGo {
			t.Logf("INFO: Swagger field %q exists in handlers.ReleaseChannelResponse but not in BifrostV1ReleaseChannelResponse (additive change, not breaking)", swaggerFieldName)
		}
	}

	// Verify expected fields exist - these are the fields we actually use
	expectedFields := []string{
		"name",
		"version",
		"type",
		"description",
		"schedule",
		"current_version",
		"last_updated",
		"created_at",
	}

	for _, field := range expectedFields {
		_, existsInSwagger := def.Properties[field]
		_, existsInGo := goFields[field]
		assert.True(t, existsInSwagger, "Expected field %q not found in swagger spec", field)
		assert.True(t, existsInGo, "Expected field %q not found in BifrostV1ReleaseChannelResponse", field)
	}
}

// TestSwaggerAPIEndpointsExist validates that the expected API endpoints exist in swagger
func TestSwaggerAPIEndpointsExist(t *testing.T) {
	spec := fetchSwaggerSpec(t)

	// Verify basePath
	assert.Equal(t, "/", spec.BasePath, "Unexpected basePath")

	// Expected endpoints
	expectedEndpoints := map[string][]string{
		"/v1/unleash":                {"get", "post"},
		"/v1/unleash/{name}":         {"get", "put", "delete"},
		"/v1/releasechannels":        {"get"},
		"/v1/releasechannels/{name}": {"get"},
	}

	for path, methods := range expectedEndpoints {
		pathSpec, exists := spec.Paths[path]
		assert.True(t, exists, "Expected endpoint %q not found in swagger spec", path)

		if exists {
			for _, method := range methods {
				switch method {
				case "get":
					assert.NotNil(t, pathSpec.Get, "Expected GET method for %q", path)
				case "post":
					assert.NotNil(t, pathSpec.Post, "Expected POST method for %q", path)
				case "put":
					assert.NotNil(t, pathSpec.Put, "Expected PUT method for %q", path)
				case "delete":
					assert.NotNil(t, pathSpec.Delete, "Expected DELETE method for %q", path)
				}
			}
		}
	}
}

// TestSwaggerRequestBodySchema validates that our request struct matches the swagger request body schema
func TestSwaggerRequestBodySchema(t *testing.T) {
	spec := fetchSwaggerSpec(t)

	// Check POST /v1/unleash request body
	unleashPath, exists := spec.Paths["/v1/unleash"]
	require.True(t, exists, "Path /v1/unleash not found")
	require.NotNil(t, unleashPath.Post, "POST /v1/unleash not found")

	// Find the body parameter
	var bodyParam *SwaggerParameter
	for i := range unleashPath.Post.Parameters {
		if unleashPath.Post.Parameters[i].In == "body" {
			bodyParam = &unleashPath.Post.Parameters[i]
			break
		}
	}
	require.NotNil(t, bodyParam, "Body parameter not found for POST /v1/unleash")
	require.NotNil(t, bodyParam.Schema, "Schema not found for body parameter")

	// Verify it references dto.UnleashConfigRequest
	expectedRef := "#/definitions/dto.UnleashConfigRequest"
	assert.Equal(t, expectedRef, bodyParam.Schema.Ref, "Unexpected schema reference for POST /v1/unleash body")
}

// TestSwaggerResponseSchema validates that our understanding of response types is correct
func TestSwaggerResponseSchema(t *testing.T) {
	spec := fetchSwaggerSpec(t)

	// Check POST /v1/unleash 201 response
	unleashPath, exists := spec.Paths["/v1/unleash"]
	require.True(t, exists, "Path /v1/unleash not found")
	require.NotNil(t, unleashPath.Post, "POST /v1/unleash not found")

	created, exists := unleashPath.Post.Responses["201"]
	require.True(t, exists, "201 response not found for POST /v1/unleash")
	require.NotNil(t, created.Schema, "Schema not found for 201 response")

	// Verify it references handlers.Unleash (which is the Kubernetes CRD)
	expectedRef := "#/definitions/handlers.Unleash"
	assert.Equal(t, expectedRef, created.Schema.Ref, "Unexpected schema reference for POST /v1/unleash 201 response")

	// Check GET /v1/releasechannels 200 response
	releaseChannelsPath, exists := spec.Paths["/v1/releasechannels"]
	require.True(t, exists, "Path /v1/releasechannels not found")
	require.NotNil(t, releaseChannelsPath.Get, "GET /v1/releasechannels not found")

	ok, exists := releaseChannelsPath.Get.Responses["200"]
	require.True(t, exists, "200 response not found for GET /v1/releasechannels")
	require.NotNil(t, ok.Schema, "Schema not found for 200 response")

	// Verify it's an array of handlers.ReleaseChannelResponse
	assert.Equal(t, "array", ok.Schema.Type, "Expected array type for GET /v1/releasechannels response")
	require.NotNil(t, ok.Schema.Items, "Items not found for array response")
	assert.Equal(t, "#/definitions/handlers.ReleaseChannelResponse", ok.Schema.Items.Ref, "Unexpected items reference")
}

// TestHandlersUnleashIsKubernetesCRD validates that handlers.Unleash has the expected Kubernetes CRD structure
func TestHandlersUnleashIsKubernetesCRD(t *testing.T) {
	spec := fetchSwaggerSpec(t)

	def, ok := spec.Definitions["handlers.Unleash"]
	require.True(t, ok, "handlers.Unleash not found in swagger definitions")

	// Kubernetes CRD should have these standard fields
	expectedFields := []string{
		"apiVersion",
		"kind",
		"metadata",
		"spec",
		"status",
	}

	for _, field := range expectedFields {
		_, exists := def.Properties[field]
		assert.True(t, exists, "Expected Kubernetes CRD field %q not found in handlers.Unleash", field)
	}

	// Verify spec references the correct type
	specProp, exists := def.Properties["spec"]
	require.True(t, exists, "spec property not found")
	assert.Equal(t, "#/definitions/unleash_nais_io_v1.UnleashSpec", specProp.Ref, "Unexpected spec reference")

	// Verify status references the correct type
	statusProp, exists := def.Properties["status"]
	require.True(t, exists, "status property not found")
	assert.Equal(t, "#/definitions/unleash_nais_io_v1.UnleashStatus", statusProp.Ref, "Unexpected status reference")
}
