// Package installerconf reads per-package releases.conf files.
//
// The format is simple key=value, one per line. Blank lines and lines
// starting with # are ignored. Keys and values are trimmed of whitespace.
//
//	source = github
//	owner = gohugoio
//	repo = hugo
package installerconf

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Conf holds the parsed contents of a releases.conf file.
type Conf struct {
	m map[string]string
}

// Read parses a releases.conf file.
func Read(path string) (*Conf, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("installerconf: %w", err)
	}
	defer f.Close()

	c := &Conf{m: make(map[string]string)}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		c.m[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("installerconf: read %s: %w", path, err)
	}
	return c, nil
}

// Get returns the value for a key, or "" if not set.
func (c *Conf) Get(key string) string {
	return c.m[key]
}

// Source returns the fetch source type (e.g. "github", "nodedist", "gitea").
func (c *Conf) Source() string {
	return c.m["source"]
}
