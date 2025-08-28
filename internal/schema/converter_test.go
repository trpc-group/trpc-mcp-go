// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package schema

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// Test types for comprehensive validation

// Basic types test
type BasicTypesStruct struct {
	StringField  string  `json:"stringField"`
	IntField     int     `json:"intField"`
	Int32Field   int32   `json:"int32Field"`
	Int64Field   int64   `json:"int64Field"`
	Float32Field float32 `json:"float32Field"`
	Float64Field float64 `json:"float64Field"`
	BoolField    bool    `json:"boolField"`
}

// Pointer types test
type PointerTypesStruct struct {
	StringPtr  *string  `json:"stringPtr,omitempty"`
	IntPtr     *int     `json:"intPtr,omitempty"`
	Float64Ptr *float64 `json:"float64Ptr,omitempty"`
	BoolPtr    *bool    `json:"boolPtr,omitempty"`
}

// Array and slice types test
type ArraySliceStruct struct {
	StringArray [3]string          `json:"stringArray"`
	StringSlice []string           `json:"stringSlice"`
	IntSlice    []int              `json:"intSlice"`
	NestedSlice [][]string         `json:"nestedSlice"`
	StructSlice []BasicTypesStruct `json:"structSlice"`
}

// Map types test
type MapStruct struct {
	StringMap map[string]string            `json:"stringMap"`
	IntMap    map[string]int               `json:"intMap"`
	NestedMap map[string]map[string]string `json:"nestedMap"`
	StructMap map[string]BasicTypesStruct  `json:"structMap"`
}

// JSONSchema tags test
type JSONSchemaTagsStruct struct {
	Required    string `json:"required" jsonschema:"required,description=This field is required"`
	Optional    string `json:"optional,omitempty" jsonschema:"description=This field is optional"`
	WithEnum    string `json:"withEnum" jsonschema:"enum=option1,enum=option2,enum=option3,description=Enum field"`
	WithMinMax  int    `json:"withMinMax" jsonschema:"minimum=0,maximum=100,description=Number with constraints"`
	WithLength  string `json:"withLength" jsonschema:"minLength=5,maxLength=50,description=String with length constraints"`
	WithFormat  string `json:"withFormat" jsonschema:"format=email,description=Email format field"`
	WithPattern string `json:"withPattern" jsonschema:"pattern=^[A-Z]+$,description=Pattern field"`
	WithDefault string `json:"withDefault,omitempty" jsonschema:"default=defaultValue,description=Field with default"`
	WithExample string `json:"withExample" jsonschema:"example=exampleValue,description=Field with example"`
}

// Nested struct test
type NestedStruct struct {
	ID       string        `json:"id" jsonschema:"required,description=Unique identifier"`
	User     UserInfo      `json:"user" jsonschema:"required,description=User information"`
	Settings *UserSettings `json:"settings,omitempty" jsonschema:"description=Optional user settings"`
	Tags     []string      `json:"tags,omitempty" jsonschema:"description=User tags"`
}

type UserInfo struct {
	Name  string `json:"name" jsonschema:"required,description=User name"`
	Email string `json:"email" jsonschema:"required,format=email,description=User email"`
	Age   int    `json:"age,omitempty" jsonschema:"minimum=0,maximum=150,description=User age"`
}

type UserSettings struct {
	Theme         string `json:"theme,omitempty" jsonschema:"enum=light,enum=dark,default=light,description=UI theme"`
	Language      string `json:"language,omitempty" jsonschema:"enum=en,enum=zh,enum=es,default=en,description=Language preference"`
	Notifications bool   `json:"notifications,omitempty" jsonschema:"default=true,description=Enable notifications"`
}

// Complex nested structure test
type ComplexNestedStruct struct {
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Items    []NestedItem           `json:"items"`
	Config   NestedConfig           `json:"config"`
}

type NestedItem struct {
	ID       string            `json:"id" jsonschema:"required"`
	Type     string            `json:"type" jsonschema:"required,enum=type1,enum=type2"`
	Data     map[string]string `json:"data,omitempty"`
	Children []NestedItem      `json:"children,omitempty"`
}

type NestedConfig struct {
	Name     string                 `json:"name" jsonschema:"required"`
	Values   map[string]interface{} `json:"values,omitempty"`
	Settings []ConfigSetting        `json:"settings,omitempty"`
}

