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

// ConverterOptions controls schema generation behavior.
type ConverterOptions struct {
	// UseReferences determines whether to use $defs + $ref for type references.
	// When true (default), generates compact schemas with $defs and $ref.
	// When false, uses inline expansion with depth limit (legacy behavior).
	UseReferences bool

	// MaxInlineDepth is the maximum depth for inline expansion mode.
	// Only used when UseReferences is false. Default is 6.
	MaxInlineDepth int
}

// DefaultConverterOptions provides default configuration.
// Default uses $ref mode for compact, standard-compliant schemas.
var DefaultConverterOptions = ConverterOptions{
	UseReferences:  true, // Default: use $defs + $ref
	MaxInlineDepth: 6,
}

// Generator manages schema generation with $defs support.
//
// IMPORTANT: Generator is NOT safe for concurrent use.
// Create a new generator for each schema generation operation.
//
// The generator uses a placeholder pattern to handle circular references:
// 1. Mark type as visited
// 2. Create placeholder in $defs
// 3. Generate full schema
// 4. Replace placeholder with full schema
type Generator struct {
	options ConverterOptions
	defs    map[string]*openapi3.Schema // $defs storage
	visited map[reflect.Type]string     // Type → type name mapping
}

// NewGenerator creates a new generator with the given options.
func NewGenerator(options ConverterOptions) *Generator {
	return &Generator{
		options: options,
		defs:    make(map[string]*openapi3.Schema),
		visited: make(map[reflect.Type]string),
	}
}

// ConvertStructToOpenAPISchema converts a Go struct type to OpenAPI 3.0 Schema.
// Uses default options (UseReferences=true).
func ConvertStructToOpenAPISchema[T any]() *openapi3.Schema {
	return ConvertStructToOpenAPISchemaWithOptions[T](DefaultConverterOptions)
}

// ConvertStructToOpenAPISchemaWithOptions converts with custom options.
func ConvertStructToOpenAPISchemaWithOptions[T any](options ConverterOptions) *openapi3.Schema {
	var zero T
	t := reflect.TypeOf(zero)

	if options.UseReferences {
		// New mode: use $defs + $ref.
		gen := NewGenerator(options)
		schema := gen.generateWithRefs(t)
		// Add $defs to the schema.
		if len(gen.defs) > 0 {
			schema.Extensions = map[string]interface{}{
				"$defs": gen.defs,
			}
		}
		return schema
	}

	// Legacy mode: inline expansion with depth limit.
	visited := make(map[reflect.Type]*openapi3.Schema)
	return convertReflectTypeToSchemaWithVisited(t, visited)
}

// generateWithRefs generates schema with $defs + $ref support.
// This is the main entry point for the new reference-based mode.
func (g *Generator) generateWithRefs(t reflect.Type) *openapi3.Schema {
	// Dereference pointers.
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Check if already visited.
	if typeName, exists := g.visited[t]; exists {
		// Return schema with reference in extensions.
		schema := openapi3.NewObjectSchema()
		schema.Extensions = map[string]interface{}{
			"$ref": "#/$defs/" + typeName,
		}
		return schema
	}

	// For non-struct types, generate inline.
	if t.Kind() != reflect.Struct {
		return g.generateTypeSchemaWithRefs(t)
	}

	// For structs: use placeholder pattern.
	typeName := getTypeName(t)
	g.visited[t] = typeName

	// Create placeholder.
	placeholder := openapi3.NewObjectSchema()
	g.defs[typeName] = placeholder

	// Generate full schema.
	schema := g.generateStructSchemaWithRefs(t)
	g.defs[typeName] = schema

	// Return schema with reference in extensions.
	refSchema := openapi3.NewObjectSchema()
	refSchema.Extensions = map[string]interface{}{
		"$ref": "#/$defs/" + typeName,
	}
	return refSchema
}

