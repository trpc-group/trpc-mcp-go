// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package schema

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// Test types for Ref mode tests

// Self-referencing type (circular reference)
type TreeNode struct {
	Value int       `json:"value"`
	Left  *TreeNode `json:"left,omitempty"`
	Right *TreeNode `json:"right,omitempty"`
}

// Map with self-referencing value
type NestedMap struct {
	Name     string               `json:"name"`
	Children map[string]NestedMap `json:"children,omitempty"`
}

// Complex circular reference
type LinkedListNode struct {
	Data string          `json:"data"`
	Next *LinkedListNode `json:"next,omitempty"`
}

// Note: EmptyStruct and JSONSchemaTagsStruct are defined in converter_test.go

// Helper functions for Ref mode tests

// Helper to get $defs from schema
func getDefs(t *testing.T, schema *openapi3.Schema) map[string]*openapi3.Schema {
	t.Helper()
	if schema.Extensions == nil {
		t.Fatal("Expected Extensions to contain $defs")
	}
	defs, ok := schema.Extensions["$defs"].(map[string]*openapi3.Schema)
	if !ok {
		t.Fatalf("Expected $defs to be map[string]*openapi3.Schema, got %T", schema.Extensions["$defs"])
	}
	return defs
}

// Helper to check if type exists in $defs
func assertDefExists(t *testing.T, defs map[string]*openapi3.Schema, typeName string) *openapi3.Schema {
	t.Helper()
	def, exists := defs[typeName]
	if !exists {
		t.Fatalf("Expected type %s in $defs, available types: %v", typeName, getDefKeys(defs))
	}
	return def
}

// Helper to get keys from defs
func getDefKeys(defs map[string]*openapi3.Schema) []string {
	keys := make([]string, 0, len(defs))
	for k := range defs {
		keys = append(keys, k)
	}
	return keys
}

// Helper to check $ref in extensions
func assertHasRef(t *testing.T, schema *openapi3.Schema, expectedRef string) {
	t.Helper()
	if schema.Extensions == nil {
		t.Fatal("Expected Extensions to contain $ref")
	}
	ref, ok := schema.Extensions["$ref"].(string)
	if !ok {
		t.Fatalf("Expected $ref to be string, got %T", schema.Extensions["$ref"])
	}
	if ref != expectedRef {
		t.Fatalf("Expected $ref to be %s, got %s", expectedRef, ref)
	}
}

// Test cases - Ref Mode

// TestRefMode_BasicTypes tests basic type conversion with $defs + $ref
func TestRefMode_BasicTypes(t *testing.T) {
	schema := convertRefMode[BasicTypesStruct]()

	// 1. Verify top-level structure
	defs := getDefs(t, schema)

	// 2. Verify BasicTypesStruct definition exists
	typeName := "schema.BasicTypesStruct"
	def := assertDefExists(t, defs, typeName)

	// 3. Verify properties
	if def.Properties == nil {
		t.Fatal("Expected properties in type definition")
	}

	expectedFields := map[string]string{
		"stringField":  "string",
		"intField":     "integer",
		"int32Field":   "integer",
		"int64Field":   "integer",
		"float32Field": "number",
		"float64Field": "number",
		"boolField":    "boolean",
	}

	for fieldName, expectedType := range expectedFields {
		propRef, exists := def.Properties[fieldName]
		if !exists {
			t.Errorf("Expected field %s not found in properties", fieldName)
			continue
		}
		prop := propRef.Value
		if prop.Type == nil || len(*prop.Type) == 0 {
			t.Errorf("Field %s has no type", fieldName)
			continue
		}
		actualType := (*prop.Type)[0]
		if actualType != expectedType {
			t.Errorf("Field %s: expected type %s, got %s", fieldName, expectedType, actualType)
		}
	}

	// 4. Verify required fields
	if len(def.Required) != len(expectedFields) {
		t.Errorf("Expected %d required fields, got %d: %v", len(expectedFields), len(def.Required), def.Required)
	}
}

