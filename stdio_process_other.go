//go:build !unix

// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"os"
	"os/exec"
)

func configureStdioProcess(cmd *exec.Cmd) {}

func stdioProcessGroupID(cmd *exec.Cmd) int {
	return 0
}

func stdioDescendantProcessGroupIDs(cmd *exec.Cmd) []int {
	return nil
}

func signalStdioProcess(cmd *exec.Cmd, pgid int, signal os.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(signal)
}

func killStdioProcess(cmd *exec.Cmd, pgid int) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

func killStdioProcessGroups(pgids []int) error {
	return nil
}
