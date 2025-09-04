// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

// Package schema provides utilities for converting Go structs to OpenAPI schemas.
package schema

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// ConvertStructToOpenAPISchema converts a Go struct type to OpenAPI 3.0 Schema.
// It uses reflection to analyze the struct fields and their tags to generate
// a comprehensive schema that's compatible with the MCP protocol.
func ConvertStructToOpenAPISchema[T any]() *openapi3.Schema {
	var zero T
	visited := make(map[reflect.Type]*openapi3.Schema)
	return convertReflectTypeToSchemaWithVisited(reflect.TypeOf(zero), visited)
}

// convertReflectTypeToSchema converts a reflect.Type to openapi3.Schema
func convertReflectTypeToSchema(t reflect.Type) *openapi3.Schema {
	visited := make(map[reflect.Type]*openapi3.Schema)
	return convertReflectTypeToSchemaWithVisited(t, visited)
}

// convertReflectTypeToSchemaWithVisited converts a reflect.Type to openapi3.Schema with cycle detection
func convertReflectTypeToSchemaWithVisited(t reflect.Type, visited map[reflect.Type]*openapi3.Schema) *openapi3.Schema {
	// Handle pointer types
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Only check for cycles with struct types, as primitive types should always create new instances
	if t.Kind() == reflect.Struct {
		if schema, exists := visited[t]; exists {
			if schema != nil {
				return schema
			}
			// Create a placeholder to prevent infinite recursion
			placeholder := openapi3.NewObjectSchema()
			placeholder.Description = fmt.Sprintf("Circular reference to %s", t.String())
			return placeholder
		}
		// Mark this struct type as being processed
		visited[t] = nil
	}

	var schema *openapi3.Schema
	switch t.Kind() {
	case reflect.Struct:
		schema = convertStructToSchemaWithVisited(t, visited)
		// Update the visited map with the actual schema
		visited[t] = schema
	case reflect.Slice, reflect.Array:
		schema = convertArrayToSchemaWithVisited(t, visited)
	case reflect.Map:
		schema = convertMapToSchemaWithVisited(t, visited)
	case reflect.String:
		schema = openapi3.NewStringSchema()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema = openapi3.NewIntegerSchema()
	case reflect.Float32, reflect.Float64:
		schema = openapi3.NewSchema()
		schema.Type = "number"
	case reflect.Bool:
		schema = openapi3.NewBoolSchema()
	default:
		// Fallback to object type for unknown types
		schema = openapi3.NewObjectSchema()
	}

	return schema
}

// convertStructToSchema converts a struct type to OpenAPI object schema
func convertStructToSchema(t reflect.Type) *openapi3.Schema {
	visited := make(map[reflect.Type]*openapi3.Schema)
	return convertStructToSchemaWithVisited(t, visited)
}

// convertStructToSchemaWithVisited converts a struct type to OpenAPI object schema with cycle detection
func convertStructToSchemaWithVisited(t reflect.Type, visited map[reflect.Type]*openapi3.Schema) *openapi3.Schema {
	schema := openapi3.NewObjectSchema()
	schema.Properties = make(openapi3.Schemas)
	var required []string
	requiredSet := make(map[string]bool) // Track required fields to avoid duplicates

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JSON field name
		jsonName := getJSONFieldName(field)
		if jsonName == "" || jsonName == "-" {
			continue // Skip fields without JSON tags or explicitly ignored
		}

		// Convert field type to schema
		fieldSchema := convertReflectTypeToSchemaWithVisited(field.Type, visited)

		// Parse jsonschema tags and apply to schema
		if err := parseJSONSchemaTags(field.Tag, fieldSchema); err != nil {
			// Log error but continue processing
			continue
		}

		// Add to properties
		schema.Properties[jsonName] = openapi3.NewSchemaRef("", fieldSchema)

		// Check if field is required (avoid duplicates)
		if isRequiredField(field) && !requiredSet[jsonName] {
			required = append(required, jsonName)
			requiredSet[jsonName] = true
		}
	}

	if len(required) > 0 {
		schema.Required = required
	}

	return schema
}