type ConfigSetting struct {
	Key   string `json:"key" jsonschema:"required"`
	Value string `json:"value" jsonschema:"required"`
}

// Test cases

func TestConvertStructToOpenAPISchema_BasicTypes(t *testing.T) {
	schema := ConvertStructToOpenAPISchema[BasicTypesStruct]()

	// Verify it's an object
	if schema.Type == nil || (*schema.Type)[0] != "object" {
		t.Errorf("Expected object type, got %v", schema.Type)
	}

	// Verify all fields are present
	expectedFields := []string{"stringField", "intField", "int32Field", "int64Field", "float32Field", "float64Field", "boolField"}
	for _, field := range expectedFields {
		if _, exists := schema.Properties[field]; !exists {
			t.Errorf("Expected field %s not found", field)
		}
	}

	// Verify field types
	assertFieldType(t, schema, "stringField", "string")
	assertFieldType(t, schema, "intField", "integer")
	assertFieldType(t, schema, "float32Field", "number")
	assertFieldType(t, schema, "boolField", "boolean")

	// All non-pointer fields should be required
	if len(schema.Required) != len(expectedFields) {
		t.Errorf("Expected %d required fields, got %d", len(expectedFields), len(schema.Required))
	}
}

func TestConvertStructToOpenAPISchema_PointerTypes(t *testing.T) {
	schema := ConvertStructToOpenAPISchema[PointerTypesStruct]()

	// Pointer fields should not be required (have omitempty)
	if len(schema.Required) != 0 {
		t.Errorf("Expected no required fields for pointer types, got %d", len(schema.Required))
	}

	// Verify field types are still correct
	assertFieldType(t, schema, "stringPtr", "string")
	assertFieldType(t, schema, "intPtr", "integer")
	assertFieldType(t, schema, "float64Ptr", "number")
	assertFieldType(t, schema, "boolPtr", "boolean")
}

func TestConvertStructToOpenAPISchema_ArraySlice(t *testing.T) {
	schema := ConvertStructToOpenAPISchema[ArraySliceStruct]()

	// Test string array
	arrayField := schema.Properties["stringArray"].Value
	if arrayField.Type == nil || (*arrayField.Type)[0] != "array" {
		t.Errorf("Expected array type for stringArray")
	}
	if arrayField.Items.Value.Type == nil || (*arrayField.Items.Value.Type)[0] != "string" {
		t.Errorf("Expected string items for stringArray")
	}

	// Test nested slice
	nestedField := schema.Properties["nestedSlice"].Value
	if nestedField.Type == nil || (*nestedField.Type)[0] != "array" {
		t.Errorf("Expected array type for nestedSlice")
	}
	if nestedField.Items.Value.Type == nil || (*nestedField.Items.Value.Type)[0] != "array" {
		t.Errorf("Expected array items for nestedSlice")
	}

	// Test struct slice
	structSliceField := schema.Properties["structSlice"].Value
	if structSliceField.Items.Value.Type == nil || (*structSliceField.Items.Value.Type)[0] != "object" {
		t.Errorf("Expected object items for structSlice")
	}
}

func TestConvertStructToOpenAPISchema_Maps(t *testing.T) {
	schema := ConvertStructToOpenAPISchema[MapStruct]()

	// Test string map
	stringMapField := schema.Properties["stringMap"].Value
	if stringMapField.Type == nil || (*stringMapField.Type)[0] != "object" {
		t.Errorf("Expected object type for stringMap")
	}
	if stringMapField.AdditionalProperties.Schema == nil {
		t.Errorf("Expected additionalProperties for stringMap")
	}
	if stringMapField.AdditionalProperties.Schema.Value.Type == nil ||
		(*stringMapField.AdditionalProperties.Schema.Value.Type)[0] != "string" {
		t.Errorf("Expected string additionalProperties for stringMap")
	}

	// Test nested map
	nestedMapField := schema.Properties["nestedMap"].Value
	if nestedMapField.AdditionalProperties.Schema.Value.Type == nil ||
		(*nestedMapField.AdditionalProperties.Schema.Value.Type)[0] != "object" {
		t.Errorf("Expected object additionalProperties for nestedMap")
	}
}

