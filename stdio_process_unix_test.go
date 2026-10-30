//go:build unix

// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStdioProcessEntriesIncludeCurrentProcess(t *testing.T) {
	pgid, err := syscall.Getpgid(0)
	require.NoError(t, err)
	require.Contains(t, readStdioProcEntries(), stdioProcEntry{pid: os.Getpid(), ppid: os.Getppid(), pgid: pgid})
}

func TestStdioDescendantGroupsExcludeUnrelatedProcesses(t *testing.T) {
	entries := []stdioProcEntry{
		{pid: 10, ppid: 1, pgid: 10},
		{pid: 11, ppid: 10, pgid: 10}, // Shares the root group, cleaned separately.
		{pid: 12, ppid: 11, pgid: 12}, // Grandchild starts another group.
		{pid: 13, ppid: 12, pgid: 12}, // Repeated group must be deduplicated.
		{pid: 14, ppid: 13, pgid: 14},
		{pid: 20, ppid: 1, pgid: 20}, // Unrelated process tree.
		{pid: 21, ppid: 20, pgid: 21},
	}
	require.ElementsMatch(t, []int{12, 14}, stdioDescendantGroupIDs(10, entries))
	require.Empty(t, stdioDescendantGroupIDs(99, entries))
}

func TestParseStdioProcStat(t *testing.T) {
	entry, ok := parseStdioProcStat(42, "42 (name with ) parentheses) S 10 42 9 0")
	require.True(t, ok)
	require.Equal(t, stdioProcEntry{pid: 42, ppid: 10, pgid: 42}, entry)
	for _, raw := range []string{"", "42 missing delimiters", "42 (name) S", "42 (name) S bad 42", "42 (name) S 10 bad"} {
		_, ok := parseStdioProcStat(42, raw)
		require.False(t, ok)
	}
}
