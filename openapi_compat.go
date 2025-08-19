package mcp

import (
	"reflect"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPICompat provides version compatibility for kin-openapi
// It handles the breaking change in v0.124.0 where Schema.Type changed from string to *Types
type OpenAPICompat struct {
	// Cache the type information to avoid repeated reflection
	once           sync.Once
	typeFieldType  reflect.Type
	isPointerType  bool
	typesSliceType reflect.Type
}

// Global instance for convenience
var compat = &OpenAPICompat{}

// initTypeInfo performs one-time reflection to determine the API version
func (c *OpenAPICompat) initTypeInfo() {
	c.once.Do(func() {
		schema := &openapi3.Schema{}
		v := reflect.ValueOf(schema).Elem()
		typeField := v.FieldByName("Type")

		if typeField.IsValid() {
			c.typeFieldType = typeField.Type()
			c.isPointerType = c.typeFieldType.Kind() == reflect.Ptr

			if c.isPointerType {
				// v0.124.0+: Type is *Types where Types is []string
				c.typesSliceType = c.typeFieldType.Elem()
			}
		}
	})
}

// SetSchemaTypeCompat sets the Type field of a schema in a version-compatible way
func (c *OpenAPICompat) SetSchemaTypeCompat(schema *openapi3.Schema, typeName string) {
	if schema == nil {
		return
	}

	// Initialize type info once
	c.initTypeInfo()

	v := reflect.ValueOf(schema).Elem()
	typeField := v.FieldByName("Type")

	if !typeField.IsValid() || !typeField.CanSet() {
		return
	}

	if c.isPointerType {
		// v0.124.0+: Type is *Types ([]string)
		typesValue := reflect.New(c.typesSliceType)
		typesSlice := reflect.MakeSlice(c.typesSliceType, 1, 1)
		typesSlice.Index(0).SetString(typeName)
		typesValue.Elem().Set(typesSlice)
		typeField.Set(typesValue)
	} else {
		// v0.123.0-: Type is string
		typeField.SetString(typeName)
	}
}

// CreateSchemaCompat creates a new schema with proper Type field
func (c *OpenAPICompat) CreateSchemaCompat(typeName string) *openapi3.Schema {
	schema := &openapi3.Schema{}
	c.SetSchemaTypeCompat(schema, typeName)
	return schema
}

// CreateObjectSchemaCompat creates a new object schema with properties
func (c *OpenAPICompat) CreateObjectSchemaCompat() *openapi3.Schema {
	schema := &openapi3.Schema{
		Properties: make(openapi3.Schemas),
		Required:   []string{},
	}
	c.SetSchemaTypeCompat(schema, openapi3.TypeObject)
	return schema
}

// Convenience functions for common types
func (c *OpenAPICompat) CreateStringSchemaCompat() *openapi3.Schema {
	return c.CreateSchemaCompat(openapi3.TypeString)
}

func (c *OpenAPICompat) CreateNumberSchemaCompat() *openapi3.Schema {
	return c.CreateSchemaCompat(openapi3.TypeNumber)
}

func (c *OpenAPICompat) CreateIntegerSchemaCompat() *openapi3.Schema {
	return c.CreateSchemaCompat(openapi3.TypeInteger)
}

func (c *OpenAPICompat) CreateBooleanSchemaCompat() *openapi3.Schema {
	return c.CreateSchemaCompat(openapi3.TypeBoolean)
}

func (c *OpenAPICompat) CreateArraySchemaCompat() *openapi3.Schema {
	return c.CreateSchemaCompat(openapi3.TypeArray)
}
