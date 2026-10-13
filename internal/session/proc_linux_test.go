//go:build linux

package session

import "testing"

func TestProcessTreeWaiting(t *testing.T) {
	table := processTable{1: {ppid: 0, state: 'S'}, 2: {ppid: 1, state: 'S'}, 3: {ppid: 2, state: 'R'}}
	if foregroundBlocked(1, table) {
		t.Fatal("running descendant must prevent waiting")
	}
	table[3] = processState{ppid: 2, state: 'S'}
	if !foregroundBlocked(1, table) {
		t.Fatal("all sleeping leaves should wait")
	}
	delete(table, 1)
	if foregroundBlocked(1, table) {
		t.Fatal("missing root must not wait")
	}
}
