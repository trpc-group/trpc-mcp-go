//go:build unix && !linux

// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Non-Linux Unix systems do not necessarily provide /proc. Read only numeric
// process relationships, using portable ps columns without command arguments.
func readStdioProcEntries() []stdioProcEntry {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ps", "-A", "-o", "pid=,ppid=,pgid=").Output()
	if err != nil {
		return nil
	}
	return parseStdioPSOutput(string(output))
}

func parseStdioPSOutput(output string) []stdioProcEntry {
	var entries []stdioProcEntry
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil || ppid < 0 {
			continue
		}
		pgid, err := strconv.Atoi(fields[2])
		if err != nil || pgid <= 0 {
			continue
		}
		entries = append(entries, stdioProcEntry{pid: pid, ppid: ppid, pgid: pgid})
	}
	return entries
}