// convertArrayToSchema converts slice/array types to OpenAPI array schema
func convertArrayToSchema(t reflect.Type) *openapi3.Schema {
	visited := make(map[reflect.Type]*openapi3.Schema)
	return convertArrayToSchemaWithVisited(t, visited)
}

// convertArrayToSchemaWithVisited converts slice/array types to OpenAPI array schema with cycle detection
func convertArrayToSchemaWithVisited(t reflect.Type, visited map[reflect.Type]*openapi3.Schema) *openapi3.Schema {
	schema := openapi3.NewArraySchema()
	itemSchema := convertReflectTypeToSchemaWithVisited(t.Elem(), visited)
	schema.Items = openapi3.NewSchemaRef("", itemSchema)
	return schema
}

// convertMapToSchema converts map types to OpenAPI object schema with additionalProperties
func convertMapToSchema(t reflect.Type) *openapi3.Schema {
	visited := make(map[reflect.Type]*openapi3.Schema)
	return convertMapToSchemaWithVisited(t, visited)
}

// convertMapToSchemaWithVisited converts map types to OpenAPI object schema with additionalProperties with cycle detection
func convertMapToSchemaWithVisited(t reflect.Type, visited map[reflect.Type]*openapi3.Schema) *openapi3.Schema {
	schema := openapi3.NewObjectSchema()

	// For maps, we set additionalProperties to the value type schema
	valueSchema := convertReflectTypeToSchemaWithVisited(t.Elem(), visited)
	schema.AdditionalProperties = openapi3.AdditionalProperties{
		Schema: openapi3.NewSchemaRef("", valueSchema),
	}

	return schema
}

// getJSONFieldName extracts the JSON field name from struct field
func getJSONFieldName(field reflect.StructField) string {
	jsonTag := field.Tag.Get("json")
	if jsonTag == "" {
		return field.Name
	}

	// Parse json tag (handle omitempty, etc.)
	parts := strings.Split(jsonTag, ",")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}

	return field.Name
}

// isRequiredField determines if a field is required based on its type and tags
func isRequiredField(field reflect.StructField) bool {
	// Check jsonschema tag for explicit required
	jsonschemaTag := field.Tag.Get("jsonschema")
	if strings.Contains(jsonschemaTag, "required") {
		return true
	}

	// Check json tag for omitempty
	jsonTag := field.Tag.Get("json")
	if strings.Contains(jsonTag, "omitempty") {
		return false
	}

	// Non-pointer types are typically required (unless omitempty)
	return field.Type.Kind() != reflect.Ptr
}

// parseJSONSchemaTags parses jsonschema struct tags and applies them to the schema
func parseJSONSchemaTags(tag reflect.StructTag, schema *openapi3.Schema) error {
	jsonschemaTag := tag.Get("jsonschema")
	if jsonschemaTag == "" {
		return nil
	}

	// Split by comma to get individual directives
	directives := strings.Split(jsonschemaTag, ",")

	for _, directive := range directives {
		directive = strings.TrimSpace(directive)

		if directive == "required" {
			// Required is handled at the parent level
			continue
		}

		// Handle key=value directives
		if strings.Contains(directive, "=") {
			parts := strings.SplitN(directive, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "description":
				schema.Description = value
			case "format":
				schema.Format = value
			case "pattern":
				schema.Pattern = value
			case "minimum":
				if min, err := strconv.ParseFloat(value, 64); err == nil {
					schema.Min = &min
				}
			case "maximum":
				if max, err := strconv.ParseFloat(value, 64); err == nil {
					schema.Max = &max
				}
			case "minLength":
				if minLen, err := strconv.ParseUint(value, 10, 64); err == nil {
					schema.MinLength = minLen
				}
			case "maxLength":
				if maxLen, err := strconv.ParseUint(value, 10, 64); err == nil {
					schema.MaxLength = &maxLen
				}
			case "enum":
				// Handle enum values (multiple enum=value directives)
				if schema.Enum == nil {
					schema.Enum = make([]any, 0)
				}
				// Add single enum value (standard format: enum=val1,enum=val2,enum=val3)
				schema.Enum = append(schema.Enum, value)
			case "default":
				schema.Default = value
			case "example":
				schema.Example = value
			}
		}
	}

	return nil
}