// generateStructSchemaWithRefs generates schema for a struct type with $ref support.
func (g *Generator) generateStructSchemaWithRefs(t reflect.Type) *openapi3.Schema {
	schema := openapi3.NewObjectSchema()
	schema.Properties = make(openapi3.Schemas)
	var required []string
	requiredSet := make(map[string]bool)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		jsonName := getJSONFieldName(field)
		if jsonName == "" || jsonName == "-" {
			continue
		}

		// Generate field schema with $ref support.
		fieldSchema := g.generateFieldSchemaWithRefs(field.Type, field)

		if err := parseJSONSchemaTags(field.Tag, fieldSchema); err != nil {
			continue
		}

		schema.Properties[jsonName] = openapi3.NewSchemaRef("", fieldSchema)

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

// generateFieldSchemaWithRefs generates schema for a field type with $ref support.
func (g *Generator) generateFieldSchemaWithRefs(t reflect.Type, field reflect.StructField) *openapi3.Schema {
	// Dereference pointers.
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		// For structs, check if already visited.
		if typeName, exists := g.visited[t]; exists {
			// Return schema with reference in extensions.
			refSchema := openapi3.NewObjectSchema()
			refSchema.Extensions = map[string]interface{}{
				"$ref": "#/$defs/" + typeName,
			}
			return refSchema
		}
		// Otherwise, generate and add to defs.
		typeName := getTypeName(t)
		g.visited[t] = typeName
		placeholder := openapi3.NewObjectSchema()
		g.defs[typeName] = placeholder
		schema := g.generateStructSchemaWithRefs(t)
		g.defs[typeName] = schema
		refSchema := openapi3.NewObjectSchema()
		refSchema.Extensions = map[string]interface{}{
			"$ref": "#/$defs/" + typeName,
		}
		return refSchema

	case reflect.Slice, reflect.Array:
		elemSchema := g.generateFieldSchemaWithRefs(t.Elem(), field)
		arraySchema := openapi3.NewArraySchema()
		arraySchema.Items = openapi3.NewSchemaRef("", elemSchema)
		return arraySchema

	case reflect.Map:
		valueSchema := g.generateFieldSchemaWithRefs(t.Elem(), field)
		mapSchema := openapi3.NewObjectSchema()
		mapSchema.AdditionalProperties = openapi3.AdditionalProperties{
			Schema: openapi3.NewSchemaRef("", valueSchema),
		}
		return mapSchema

	default:
		// Primitive types.
		return convertPrimitiveType(t)
	}
}

// generateTypeSchemaWithRefs generates schema for non-struct types with $ref support.
func (g *Generator) generateTypeSchemaWithRefs(t reflect.Type) *openapi3.Schema {
	// Dereference pointers.
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		elemSchema := g.generateTypeSchemaWithRefs(t.Elem())
		arraySchema := openapi3.NewArraySchema()
		arraySchema.Items = openapi3.NewSchemaRef("", elemSchema)
		return arraySchema

	case reflect.Map:
		valueSchema := g.generateTypeSchemaWithRefs(t.Elem())
		mapSchema := openapi3.NewObjectSchema()
		mapSchema.AdditionalProperties = openapi3.AdditionalProperties{
			Schema: openapi3.NewSchemaRef("", valueSchema),
		}
		return mapSchema

	default:
		return convertPrimitiveType(t)
	}
}

// getTypeName returns a readable type name for use in $defs.
func getTypeName(t reflect.Type) string {
	if t.Name() != "" {
		// Use package name + type name for uniqueness.
		if t.PkgPath() != "" {
			// Strip common prefixes and keep last part.
			pkgParts := strings.Split(t.PkgPath(), "/")
			pkgName := pkgParts[len(pkgParts)-1]
			return pkgName + "." + t.Name()
		}
		return t.Name()
	}
	// Fallback for anonymous types.
	return fmt.Sprintf("Type%p", t)
}

// convertPrimitiveType converts primitive Go types to OpenAPI schemas.
func convertPrimitiveType(t reflect.Type) *openapi3.Schema {
	switch t.Kind() {
	case reflect.String:
		return openapi3.NewStringSchema()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return openapi3.NewIntegerSchema()
	case reflect.Float32, reflect.Float64:
		schema := openapi3.NewSchema()
		schema.Type = &openapi3.Types{"number"}
		return schema
	case reflect.Bool:
		return openapi3.NewBoolSchema()
	default:
		return openapi3.NewObjectSchema()
	}
}

// generateCycleRef generates a schema reference for circular/recursive types.
// This follows the kin-openapi/openapi3gen approach of using $ref to handle cycles.
func generateCycleRef(t reflect.Type) *openapi3.Schema {
	// Dereference pointer types
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		// For arrays, create array schema with ref to element type.
		elemRef := generateCycleRef(t.Elem())
		arraySchema := openapi3.NewArraySchema()
		arraySchema.Items = openapi3.NewSchemaRef("", elemRef)
		return arraySchema
	case reflect.Map:
		// For maps, create object schema with additionalProperties ref.
		valueRef := generateCycleRef(t.Elem())
		mapSchema := openapi3.NewObjectSchema()
		mapSchema.AdditionalProperties = openapi3.AdditionalProperties{
			Schema: openapi3.NewSchemaRef("", valueRef),
		}
		return mapSchema
	default:
		// For structs and other types, generate a recursive object reference.
		// Instead of using $ref (which requires components/schemas setup),
		// we recursively generate the schema with limited depth.
		// Increased depth to 6 to match inputSchema complexity (which goes 5-6 levels deep).
		return convertReflectTypeToSchemaWithDepth(t, 6)
	}
}

