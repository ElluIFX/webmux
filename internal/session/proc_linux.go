//go:build linux

package session

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type processState struct {
	ppid  int
	state byte
}
type processTable map[int]processState

// Read once per probe, not once per session. Commands may include spaces or ')'.
func processes() processTable {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	table := make(processTable)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}
		end := strings.LastIndexByte(string(data), ')')
		if end < 0 {
			continue
		}
		fields := strings.Fields(string(data[end+1:]))
		if len(fields) < 2 || len(fields[0]) != 1 {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err == nil {
			table[pid] = processState{ppid, fields[0][0]}
		}
	}
	return table
}
func foregroundBlocked(pid int, table processTable) bool {
	root, ok := table[pid]
	if !ok {
		return false
	}
	if root.state == 'R' {
		return false
	}
	tree := map[int]bool{pid: true}
	for changed := true; changed; {
		changed = false
		for child, state := range table {
			if !tree[child] && tree[state.ppid] {
				tree[child] = true
				changed = true
			}
		}
	}
	leaves := 0
	for member := range tree {
		state := table[member]
		if state.state == 'R' {
			return false
		}
		leaf := true
		for child := range tree {
			if table[child].ppid == member {
				leaf = false
				break
			}
		}
		if leaf {
			leaves++
			if state.state != 'S' {
				return false
			}
		}
	}
	return leaves > 0
}
