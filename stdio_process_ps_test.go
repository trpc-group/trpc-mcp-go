//go:build unix && !linux

// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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

func TestStdioProcessEntriesBoundInheritedOutput(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	releaseFile := filepath.Join(dir, "release")
	t.Setenv("TRPC_MCP_PS_OUTPUT_HELPER", "parent")
	t.Setenv("TRPC_MCP_PS_OUTPUT_CHILD_PID", pidFile)
	t.Setenv("TRPC_MCP_PS_OUTPUT_RELEASE", releaseFile)
	script := "#!/bin/sh\nexec '" + strings.ReplaceAll(os.Args[0], "'", "'\"'\"'") +
		"' -test.run='^TestStdioPSOutputHelper$'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ps"), []byte(script), 0o700))
	t.Setenv("PATH", dir)

	result := make(chan []stdioProcEntry, 1)
	done := make(chan struct{})
	t.Cleanup(func() {
		_ = os.WriteFile(releaseFile, nil, 0o600)
		if raw, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(string(raw)); err == nil {
				if process, err := os.FindProcess(pid); err == nil {
					_ = process.Kill()
					_ = process.Release()
				}
			}
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("process scan did not return after its child was released")
		}
	})
	go func() {
		defer close(done)
		result <- readStdioProcEntries()
	}()

	select {
	case entries := <-result:
		require.Empty(t, entries)
	case <-time.After(5 * time.Second):
		t.Fatal("process scan did not bound its inherited output wait")
	}
	require.FileExists(t, pidFile)
}

func TestStdioPSOutputHelper(t *testing.T) {
	switch os.Getenv("TRPC_MCP_PS_OUTPUT_HELPER") {
	case "parent":
		child := exec.Command(os.Args[0], "-test.run=^TestStdioPSOutputHelper$")
		child.Env = append(os.Environ(), "TRPC_MCP_PS_OUTPUT_HELPER=child")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		require.NoError(t, child.Start())
		if err := os.WriteFile(os.Getenv("TRPC_MCP_PS_OUTPUT_CHILD_PID"),
			[]byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			_ = child.Process.Kill()
			_ = child.Wait()
			t.Fatal(err)
		}
		os.Exit(0)
	case "child":
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ticker.C:
				if _, err := os.Stat(os.Getenv("TRPC_MCP_PS_OUTPUT_RELEASE")); err == nil {
					return
				}
			case <-timer.C:
				return
			}
		}
	}
}