// TestRefMode_NestedTypes tests nested types with $defs + $ref
func TestRefMode_NestedTypes(t *testing.T) {
	schema := convertRefMode[NestedStruct]()

	// 1. Get $defs
	defs := getDefs(t, schema)

	// 2. Verify all types are defined (NestedStruct uses UserInfo and UserSettings)
	nestedDef := assertDefExists(t, defs, "schema.NestedStruct")
	_ = assertDefExists(t, defs, "schema.UserInfo")
	_ = assertDefExists(t, defs, "schema.UserSettings")

	// 3. Verify NestedStruct has a field referencing UserInfo
	userFieldRef, exists := nestedDef.Properties["user"]
	if !exists {
		t.Fatal("Expected 'user' field in NestedStruct")
	}

	userField := userFieldRef.Value
	// Should have $ref in extensions
	if userField.Extensions == nil {
		t.Fatal("Expected Extensions in user field for $ref")
	}

	ref, ok := userField.Extensions["$ref"]
	if !ok {
		t.Fatal("Expected $ref in user field extensions")
	}

	expectedRef := "#/$defs/schema.UserInfo"
	if ref != expectedRef {
		t.Errorf("Expected $ref to be %s, got %s", expectedRef, ref)
	}

	// 4. Verify settings field references UserSettings
	settingsFieldRef, exists := nestedDef.Properties["settings"]
	if !exists {
		t.Fatal("Expected 'settings' field in NestedStruct")
	}

	settingsField := settingsFieldRef.Value
	if settingsField.Extensions == nil {
		t.Fatal("Expected Extensions in settings field for $ref")
	}

	ref2, ok := settingsField.Extensions["$ref"]
	if !ok {
		t.Fatal("Expected $ref in settings field extensions")
	}

	expectedRef2 := "#/$defs/schema.UserSettings"
	if ref2 != expectedRef2 {
		t.Errorf("Expected $ref to be %s, got %s", expectedRef2, ref2)
	}
}

// TestRefMode_CircularReference tests self-referencing types
func TestRefMode_CircularReference(t *testing.T) {
	schema := convertRefMode[TreeNode]()

	// 1. Get $defs
	defs := getDefs(t, schema)

	// 2. Verify TreeNode is defined only once
	treeDef := assertDefExists(t, defs, "schema.TreeNode")

	// 3. Verify left and right fields reference TreeNode
	for _, fieldName := range []string{"left", "right"} {
		fieldRef, exists := treeDef.Properties[fieldName]
		if !exists {
			t.Fatalf("Expected '%s' field in TreeNode", fieldName)
		}

		field := fieldRef.Value
		if field.Extensions == nil {
			t.Fatalf("Expected Extensions in %s field", fieldName)
		}

		ref, ok := field.Extensions["$ref"]
		if !ok {
			t.Fatalf("Expected $ref in %s field", fieldName)
		}

		expectedRef := "#/$defs/schema.TreeNode"
		if ref != expectedRef {
			t.Errorf("Field %s: expected $ref to be %s, got %s", fieldName, expectedRef, ref)
		}
	}

	// 4. Verify value field is integer
	valueRef, exists := treeDef.Properties["value"]
	if !exists {
		t.Fatal("Expected 'value' field in TreeNode")
	}

	value := valueRef.Value
	if value.Type == nil || len(*value.Type) == 0 || (*value.Type)[0] != "integer" {
		t.Errorf("Expected 'value' field to be integer")
	}
}

// TestRefMode_MapWithCircularReference tests map with self-referencing values
func TestRefMode_MapWithCircularReference(t *testing.T) {
	schema := convertRefMode[NestedMap]()

	// 1. Get $defs
	defs := getDefs(t, schema)

	// 2. Verify NestedMap is defined
	mapDef := assertDefExists(t, defs, "schema.NestedMap")

	// 3. Verify children field is a map
	childrenRef, exists := mapDef.Properties["children"]
	if !exists {
		t.Fatal("Expected 'children' field in NestedMap")
	}

	children := childrenRef.Value
	if children.Type == nil || len(*children.Type) == 0 || (*children.Type)[0] != "object" {
		t.Fatal("Expected 'children' field to be object (map)")
	}

	// 4. Verify additionalProperties has $ref to NestedMap
	if children.AdditionalProperties.Schema == nil {
		t.Fatal("Expected AdditionalProperties.Schema for map values")
	}

	valueSchema := children.AdditionalProperties.Schema.Value
	if valueSchema.Extensions == nil {
		t.Fatal("Expected Extensions in map value schema")
	}

	ref, ok := valueSchema.Extensions["$ref"]
	if !ok {
		t.Fatal("Expected $ref in map value schema")
	}

	expectedRef := "#/$defs/schema.NestedMap"
	if ref != expectedRef {
		t.Errorf("Expected $ref to be %s, got %s", expectedRef, ref)
	}
}

// TestRefMode_LinkedList tests linked list pattern
func TestRefMode_LinkedList(t *testing.T) {
	schema := convertRefMode[LinkedListNode]()

	// 1. Get $defs
	defs := getDefs(t, schema)

	// 2. Verify LinkedListNode is defined
	nodeDef := assertDefExists(t, defs, "schema.LinkedListNode")

	// 3. Verify data field
	dataRef, exists := nodeDef.Properties["data"]
	if !exists {
		t.Fatal("Expected 'data' field in LinkedListNode")
	}

	data := dataRef.Value
	if data.Type == nil || len(*data.Type) == 0 || (*data.Type)[0] != "string" {
		t.Error("Expected 'data' field to be string")
	}

	// 4. Verify next field references LinkedListNode
	nextRef, exists := nodeDef.Properties["next"]
	if !exists {
		t.Fatal("Expected 'next' field in LinkedListNode")
	}

	next := nextRef.Value
	assertHasRef(t, next, "#/$defs/schema.LinkedListNode")
}

