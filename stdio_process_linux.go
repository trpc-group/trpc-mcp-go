//go:build linux

// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"os"
	"strconv"
)

func readStdioProcEntries() []stdioProcEntry {
	files, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	entries := make([]stdioProcEntry, 0, len(files))
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(file.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile("/proc/" + file.Name() + "/stat")
		if err != nil {
			continue
		}
		if entry, ok := parseStdioProcStat(pid, string(data)); ok {
			entries = append(entries, entry)
		}
	}
	return entries
}
