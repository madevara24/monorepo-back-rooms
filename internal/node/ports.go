package node

import "fmt"

// allocatePort returns the lowest free port in [min, max], given the set
// of ports Docker currently has published.
func allocatePort(min, max int, used map[int]bool) (int, error) {
	for p := min; p <= max; p++ {
		if !used[p] {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port in range %d-%d", min, max)
}