func TestConvertStructToOpenAPISchema_JSONSchemaTags(t *testing.T) {
	schema := ConvertStructToOpenAPISchema[JSONSchemaTagsStruct]()

	// Debug: print all properties
	for name, prop := range schema.Properties {
		t.Logf("Property %s: description='%s'", name, prop.Value.Description)
	}

	// Test required field
	if !contains(schema.Required, "required") {
		t.Errorf("Expected 'required' field to be in required list")
	}

	// Test descriptions
	actualDesc := schema.Properties["required"].Value.Description
	if actualDesc != "This field is required" {
		t.Errorf("Expected description for required field, got: '%s'", actualDesc)
	}

	// Test enum
	enumField := schema.Properties["withEnum"].Value
	expectedEnum := []interface{}{"option1", "option2", "option3"}
	if !reflect.DeepEqual(enumField.Enum, expectedEnum) {
		t.Errorf("Expected enum %v, got %v", expectedEnum, enumField.Enum)
	}

	// Test min/max
	minMaxField := schema.Properties["withMinMax"].Value
	if minMaxField.Min == nil || *minMaxField.Min != 0 {
		t.Errorf("Expected minimum 0")
	}
	if minMaxField.Max == nil || *minMaxField.Max != 100 {
		t.Errorf("Expected maximum 100")
	}

	// Test string length
	lengthField := schema.Properties["withLength"].Value
	if lengthField.MinLength != 5 {
		t.Errorf("Expected minLength 5")
	}
	if lengthField.MaxLength == nil || *lengthField.MaxLength != 50 {
		t.Errorf("Expected maxLength 50")
	}

	// Test format
	formatField := schema.Properties["withFormat"].Value
	if formatField.Format != "email" {
		t.Errorf("Expected format email")
	}

	// Test pattern
	patternField := schema.Properties["withPattern"].Value
	if patternField.Pattern != "^[A-Z]+$" {
		t.Errorf("Expected pattern ^[A-Z]+$")
	}

	// Test default
	defaultField := schema.Properties["withDefault"].Value
	if defaultField.Default != "defaultValue" {
		t.Errorf("Expected default defaultValue")
	}

	// Test example
	exampleField := schema.Properties["withExample"].Value
	if exampleField.Example != "exampleValue" {
		t.Errorf("Expected example exampleValue")
	}
}

func TestConvertStructToOpenAPISchema_NestedStruct(t *testing.T) {
	schema := ConvertStructToOpenAPISchema[NestedStruct]()

	// Test top-level required fields
	expectedRequired := []string{"id", "user"}
	for _, field := range expectedRequired {
		if !contains(schema.Required, field) {
			t.Errorf("Expected %s to be required", field)
		}
	}

	// Test nested struct
	userField := schema.Properties["user"].Value
	if userField.Type == nil || (*userField.Type)[0] != "object" {
		t.Errorf("Expected object type for user field")
	}

	// Test nested struct properties
	if _, exists := userField.Properties["name"]; !exists {
		t.Errorf("Expected name property in user field")
	}
	if _, exists := userField.Properties["email"]; !exists {
		t.Errorf("Expected email property in user field")
	}

	// Test nested required fields
	expectedUserRequired := []string{"name", "email"}
	for _, field := range expectedUserRequired {
		if !contains(userField.Required, field) {
			t.Errorf("Expected %s to be required in user struct", field)
		}
	}

	// Test nested optional pointer struct
	settingsField := schema.Properties["settings"].Value
	if settingsField.Type == nil || (*settingsField.Type)[0] != "object" {
		t.Errorf("Expected object type for settings field")
	}

	// Settings should not be required (it's a pointer with omitempty)
	if contains(schema.Required, "settings") {
		t.Errorf("Expected settings to not be required")
	}
}

func TestConvertStructToOpenAPISchema_ComplexNested(t *testing.T) {
	schema := ConvertStructToOpenAPISchema[ComplexNestedStruct]()

	// Test deeply nested structure
	itemsField := schema.Properties["items"].Value
	if itemsField.Type == nil || (*itemsField.Type)[0] != "array" {
		t.Errorf("Expected array type for items field")
	}

	itemSchema := itemsField.Items.Value
	if itemSchema.Type == nil || (*itemSchema.Type)[0] != "object" {
		t.Errorf("Expected object type for item elements")
	}

	// Test recursive structure (children field)
	if _, exists := itemSchema.Properties["children"]; !exists {
		t.Errorf("Expected children property in nested item")
	}

	childrenField := itemSchema.Properties["children"].Value
	if childrenField.Type == nil || (*childrenField.Type)[0] != "array" {
		t.Errorf("Expected array type for children field")
	}
}

