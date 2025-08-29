package openapi

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"trpc.group/trpc-go/trpc-mcp-go/internal/log"
)

// Compat provides version compatibility for kin-openapi
// It handles the breaking change in v0.124.0 where Schema.Type changed from string to *Types
type Compat struct {
	// Cache the type information to avoid repeated reflection
	once           sync.Once
	typeFieldType  reflect.Type
	isPointerType  bool
	typesSliceType reflect.Type
	initError      error // Track initialization errors
}

// Global instances for convenience
var (
	DefaultCompat = &Compat{}
	logger        = log.NewZapLogger()
)

// OpenAPI Compatibility Layer Error Handling:
//
// This compatibility layer handles kin-openapi version differences automatically.
// If you see ERROR logs mentioning "OpenAPI compatibility", it means:
//
// 1. There's a version mismatch between kin-openapi versions used by different libraries
// 2. The automatic compatibility failed - this usually requires framework developer attention
// 3. You should report this issue with the error message to the trpc-mcp-go maintainers
//
// Common causes:
// - Using an unsupported kin-openapi version (< v0.118.0 or > v0.132.0)
// - Conflicting dependencies pulling incompatible versions
// - Breaking changes in kin-openapi that we haven't adapted to yet

// initTypeInfo performs one-time reflection to determine the API version
func (c *Compat) initTypeInfo() {
	c.once.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				c.initError = fmt.Errorf("openapi compatibility init panic: %v", r)
				logger.Errorf("OpenAPI compatibility FAILED - please report this issue: %v", c.initError)
			}
		}()

		schema := &openapi3.Schema{}

		// Validate input
		if schema == nil {
			c.initError = fmt.Errorf("schema is nil")
			return
		}

		v := reflect.ValueOf(schema)
		if v.Kind() != reflect.Ptr || v.IsNil() {
			c.initError = fmt.Errorf("schema must be a non-nil pointer")
			return
		}

		elem := v.Elem()
		typeField := elem.FieldByName("Type")

		if !typeField.IsValid() {
			c.initError = fmt.Errorf("Type field not found in openapi3.Schema")
			logger.Errorf("OpenAPI compatibility FAILED: incompatible kin-openapi version detected - Schema.Type field missing")
			return
		}

		c.typeFieldType = typeField.Type()
		c.isPointerType = c.typeFieldType.Kind() == reflect.Ptr

		if c.isPointerType {
			// v0.124.0+: Type is *Types where Types is []string
			elemType := c.typeFieldType.Elem()
			if elemType.Kind() != reflect.Slice {
				c.initError = fmt.Errorf("expected *[]string for Type field, got %v", c.typeFieldType)
				logger.Errorf("OpenAPI compatibility FAILED: unexpected kin-openapi API structure change - Type field is %v, expected *[]string", c.typeFieldType)
				return
			}
			c.typesSliceType = elemType
		}

		// Initialization successful - no logging needed for normal operation
	})
}

// SetSchemaTypeCompat sets the Type field of a schema in a version-compatible way
func (c *Compat) SetSchemaType(schema *openapi3.Schema, typeName string) {
	// Input validation - silent return for user errors (these are not our bugs)
	if schema == nil {
		return
	}

	if typeName == "" {
		return
	}

	// Initialize type info once
	c.initTypeInfo()

	// Check for initialization errors
	if c.initError != nil {
		logger.Errorf("OpenAPI compatibility FAILED: %v", c.initError)
		return
	}

	// Safe reflection with panic recovery
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("OpenAPI compatibility PANIC: %v - this indicates a serious compatibility issue", r)
		}
	}()

	v := reflect.ValueOf(schema)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		// Silent return - this is a user error, not our bug
		return
	}

	elem := v.Elem()
	typeField := elem.FieldByName("Type")

	if !typeField.IsValid() {
		// Silent return - already logged during initialization
		return
	}

	if !typeField.CanSet() {
		// Silent return - reflection constraint, not our bug
		return
	}

	if c.isPointerType {
		// v0.124.0+: Type is *Types ([]string)
		if c.typesSliceType.Kind() == reflect.Invalid {
			// Silent return - already logged during initialization
			return
		}

		typesValue := reflect.New(c.typesSliceType)
		typesSlice := reflect.MakeSlice(c.typesSliceType, 1, 1)

		// Validate slice element type
		if typesSlice.Index(0).Kind() != reflect.String {
			// Silent return - this would be an internal error already caught in init
			return
		}

		typesSlice.Index(0).SetString(typeName)
		typesValue.Elem().Set(typesSlice)
		typeField.Set(typesValue)
	} else {
		// v0.123.0-: Type is string
		if typeField.Kind() != reflect.String {
			// Silent return - this would be an internal error already caught in init
			return
		}
		typeField.SetString(typeName)
	}
}

// CreateSchemaCompat creates a new schema with proper Type field
func (c *Compat) CreateSchema(typeName string) *openapi3.Schema {
	schema := &openapi3.Schema{}
	c.SetSchemaType(schema, typeName)
	return schema
}

// CreateObjectSchema creates a new object schema with properties
func (c *Compat) CreateObjectSchema() *openapi3.Schema {
	schema := &openapi3.Schema{
		Properties: make(openapi3.Schemas),
		Required:   []string{},
	}
	c.SetSchemaType(schema, openapi3.TypeObject)
	return schema
}

// Convenience functions for common types
func (c *Compat) CreateStringSchema() *openapi3.Schema {
	return c.CreateSchema(openapi3.TypeString)
}

func (c *Compat) CreateNumberSchema() *openapi3.Schema {
	return c.CreateSchema(openapi3.TypeNumber)
}

func (c *Compat) CreateIntegerSchema() *openapi3.Schema {
	return c.CreateSchema(openapi3.TypeInteger)
}

func (c *Compat) CreateBooleanSchema() *openapi3.Schema {
	return c.CreateSchema(openapi3.TypeBoolean)
}

func (c *Compat) CreateArraySchema() *openapi3.Schema {
	return c.CreateSchema(openapi3.TypeArray)
}
