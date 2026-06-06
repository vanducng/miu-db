package activity

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vanducng/miu-db/internal/config"
)

// DefaultRoot returns the canonical activity directory.
func DefaultRoot() string {
	return filepath.Join(config.DefaultConfigDir(), "activity")
}

// Filter narrows which events Query returns.
type Filter struct {
	Connection string
	Group      string
	Session    string
	Since      time.Duration // 0 means unbounded
	FailedOnly bool
	Limit      int // 0 means no cap
}

// Query reads all JSONL files under root, applies f, and returns events sorted ts-desc.
// Unparseable lines are silently skipped (crash-tolerance).
func Query(root string, f Filter) ([]Event, error) {
	cutoff := time.Time{}
	if f.Since > 0 {
		cutoff = time.Now().UTC().Add(-f.Since)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var events []Event
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !looksLikeDateDir(name) {
			continue
		}
		if !cutoff.IsZero() {
			dirDate, err := time.Parse("2006-01-02", name)
			if err != nil {
				continue
			}
			// Skip entire date dir if its day-end is before cutoff.
			// dirDate represents the start of that day; add 24h to get day-end.
			if dirDate.Add(24 * time.Hour).Before(cutoff) {
				continue
			}
		}

		files, err := filepath.Glob(filepath.Join(root, name, "*.jsonl"))
		if err != nil {
			continue
		}
		for _, fpath := range files {
			if err := scanJSONL(fpath, f, cutoff, &events); err != nil {
				continue
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Ts > events[j].Ts
	})

	if f.Limit > 0 && len(events) > f.Limit {
		events = events[:f.Limit]
	}
	return events, nil
}

func scanJSONL(path string, f Filter, cutoff time.Time, out *[]Event) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip unparseable
		}
		if !matchFilter(e, f, cutoff) {
			continue
		}
		*out = append(*out, e)
	}
	return sc.Err()
}

func matchFilter(e Event, f Filter, cutoff time.Time) bool {
	if !cutoff.IsZero() {
		ts, err := time.Parse(time.RFC3339Nano, e.Ts)
		if err != nil {
			ts, _ = time.Parse(time.RFC3339, e.Ts)
		}
		if !ts.IsZero() && ts.Before(cutoff) {
			return false
		}
	}
	if f.Session != "" && e.SessionID != f.Session {
		return false
	}
	if f.Group != "" && e.Group != f.Group {
		return false
	}
	if f.Connection != "" {
		// Accept "group/connection" or bare "connection".
		if strings.Contains(f.Connection, "/") {
			parts := strings.SplitN(f.Connection, "/", 2)
			if e.Group != parts[0] || e.Connection != parts[1] {
				return false
			}
		} else if e.Connection != f.Connection {
			return false
		}
	}
	if f.FailedOnly && e.Error == nil {
		return false
	}
	return true
}

func looksLikeDateDir(name string) bool {
	if len(name) != 10 {
		return false
	}
	_, err := time.Parse("2006-01-02", name)
	return err == nil
}

// Prune removes date directories under root that are older than olderThan.
// Returns the count of removed directories. When dryRun is true, lists dirs
// that would be removed without deleting them.
func Prune(root string, olderThan time.Duration, dryRun bool) (removed int, dirs []string, err error) {
	threshold := time.Now().UTC().Add(-olderThan)

	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return 0, nil, nil
		}
		return 0, nil, readErr
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !looksLikeDateDir(name) {
			continue
		}
		dirDate, parseErr := time.Parse("2006-01-02", name)
		if parseErr != nil {
			continue
		}
		if dirDate.Add(24 * time.Hour).Before(threshold) {
			dirPath := filepath.Join(root, name)
			dirs = append(dirs, dirPath)
			if !dryRun {
				if removeErr := os.RemoveAll(dirPath); removeErr == nil {
					removed++
				}
			} else {
				removed++
			}
		}
	}
	return removed, dirs, nil
}

// ParseSince parses a duration string supporting h/d/w suffixes (e.g. 24h, 7d, 2w).
// Falls back to time.ParseDuration for standard Go suffixes.
func ParseSince(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	last := s[len(s)-1]
	switch last {
	case 'd':
		n, err := parsePosInt(s[:len(s)-1])
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		n, err := parsePosInt(s[:len(s)-1])
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func parsePosInt(s string) (int64, error) {
	if s == "" {
		return 0, &parseErr{s}
	}
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &parseErr{s}
		}
		n = n*10 + int64(c-'0')
	}
	if n <= 0 {
		return 0, &parseErr{s}
	}
	return n, nil
}

type parseErr struct{ s string }

func (e *parseErr) Error() string { return "invalid duration: " + e.s }
