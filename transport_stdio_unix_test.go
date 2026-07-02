//go:build unix

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
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStdioTransportCloseKillsBackgroundChild(t *testing.T) {
	sh, err := exec.LookPath("sh")
	require.NoError(t, err)

	pidFile := filepath.Join(t.TempDir(), "child.pid")
	script := "sleep 60 & echo $! > " + pidFile + "; wait"
	childPID := startStdioProcessTree(t, sh, script, pidFile)

	require.Eventually(t, func() bool {
		return !processExists(childPID)
	}, time.Second, 10*time.Millisecond)
}

func TestStdioTransportCloseKillsDescendantProcessGroup(t *testing.T) {
	sh, err := exec.LookPath("sh")
	require.NoError(t, err)

	pidFile := filepath.Join(t.TempDir(), "child.pid")
	script := "TRPC_MCP_STDIO_PG_CHILD=1 " +
		"TRPC_MCP_STDIO_PG_CHILD_PID_FILE=" + pidFile +
		" " + os.Args[0] +
		" -test.run=TestStdioProcessGroupChildHelper" +
		" & wait"
	childPID := startStdioProcessTree(t, sh, script, pidFile)

	require.Eventually(t, func() bool {
		return !processExists(childPID)
	}, time.Second, 10*time.Millisecond)
}

func startStdioProcessTree(
	t *testing.T,
	sh string,
	script string,
	pidFile string,
) int {
	t.Helper()

	transport := newStdioClientTransport(StdioServerParameters{
		Command: sh,
		Args:    []string{"-c", script},
	})

	require.NoError(t, transport.startProcess())

	var childPID int
	require.Eventually(t, func() bool {
		raw, readErr := os.ReadFile(pidFile)
		if readErr != nil {
			return false
		}
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
		if parseErr != nil {
			return false
		}
		childPID = pid
		return processExists(pid)
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, transport.close())
	return childPID
}

func TestStdioProcessGroupChildHelper(t *testing.T) {
	if os.Getenv("TRPC_MCP_STDIO_PG_CHILD") != "1" {
		return
	}
	require.NoError(t, syscall.Setpgid(0, 0))
	pidFile := os.Getenv("TRPC_MCP_STDIO_PG_CHILD_PID_FILE")
	require.NotEmpty(t, pidFile)
	err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600)
	require.NoError(t, err)
	time.Sleep(time.Minute)
	os.Exit(0)
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}
