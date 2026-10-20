// Package routemap parses the SNI host map the control plane's
// reconciler writes (see control-panel-service/internal/usecase
// Render), and watches it for changes so the router can pick up newly
// provisioned or deleted tenants without a restart.
package routemap

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Map is an immutable snapshot of the host map: public SNI hostname
// to the tenant's internal "host:port" dial address. Callers never
// mutate a Map in place; a reload builds a new one and swaps it in.
type Map map[string]string

// Lookup returns the dial address for host, and whether it was found.
func (m Map) Lookup(host string) (string, bool) {
	addr, ok := m[host]
	return addr, ok
}

// Parse reads the map file format the reconciler writes: one route
// per line, "<hostname> <host>:<port>;", blank lines ignored. Any
// malformed line fails the whole parse, so a caller watching the file
// can keep serving the last good Map instead of a partial one.
func Parse(r *bufio.Reader) (Map, error) {
	out := make(Map)
	scanner := bufio.NewScanner(r)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		line = strings.TrimSuffix(line, ";")

		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("routemap: line %d: want 2 fields, got %d: %q", lineNo, len(fields), line)
		}
		host, addr := fields[0], fields[1]
		if host == "" || addr == "" {
			return nil, fmt.Errorf("routemap: line %d: empty host or address: %q", lineNo, line)
		}
		out[host] = addr
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("routemap: scan: %w", err)
	}
	return out, nil
}

// ParseFile opens path and parses it with Parse.
func ParseFile(path string) (Map, error) {
	f, err := os.Open(path) //nolint:gosec // path is an operator-controlled config value, not user input
	if err != nil {
		return nil, fmt.Errorf("routemap: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return Parse(bufio.NewReader(f))
}
