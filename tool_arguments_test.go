// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeToolArguments_IntegerConversionEdges(t *testing.T) {
	inputSchema := openapi3.NewObjectSchema()
	inputSchema.Properties = openapi3.Schemas{
		"whole":    openapi3.NewSchemaRef("", openapi3.NewIntegerSchema()),
		"fraction": openapi3.NewSchemaRef("", openapi3.NewIntegerSchema()),
		"unsafe":   openapi3.NewSchemaRef("", openapi3.NewIntegerSchema()),
		"number":   openapi3.NewSchemaRef("", openapi3.NewFloat64Schema()),
	}

	unsafeInteger := float64(maxSafeJSONInteger) + 1
	arguments := map[string]interface{}{
		"whole":    float64(42),
		"fraction": float64(1.25),
		"unsafe":   unsafeInteger,
		"number":   float64(7),
	}

	normalized := normalizeToolArguments(arguments, inputSchema)

	assert.Equal(t, 42, normalized["whole"])
	assert.IsType(t, 0, normalized["whole"])
	assert.Equal(t, float64(1.25), normalized["fraction"])
	assert.Equal(t, unsafeInteger, normalized["unsafe"])
	assert.Equal(t, float64(7), normalized["number"])

	assert.Equal(t, float64(42), arguments["whole"])
}