// convertReflectTypeToSchemaWithDepth converts with a depth limit to handle recursion.
func convertReflectTypeToSchemaWithDepth(t reflect.Type, maxDepth int) *openapi3.Schema {
	if maxDepth <= 0 {
		// At maximum depth, return a simple object schema.
		schema := openapi3.NewObjectSchema()
		schema.Description = fmt.Sprintf("Recursive reference to %s (depth limit reached)", t.Name())
		return schema
	}

	visited := make(map[reflect.Type]*openapi3.Schema)
	return convertStructToSchemaWithDepthLimit(t, visited, maxDepth)
}

// convertTypeWithDepthLimit converts any type with depth tracking.
func convertTypeWithDepthLimit(t reflect.Type, visited map[reflect.Type]*openapi3.Schema, depth int) *openapi3.Schema {
	if depth <= 0 {
		schema := openapi3.NewObjectSchema()
		schema.Description = "Depth limit reached"
		return schema
	}

	switch t.Kind() {
	case reflect.Struct:
		return convertStructToSchemaWithDepthLimit(t, visited, depth)
	case reflect.Slice, reflect.Array:
		schema := openapi3.NewArraySchema()
		elemSchema := convertTypeWithDepthLimit(t.Elem(), visited, depth-1)
		schema.Items = openapi3.NewSchemaRef("", elemSchema)
		return schema
	case reflect.Map:
		schema := openapi3.NewObjectSchema()
		valueSchema := convertTypeWithDepthLimit(t.Elem(), visited, depth-1)
		schema.AdditionalProperties = openapi3.AdditionalProperties{
			Schema: openapi3.NewSchemaRef("", valueSchema),
		}
		return schema
	case reflect.String:
		return openapi3.NewStringSchema()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return openapi3.NewIntegerSchema()
	case reflect.Float32, reflect.Float64:
		schema := openapi3.NewSchema()
		schema.Type = &openapi3.Types{"number"}
		return schema
	case reflect.Bool:
		return openapi3.NewBoolSchema()
	default:
		return openapi3.NewObjectSchema()
	}
}

// convertStructToSchemaWithDepthLimit converts a struct with depth tracking.
func convertStructToSchemaWithDepthLimit(t reflect.Type, visited map[reflect.Type]*openapi3.Schema, depth int) *openapi3.Schema {
	if depth <= 0 {
		schema := openapi3.NewObjectSchema()
		schema.Description = fmt.Sprintf("Recursive reference to %s (depth limit)", t.Name())
		return schema
	}

	schema := openapi3.NewObjectSchema()
	schema.Properties = make(openapi3.Schemas)
	var required []string
	requiredSet := make(map[string]bool)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if !field.IsExported() {
			continue
		}

		jsonName := getJSONFieldName(field)
		if jsonName == "" || jsonName == "-" {
			continue
		}

		// For recursive fields, decrease depth.
		fieldType := field.Type
		for fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}

		var fieldSchema *openapi3.Schema
		if fieldType == t {
			// Self-referencing: recurse with reduced depth.
			fieldSchema = convertStructToSchemaWithDepthLimit(fieldType, visited, depth-1)
		} else {
			// Different type: continue with depth-limited conversion to avoid infinite loops.
			fieldSchema = convertTypeWithDepthLimit(fieldType, visited, depth-1)
		}

		if err := parseJSONSchemaTags(field.Tag, fieldSchema); err != nil {
			continue
		}

		schema.Properties[jsonName] = openapi3.NewSchemaRef("", fieldSchema)

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

// convertReflectTypeToSchema converts a reflect.Type to openapi3.Schema.
func convertReflectTypeToSchema(t reflect.Type) *openapi3.Schema {
	visited := make(map[reflect.Type]*openapi3.Schema)
	return convertReflectTypeToSchemaWithVisited(t, visited)
}