// TestRefMode_ComplexNesting tests deeply nested structures
func TestRefMode_ComplexNesting(t *testing.T) {
	schema := convertRefMode[ComplexNestedStruct]()

	// 1. Get $defs
	defs := getDefs(t, schema)

	// 2. Verify all types are defined
	// ComplexNestedStruct contains: NestedItem (self-referencing), NestedConfig, ConfigSetting
	expectedTypes := []string{
		"schema.ComplexNestedStruct",
		"schema.NestedItem",
		"schema.NestedConfig",
		"schema.ConfigSetting",
	}

	for _, typeName := range expectedTypes {
		assertDefExists(t, defs, typeName)
	}

	// 3. Verify no duplicate definitions
	if len(defs) != len(expectedTypes) {
		t.Errorf("Expected %d type definitions, got %d. Types: %v", len(expectedTypes), len(defs), getDefKeys(defs))
	}
}

// TestRefMode_ArrayOfStructs tests arrays with struct elements
func TestRefMode_ArrayOfStructs(t *testing.T) {
	schema := convertRefMode[ArraySliceStruct]()

	// 1. Get $defs
	defs := getDefs(t, schema)

	// 2. Verify ArraySliceStruct and BasicTypesStruct are defined
	arrayDef := assertDefExists(t, defs, "schema.ArraySliceStruct")
	_ = assertDefExists(t, defs, "schema.BasicTypesStruct")

	// 3. Verify structSlice field
	structSliceRef, exists := arrayDef.Properties["structSlice"]
	if !exists {
		t.Fatal("Expected 'structSlice' field in ArraySliceStruct")
	}

	structSlice := structSliceRef.Value
	if structSlice.Type == nil || len(*structSlice.Type) == 0 || (*structSlice.Type)[0] != "array" {
		t.Fatal("Expected 'structSlice' field to be array")
	}

	// 4. Verify array items reference BasicTypesStruct
	if structSlice.Items == nil || structSlice.Items.Value == nil {
		t.Fatal("Expected Items in structSlice")
	}

	items := structSlice.Items.Value
	assertHasRef(t, items, "#/$defs/schema.BasicTypesStruct")
}

// TestRefMode_MapOfStructs tests maps with struct values
func TestRefMode_MapOfStructs(t *testing.T) {
	schema := convertRefMode[MapStruct]()

	// 1. Get $defs
	defs := getDefs(t, schema)

	// 2. Verify both types are defined
	mapDef := assertDefExists(t, defs, "schema.MapStruct")
	_ = assertDefExists(t, defs, "schema.BasicTypesStruct")

	// 3. Verify structMap field
	structMapRef, exists := mapDef.Properties["structMap"]
	if !exists {
		t.Fatal("Expected 'structMap' field in MapStruct")
	}

	structMap := structMapRef.Value
	if structMap.Type == nil || len(*structMap.Type) == 0 || (*structMap.Type)[0] != "object" {
		t.Fatal("Expected 'structMap' field to be object (map)")
	}

	// 4. Verify map values reference BasicTypesStruct
	if structMap.AdditionalProperties.Schema == nil {
		t.Fatal("Expected AdditionalProperties.Schema for map values")
	}

	valueSchema := structMap.AdditionalProperties.Schema.Value
	assertHasRef(t, valueSchema, "#/$defs/schema.BasicTypesStruct")
}

// TestRefMode_PointerTypes tests pointer types with refs
func TestRefMode_PointerTypes(t *testing.T) {
	schema := convertRefMode[PointerTypesStruct]()

	// 1. Get $defs
	defs := getDefs(t, schema)

	// 2. Verify PointerTypesStruct is defined
	ptrDef := assertDefExists(t, defs, "schema.PointerTypesStruct")

	// 3. Verify pointer fields have correct types (pointers are dereferenced)
	expectedFields := map[string]string{
		"stringPtr":  "string",
		"intPtr":     "integer",
		"float64Ptr": "number",
		"boolPtr":    "boolean",
	}

	for fieldName, expectedType := range expectedFields {
		fieldRef, exists := ptrDef.Properties[fieldName]
		if !exists {
			t.Errorf("Expected field %s not found", fieldName)
			continue
		}

		field := fieldRef.Value
		if field.Type == nil || len(*field.Type) == 0 {
			t.Errorf("Field %s has no type", fieldName)
			continue
		}

		actualType := (*field.Type)[0]
		if actualType != expectedType {
			t.Errorf("Field %s: expected type %s, got %s", fieldName, expectedType, actualType)
		}
	}

	// 4. Verify no fields are required (all have omitempty)
	if len(ptrDef.Required) != 0 {
		t.Errorf("Expected no required fields for pointer types, got %d: %v", len(ptrDef.Required), ptrDef.Required)
	}
}