func TestConvertStructToOpenAPISchema_SerializesToValidJSON(t *testing.T) {
	testCases := []interface{}{
		ConvertStructToOpenAPISchema[BasicTypesStruct](),
		ConvertStructToOpenAPISchema[PointerTypesStruct](),
		ConvertStructToOpenAPISchema[ArraySliceStruct](),
		ConvertStructToOpenAPISchema[MapStruct](),
		ConvertStructToOpenAPISchema[JSONSchemaTagsStruct](),
		ConvertStructToOpenAPISchema[NestedStruct](),
		ConvertStructToOpenAPISchema[ComplexNestedStruct](),
	}

	for i, schema := range testCases {
		_, err := json.Marshal(schema)
		if err != nil {
			t.Errorf("Test case %d: Failed to marshal schema to JSON: %v", i, err)
		}
	}
}

func TestConvertStructToOpenAPISchema_EmptyStruct(t *testing.T) {
	type EmptyStruct struct{}

	schema := ConvertStructToOpenAPISchema[EmptyStruct]()

	if schema.Type == nil || (*schema.Type)[0] != "object" {
		t.Errorf("Expected object type for empty struct")
	}

	if len(schema.Properties) != 0 {
		t.Errorf("Expected no properties for empty struct")
	}

	if len(schema.Required) != 0 {
		t.Errorf("Expected no required fields for empty struct")
	}
}

func TestConvertStructToOpenAPISchema_UnexportedFields(t *testing.T) {
	type StructWithUnexported struct {
		Public  string `json:"public"`
		private string `json:"private"` // Should be ignored
	}

	schema := ConvertStructToOpenAPISchema[StructWithUnexported]()

	if _, exists := schema.Properties["public"]; !exists {
		t.Errorf("Expected public field to be included")
	}

	if _, exists := schema.Properties["private"]; exists {
		t.Errorf("Expected private field to be excluded")
	}

	if len(schema.Properties) != 1 {
		t.Errorf("Expected only 1 property, got %d", len(schema.Properties))
	}
}

func TestConvertStructToOpenAPISchema_JSONTagHandling(t *testing.T) {
	type JSONTagStruct struct {
		Field1 string `json:"customName"`
		Field2 string `json:"-"`          // Should be ignored
		Field3 string `json:",omitempty"` // Should use field name
		Field4 string // Should use field name
	}

	schema := ConvertStructToOpenAPISchema[JSONTagStruct]()

	if _, exists := schema.Properties["customName"]; !exists {
		t.Errorf("Expected customName field")
	}

	if _, exists := schema.Properties["Field2"]; exists {
		t.Errorf("Expected Field2 to be ignored due to '-' tag")
	}

	if _, exists := schema.Properties["Field3"]; !exists {
		t.Errorf("Expected Field3 field")
	}

	if _, exists := schema.Properties["Field4"]; !exists {
		t.Errorf("Expected Field4 field")
	}
}

// Helper functions

