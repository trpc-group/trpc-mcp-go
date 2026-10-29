//go:build unix && !linux

// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseStdioPSOutput(t *testing.T) {
	output := "  42   1  42\n\t43\t42\t42\n\nPID PPID PGID\n44 42\n45 42 45 extra\nbad 1 1\n46 bad 46\n47 1 bad\n0 1 1\n48 -1 48\n49 1 0\n50 0 50\n"
	require.Equal(t, []stdioProcEntry{{pid: 42, ppid: 1, pgid: 42}, {pid: 43, ppid: 42, pgid: 42}, {pid: 50, ppid: 0, pgid: 50}}, parseStdioPSOutput(output))
	require.Empty(t, parseStdioPSOutput(""))
}

func TestStdioProcessEntriesWithoutPS(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	require.Empty(t, readStdioProcEntries())
}