// TestRefMode_EmptyStruct tests empty struct
func TestRefMode_EmptyStruct(t *testing.T) {
	// Define EmptyStruct locally for this test
	type EmptyStruct struct{}

	schema := convertRefMode[EmptyStruct]()

	// 1. Get $defs
	defs := getDefs(t, schema)

	// 2. Verify EmptyStruct is defined
	emptyDef := assertDefExists(t, defs, "schema.EmptyStruct")

	// 3. Verify it's an object with no properties
	if emptyDef.Type == nil || len(*emptyDef.Type) == 0 || (*emptyDef.Type)[0] != "object" {
		t.Error("Expected EmptyStruct to be object")
	}

	if len(emptyDef.Properties) != 0 {
		t.Errorf("Expected no properties in EmptyStruct, got %d", len(emptyDef.Properties))
	}

	if len(emptyDef.Required) != 0 {
		t.Errorf("Expected no required fields in EmptyStruct, got %d", len(emptyDef.Required))
	}
}

// TestRefMode_JSONSchemaTags tests jsonschema tags with ref mode
func TestRefMode_JSONSchemaTags(t *testing.T) {
	schema := convertRefMode[JSONSchemaTagsStruct]()

	// 1. Get $defs
	defs := getDefs(t, schema)

	// 2. Verify type is defined
	tagsDef := assertDefExists(t, defs, "schema.JSONSchemaTagsStruct")

	// 3. Verify field with format (withFormat: email)
	formatRef, exists := tagsDef.Properties["withFormat"]
	if !exists {
		t.Fatal("Expected 'withFormat' field")
	}

	formatField := formatRef.Value
	if formatField.Description != "Email format field" {
		t.Errorf("Expected description 'Email format field', got '%s'", formatField.Description)
	}
	if formatField.Format != "email" {
		t.Errorf("Expected format 'email', got '%s'", formatField.Format)
	}

	// 4. Verify field with enum (withEnum)
	enumRef, exists := tagsDef.Properties["withEnum"]
	if !exists {
		t.Fatal("Expected 'withEnum' field")
	}

	enumField := enumRef.Value
	if len(enumField.Enum) != 3 {
		t.Errorf("Expected 3 enum values, got %d", len(enumField.Enum))
	}

	// 5. Verify field with minimum/maximum (withMinMax)
	minMaxRef, exists := tagsDef.Properties["withMinMax"]
	if !exists {
		t.Fatal("Expected 'withMinMax' field")
	}

	minMaxField := minMaxRef.Value
	if minMaxField.Min == nil || *minMaxField.Min != 0 {
		t.Errorf("Expected minimum 0, got %v", minMaxField.Min)
	}
	if minMaxField.Max == nil || *minMaxField.Max != 100 {
		t.Errorf("Expected maximum 100, got %v", minMaxField.Max)
	}
}

// TestRefMode_DefsDeduplication verifies no duplicate type definitions
func TestRefMode_DefsDeduplication(t *testing.T) {
	// Type that uses BasicTypesStruct in multiple places
	type MultipleReferences struct {
		First  BasicTypesStruct `json:"first"`
		Second BasicTypesStruct `json:"second"`
		Third  BasicTypesStruct `json:"third"`
	}

	schema := convertRefMode[MultipleReferences]()

	// 1. Get $defs
	defs := getDefs(t, schema)

	// 2. Verify BasicTypesStruct is defined only once
	expectedTypes := []string{
		"schema.MultipleReferences",
		"schema.BasicTypesStruct",
	}

	if len(defs) != len(expectedTypes) {
		t.Errorf("Expected %d type definitions (no duplicates), got %d. Types: %v",
			len(expectedTypes), len(defs), getDefKeys(defs))
	}

	// 3. Verify all fields reference the same definition
	multiDef := defs["schema.MultipleReferences"]
	for _, fieldName := range []string{"first", "second", "third"} {
		fieldRef, exists := multiDef.Properties[fieldName]
		if !exists {
			t.Fatalf("Expected field %s", fieldName)
		}

		field := fieldRef.Value
		assertHasRef(t, field, "#/$defs/schema.BasicTypesStruct")
	}
}
