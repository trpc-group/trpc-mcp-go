// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package middleware

// authInfoKeyType is an unexported empty struct used as a context key to prevent collisions with other packages
type authInfoKeyType struct{}

// AuthInfoKey is the context key for storing and retrieving authentication information on requests
// Use context.WithValue(ctx, AuthInfoKey, authInfo) to attach and ctx.Value(AuthInfoKey) to read
var AuthInfoKey = authInfoKeyType{}