// convertReflectTypeToSchemaWithVisited converts a reflect.Type to openapi3.Schema with cycle detection
func convertReflectTypeToSchemaWithVisited(t reflect.Type, visited map[reflect.Type]*openapi3.Schema) *openapi3.Schema {
	// Handle pointer types
	originalType := t
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Only check for cycles with struct types, as primitive types should always create new instances
	if t.Kind() == reflect.Struct {
		if schema, exists := visited[t]; exists {
			if schema != nil {
				return schema
			}
			// Generate recursive schema with depth limit for circular reference.
			refSchema := generateCycleRef(originalType)
			return refSchema
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
		schema.Type = &openapi3.Types{"number"}
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

	// More precise matching for "required" - it should be either:
	// 1. "required" (standalone)
	// 2. "required," or "required;" (with comma or semicolon)
	// 3. Start with "required," or "required;"
	// 4. Support both comma and semicolon separators
	if jsonschemaTag == "required" ||
		strings.HasPrefix(jsonschemaTag, "required,") ||
		strings.HasPrefix(jsonschemaTag, "required;") ||
		strings.Contains(jsonschemaTag, ",required,") ||
		strings.Contains(jsonschemaTag, ";required;") ||
		strings.HasSuffix(jsonschemaTag, ",required") ||
		strings.HasSuffix(jsonschemaTag, ";required") {
		return true
	}

	// If jsonschema tag exists but doesn't contain required, respect that
	if jsonschemaTag != "" {
		return false
	}

	// Check json tag for omitempty
	jsonTag := field.Tag.Get("json")
	if strings.Contains(jsonTag, "omitempty") {
		return false
	}

	// Non-pointer types are typically required (unless omitempty or explicitly specified in jsonschema)
	result := field.Type.Kind() != reflect.Ptr
	return result
}

// IsRequiredFieldForTest exports isRequiredField for testing
func IsRequiredFieldForTest(field reflect.StructField) bool {
	return isRequiredField(field)
}

// parseDirectives splits jsonschema tag by semicolon (new format) or comma (legacy format)
func parseDirectives(tag string) []string {
	// Check if using new semicolon-based format
	if strings.Contains(tag, ";") {
		parts := strings.Split(tag, ";")
		var directives []string
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				directives = append(directives, part)
			}
		}
		return directives
	}

	// Legacy comma-based format - handle description with commas
	var directives []string
	var current strings.Builder
	inDescription := false

	parts := strings.Split(tag, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		if strings.HasPrefix(part, "description=") {
			if current.Len() > 0 {
				directives = append(directives, current.String())
				current.Reset()
			}
			current.WriteString(part)
			inDescription = true
		} else if inDescription {
			// Continue building description until we hit a known directive
			if strings.Contains(part, "=") && (strings.HasPrefix(part, "title=") ||
				strings.HasPrefix(part, "minLength=") || strings.HasPrefix(part, "maxLength=") ||
				strings.HasPrefix(part, "minimum=") || strings.HasPrefix(part, "maximum=") ||
				strings.HasPrefix(part, "minItems=") || strings.HasPrefix(part, "maxItems=") ||
				strings.HasPrefix(part, "default=") || strings.HasPrefix(part, "enum=") ||
				part == "required" || part == "uniqueItems") {
				// This is a new directive, finish description
				directives = append(directives, current.String())
				current.Reset()
				current.WriteString(part)
				inDescription = false
			} else {
				// Continue description, preserve original spacing
				current.WriteString(", ")
				current.WriteString(part)
			}
		} else {
			if current.Len() > 0 {
				directives = append(directives, current.String())
				current.Reset()
			}
			current.WriteString(part)
		}
	}

	if current.Len() > 0 {
		directives = append(directives, current.String())
	}

	return directives
}

// parseJSONSchemaTags parses jsonschema struct tags and applies them to the schema
func parseJSONSchemaTags(tag reflect.StructTag, schema *openapi3.Schema) error {
	jsonschemaTag := tag.Get("jsonschema")
	if jsonschemaTag == "" {
		return nil
	}

	// Split by comma to get individual directives, but handle special cases like description with commas
	directives := parseDirectives(jsonschemaTag)

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
			case "title":
				schema.Title = value
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
			case "minItems":
				if minItems, err := strconv.ParseUint(value, 10, 64); err == nil {
					schema.MinItems = minItems
				}
			case "maxItems":
				if maxItems, err := strconv.ParseUint(value, 10, 64); err == nil {
					schema.MaxItems = &maxItems
				}
			case "enum":
				// Handle enum values (multiple enum=value directives)
				if schema.Enum == nil {
					schema.Enum = make([]any, 0)
				}
				// Add single enum value (standard format: enum=val1,enum=val2,enum=val3)
				schema.Enum = append(schema.Enum, value)
			case "default":
				// Convert default value based on schema type
				if schema.Type != nil && len(*schema.Type) > 0 {
					switch (*schema.Type)[0] {
					case "integer":
						if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
							schema.Default = int(intVal)
						} else {
							schema.Default = value // fallback to string
						}
					case "number":
						if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
							schema.Default = floatVal
						} else {
							schema.Default = value // fallback to string
						}
					case "boolean":
						if boolVal, err := strconv.ParseBool(value); err == nil {
							schema.Default = boolVal
						} else {
							schema.Default = value // fallback to string
						}
					default:
						schema.Default = value // string types
					}
				} else {
					schema.Default = value // fallback to string
				}
			case "example":
				schema.Example = value
			}
		} else if directive == "uniqueItems" {
			// Handle standalone uniqueItems directive (no value)
			schema.UniqueItems = true
		}
	}

	return nil
}