func assertFieldType(t *testing.T, schema *openapi3.Schema, fieldName, expectedType string) {
	field, exists := schema.Properties[fieldName]
	if !exists {
		t.Errorf("Field %s not found", fieldName)
		return
	}

	if field.Value.Type == nil || (*field.Value.Type)[0] != expectedType {
		t.Errorf("Expected type %s for field %s, got %v", expectedType, fieldName, field.Value.Type)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestConvertStructToOpenAPISchema_EnumHandling(t *testing.T) {
	t.Run("MultipleEnumDirectives", func(t *testing.T) {
		type TestStruct struct {
			Status string `jsonschema:"enum=success,enum=error,enum=pending"`
		}
		schema := ConvertStructToOpenAPISchema[TestStruct]()

		property, exists := schema.Properties["Status"]
		if !exists {
			t.Fatal("Status field not found")
		}

		expected := []interface{}{"success", "error", "pending"}
		if len(property.Value.Enum) != len(expected) {
			t.Errorf("Enum length: got %d, want %d", len(property.Value.Enum), len(expected))
		}

		for i, exp := range expected {
			if property.Value.Enum[i] != exp {
				t.Errorf("Enum[%d]: got %v, want %v", i, property.Value.Enum[i], exp)
			}
		}
	})

	t.Run("StandardEnumFormat", func(t *testing.T) {
		type TestStruct struct {
			Priority string `jsonschema:"description=Task priority,enum=high,enum=medium,enum=low,required"`
		}
		schema := ConvertStructToOpenAPISchema[TestStruct]()

		property, exists := schema.Properties["Priority"]
		if !exists {
			t.Fatal("Priority field not found")
		}

		expected := []interface{}{"high", "medium", "low"}
		if len(property.Value.Enum) != len(expected) {
			t.Errorf("Enum length: got %d, want %d", len(property.Value.Enum), len(expected))
		}

		for i, exp := range expected {
			if property.Value.Enum[i] != exp {
				t.Errorf("Enum[%d]: got %v, want %v", i, property.Value.Enum[i], exp)
			}
		}

		// Check description
		if property.Value.Description != "Task priority" {
			t.Errorf("Description: got %q, want %q", property.Value.Description, "Task priority")
		}
	})

	t.Run("SingleEnum", func(t *testing.T) {
		type TestStruct struct {
			State string `jsonschema:"enum=active"`
		}
		schema := ConvertStructToOpenAPISchema[TestStruct]()

		property, exists := schema.Properties["State"]
		if !exists {
			t.Fatal("State field not found")
		}

		expected := []interface{}{"active"}
		if len(property.Value.Enum) != len(expected) {
			t.Errorf("Enum length: got %d, want %d", len(property.Value.Enum), len(expected))
		}

		if property.Value.Enum[0] != expected[0] {
			t.Errorf("Enum[0]: got %v, want %v", property.Value.Enum[0], expected[0])
		}
	})
}

func TestConvertStructToOpenAPISchema_EdgeCases(t *testing.T) {
	t.Run("InvalidNumericValues", func(t *testing.T) {
		type TestStruct struct {
			InvalidMin    int    `jsonschema:"minimum=invalid"`
			InvalidMax    int    `jsonschema:"maximum=not_a_number"`
			InvalidMinLen string `jsonschema:"minLength=abc"`
			InvalidMaxLen string `jsonschema:"maxLength=-1"`
		}
		schema := ConvertStructToOpenAPISchema[TestStruct]()

		// These fields should exist but without the invalid constraints
		minField := schema.Properties["InvalidMin"]
		if minField.Value.Min != nil {
			t.Error("Invalid minimum should be ignored")
		}

		maxField := schema.Properties["InvalidMax"]
		if maxField.Value.Max != nil {
			t.Error("Invalid maximum should be ignored")
		}

		minLenField := schema.Properties["InvalidMinLen"]
		if minLenField.Value.MinLength != 0 {
			t.Error("Invalid minLength should be ignored")
		}

		maxLenField := schema.Properties["InvalidMaxLen"]
		if maxLenField.Value.MaxLength != nil {
			t.Error("Invalid maxLength should be ignored")
		}
	})

	t.Run("EmptyAndMalformedTags", func(t *testing.T) {
		type TestStruct struct {
			EmptyTag       string `jsonschema:""`
			MalformedTag   string `jsonschema:"description"` // Missing =
			EmptyValue     string `jsonschema:"description="`
			MultipleEquals string `jsonschema:"description=val1=val2"`
			OnlyCommas     string `jsonschema:",,,"`
			TrailingComma  string `jsonschema:"description=test,"`
		}
		schema := ConvertStructToOpenAPISchema[TestStruct]()

		// All fields should exist without errors
		if len(schema.Properties) != 6 {
			t.Errorf("Expected 6 properties, got %d", len(schema.Properties))
		}

		// EmptyValue should have empty description
		emptyValField := schema.Properties["EmptyValue"]
		if emptyValField.Value.Description != "" {
			t.Errorf("EmptyValue description should be empty, got %q", emptyValField.Value.Description)
		}

		// MultipleEquals should use everything after first = as value
		multiField := schema.Properties["MultipleEquals"]
		if multiField.Value.Description != "val1=val2" {
			t.Errorf("MultipleEquals description should be 'val1=val2', got %q", multiField.Value.Description)
		}
	})

	t.Run("SpecialEnumValues", func(t *testing.T) {
		type TestStruct struct {
			EmptyEnum    string `jsonschema:"enum="`
			SpacesEnum   string `jsonschema:"enum= ,enum=  ,enum=value"`
			CommaInEnum  string `jsonschema:"enum=val1,val2,enum=val3"` // Only enum= directives should be parsed
			SpecialChars string `jsonschema:"enum=@#$%,enum=中文,enum=🎉"`
		}
		schema := ConvertStructToOpenAPISchema[TestStruct]()

		// EmptyEnum should have one empty enum value
		emptyField := schema.Properties["EmptyEnum"]
		if len(emptyField.Value.Enum) != 1 || emptyField.Value.Enum[0] != "" {
			t.Errorf("EmptyEnum should have one empty value, got %v", emptyField.Value.Enum)
		}

		// SpacesEnum should trim spaces and include non-empty values
		spacesField := schema.Properties["SpacesEnum"]
		expectedSpaces := []interface{}{"", "", "value"} // Empty strings should be preserved
		if len(spacesField.Value.Enum) != 3 {
			t.Errorf("SpacesEnum should have 3 values, got %d: %v", len(spacesField.Value.Enum), spacesField.Value.Enum)
		}
		for i, expected := range expectedSpaces {
			if spacesField.Value.Enum[i] != expected {
				t.Errorf("SpacesEnum[%d]: got %v, want %v", i, spacesField.Value.Enum[i], expected)
			}
		}

		// CommaInEnum should only parse valid enum= directives (val1 and val3)
		commaField := schema.Properties["CommaInEnum"]
		expectedComma := []interface{}{"val1", "val3"} // val2 is not a valid enum= directive
		if len(commaField.Value.Enum) != 2 {
			t.Errorf("CommaInEnum should have 2 values, got %d: %v", len(commaField.Value.Enum), commaField.Value.Enum)
		}
		for i, expected := range expectedComma {
			if commaField.Value.Enum[i] != expected {
				t.Errorf("CommaInEnum[%d]: got %v, want %v", i, commaField.Value.Enum[i], expected)
			}
		}

		// SpecialChars should handle Unicode and special characters
		specialField := schema.Properties["SpecialChars"]
		expectedSpecial := []interface{}{"@#$%", "中文", "🎉"}
		if len(specialField.Value.Enum) != 3 {
			t.Errorf("SpecialChars should have 3 values, got %d: %v", len(specialField.Value.Enum), specialField.Value.Enum)
		}
		for i, expected := range expectedSpecial {
			if specialField.Value.Enum[i] != expected {
				t.Errorf("SpecialChars[%d]: got %v, want %v", i, specialField.Value.Enum[i], expected)
			}
		}
	})
}

func TestConvertStructToOpenAPISchema_MoreEdgeCases(t *testing.T) {
	t.Run("FieldsWithoutJSONTags", func(t *testing.T) {
		type TestStruct struct {
			NoJSONTag     string // Should use field name
			ExportedField int    `jsonschema:"description=test"`
			unexported    string // Should be ignored
			Ignored       string `json:"-"` // Should be ignored
		}
		schema := ConvertStructToOpenAPISchema[TestStruct]()

		// Should only have NoJSONTag and ExportedField (unexported and json:"-" should be ignored)
		expectedFields := []string{"NoJSONTag", "ExportedField"}
		if len(schema.Properties) != 2 {
			t.Errorf("Expected 2 properties, got %d", len(schema.Properties))
			for name := range schema.Properties {
				t.Logf("Found property: %s", name)
			}
		}

		for _, field := range expectedFields {
			if _, exists := schema.Properties[field]; !exists {
				t.Errorf("Field %s should exist", field)
			}
		}

		// These should not exist
		if _, exists := schema.Properties["unexported"]; exists {
			t.Error("Unexported field should not exist")
		}
		if _, exists := schema.Properties["Ignored"]; exists {
			t.Error("Field with json:\"-\" should not exist")
		}
	})

	t.Run("RequiredFieldLogic", func(t *testing.T) {
		type TestStruct struct {
			ExplicitRequired string `jsonschema:"required"`
			WithOmitEmpty    string `json:",omitempty"`
			PointerField     *string
			NonPointerField  string
			BothTags         string `json:",omitempty" jsonschema:"required"` // jsonschema should override
		}
		schema := ConvertStructToOpenAPISchema[TestStruct]()

		expectedRequired := []string{"ExplicitRequired", "NonPointerField", "BothTags"}

		if len(schema.Required) != len(expectedRequired) {
			t.Errorf("Expected %d required fields, got %d: %v", len(expectedRequired), len(schema.Required), schema.Required)
		}

		for _, field := range expectedRequired {
			found := false
			for _, req := range schema.Required {
				if req == field {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Field %s should be required", field)
			}
		}
	})
}
