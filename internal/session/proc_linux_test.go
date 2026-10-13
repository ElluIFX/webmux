//go:build linux

package session

import "testing"

func TestProcessTreeWaiting(t *testing.T) {
	table := processTable{1: {0, 'S'}, 2: {1, 'S'}, 3: {2, 'R'}}
	if foregroundBlocked(1, table) {
		t.Fatal("running descendant must prevent waiting")
	}
	table[3] = processState{2, 'S'}
	if !foregroundBlocked(1, table) {
		t.Fatal("all sleeping leaves should wait")
	}
	delete(table, 1)
	if foregroundBlocked(1, table) {
		t.Fatal("missing root must not wait")
	}
}
