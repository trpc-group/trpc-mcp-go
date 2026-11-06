// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"encoding/json"
	"math"

	"github.com/getkin/kin-openapi/openapi3"
)

const maxSafeJSONInteger = 1<<53 - 1

func normalizeToolArguments(arguments map[string]interface{}, inputSchema *openapi3.Schema) map[string]interface{} {
	if len(arguments) == 0 || inputSchema == nil {
		return arguments
	}

	normalized := make(map[string]interface{}, len(arguments))
	for name, value := range arguments {
		normalized[name] = value
	}

	normalizeObjectProperties(normalized, inputSchema)
	return normalized
}

func normalizeArgumentValue(value interface{}, schemaRef *openapi3.SchemaRef) interface{} {
	if value == nil || schemaRef == nil || schemaRef.Value == nil {
		return value
	}

	schema := schemaRef.Value
	if schema.Type.Includes(openapi3.TypeInteger) {
		if intValue, ok := convertIntegerArgument(value); ok {
			return intValue
		}
		return value
	}
	if schema.Type.Includes(openapi3.TypeArray) || schema.Items != nil {
		return normalizeArrayArgument(value, schema.Items)
	}
	if schema.Type.Includes(openapi3.TypeObject) || len(schema.Properties) > 0 || schema.AdditionalProperties.Schema != nil {
		return normalizeObjectArgument(value, schema)
	}

	return value
}

func normalizeArrayArgument(value interface{}, itemSchema *openapi3.SchemaRef) interface{} {
	items, ok := value.([]interface{})
	if !ok {
		return value
	}

	normalized := make([]interface{}, len(items))
	for i, item := range items {
		normalized[i] = normalizeArgumentValue(item, itemSchema)
	}
	return normalized
}

func normalizeObjectArgument(value interface{}, schema *openapi3.Schema) interface{} {
	objectValue, ok := value.(map[string]interface{})
	if !ok {
		return value
	}

	normalized := make(map[string]interface{}, len(objectValue))
	for name, propertyValue := range objectValue {
		normalized[name] = propertyValue
	}
	normalizeObjectProperties(normalized, schema)
	return normalized
}

func normalizeObjectProperties(arguments map[string]interface{}, schema *openapi3.Schema) {
	if schema == nil {
		return
	}

	for name, propertySchema := range schema.Properties {
		value, ok := arguments[name]
		if !ok {
			continue
		}
		arguments[name] = normalizeArgumentValue(value, propertySchema)
	}

	additionalSchema := schema.AdditionalProperties.Schema
	if additionalSchema == nil {
		return
	}
	for name, value := range arguments {
		if _, ok := schema.Properties[name]; ok {
			continue
		}
		arguments[name] = normalizeArgumentValue(value, additionalSchema)
	}
}

func convertIntegerArgument(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		if !integerFitsInt(v) {
			return 0, false
		}
		return int(v), true
	case uint:
		if uint64(v) > uint64(math.MaxInt) {
			return 0, false
		}
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		if uint64(v) > uint64(math.MaxInt) {
			return 0, false
		}
		return int(v), true
	case uint64:
		if v > uint64(math.MaxInt) {
			return 0, false
		}
		return int(v), true
	case float32:
		return convertFloatInteger(float64(v))
	case float64:
		return convertFloatInteger(v)
	case json.Number:
		if intValue, err := v.Int64(); err == nil {
			if !integerFitsInt(intValue) {
				return 0, false
			}
			return int(intValue), true
		}
		floatValue, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return convertFloatInteger(floatValue)
	default:
		return 0, false
	}
}

func convertFloatInteger(value float64) (int, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	if value != math.Trunc(value) {
		return 0, false
	}
	if value < float64(math.MinInt) || value > float64(math.MaxInt) {
		return 0, false
	}
	if value < -maxSafeJSONInteger || value > maxSafeJSONInteger {
		return 0, false
	}
	return int(value), true
}

func integerFitsInt(value int64) bool {
	return value >= int64(math.MinInt) && value <= int64(math.MaxInt)
}
