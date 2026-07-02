//go:build unix

// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type stdioProcEntry struct {
	pid  int
	ppid int
	pgid int
}

func configureStdioProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stdioProcessGroupID(cmd *exec.Cmd) int {
	if cmd == nil || cmd.Process == nil {
		return 0
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return 0
	}
	return pgid
}

func stdioDescendantProcessGroupIDs(cmd *exec.Cmd) []int {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	entries := readStdioProcEntries()
	children := make(map[int][]stdioProcEntry)
	for _, entry := range entries {
		children[entry.ppid] = append(children[entry.ppid], entry)
	}

	seenPID := map[int]bool{cmd.Process.Pid: true}
	seenPGID := make(map[int]bool)
	queue := []int{cmd.Process.Pid}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		for _, child := range children[parent] {
			if seenPID[child.pid] {
				continue
			}
			seenPID[child.pid] = true
			queue = append(queue, child.pid)
			if child.pgid > 0 {
				seenPGID[child.pgid] = true
			}
		}
	}

	pgids := make([]int, 0, len(seenPGID))
	for pgid := range seenPGID {
		pgids = append(pgids, pgid)
	}
	return pgids
}

func signalStdioProcess(cmd *exec.Cmd, pgid int, signal os.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	sig, ok := signal.(syscall.Signal)
	if !ok {
		return cmd.Process.Signal(signal)
	}
	if pgid > 0 {
		if err := killProcessGroup(pgid, sig); err == nil {
			return nil
		}
	}
	return cmd.Process.Signal(signal)
}

func killStdioProcess(cmd *exec.Cmd, pgid int) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if pgid > 0 {
		if err := killProcessGroup(pgid, syscall.SIGKILL); err == nil {
			return nil
		}
	}
	return cmd.Process.Kill()
}

func killStdioProcessGroups(pgids []int) error {
	var errs []error
	seen := make(map[int]bool)
	for _, pgid := range pgids {
		if pgid <= 0 || seen[pgid] {
			continue
		}
		seen[pgid] = true
		if err := killProcessGroup(pgid, syscall.SIGKILL); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func killProcessGroup(pgid int, sig syscall.Signal) error {
	err := syscall.Kill(-pgid, sig)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

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

func parseStdioProcStat(pid int, stat string) (stdioProcEntry, bool) {
	end := strings.LastIndex(stat, ") ")
	if end < 0 || end+2 >= len(stat) {
		return stdioProcEntry{}, false
	}
	fields := strings.Fields(stat[end+2:])
	if len(fields) < 3 {
		return stdioProcEntry{}, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return stdioProcEntry{}, false
	}
	pgid, err := strconv.Atoi(fields[2])
	if err != nil {
		return stdioProcEntry{}, false
	}
	return stdioProcEntry{pid: pid, ppid: ppid, pgid: pgid}, true
}
