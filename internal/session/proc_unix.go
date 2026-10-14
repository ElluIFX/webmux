//go:build linux || darwin

package session

// state uses ps letters: R running, S sleeping, Z zombie. Other values, including
// '?' for a process whose run state is unreadable, never count as waiting.
type processState struct {
	ppid  int
	state byte
	sid   int
	start uint64
}
type processTable map[int]processState

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
