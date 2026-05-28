package result

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type PageStore struct {
	dir string
}

type storedPage struct {
	Columns []Column `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

func NewPageStore(dir string) *PageStore {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "miudb-pages")
	}
	return &PageStore{dir: dir}
}

func (s *PageStore) Save(columns []Column, remaining [][]any, limit int) (string, error) {
	if len(remaining) == 0 {
		return "", nil
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return "", err
	}
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	path := filepath.Join(s.dir, id+".json")
	data, err := json.Marshal(storedPage{Columns: columns, Rows: remaining})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return encodeCursor(id, 0, limit), nil
}

func (s *PageStore) Fetch(cursor string) (QueryPage, error) {
	id, offset, limit, err := decodeCursor(cursor)
	if err != nil {
		return QueryPage{}, err
	}
	data, err := os.ReadFile(filepath.Join(s.dir, id+".json"))
	if err != nil {
		return QueryPage{}, err
	}
	var stored storedPage
	if err := json.Unmarshal(data, &stored); err != nil {
		return QueryPage{}, err
	}
	if offset >= len(stored.Rows) {
		return QueryPage{Result: QueryResult{Columns: stored.Columns, Rows: [][]any{}, Truncated: false}}, nil
	}
	end := offset + limit
	if end > len(stored.Rows) {
		end = len(stored.Rows)
	}
	next := ""
	truncated := end < len(stored.Rows)
	if truncated {
		next = encodeCursor(id, end, limit)
	}
	return QueryPage{
		Result: QueryResult{
			Columns:   stored.Columns,
			Rows:      stored.Rows[offset:end],
			Truncated: truncated,
		},
		NextCursor: next,
	}, nil
}

func encodeCursor(id string, offset, limit int) string {
	raw := strings.Join([]string{id, strconv.Itoa(offset), strconv.Itoa(limit)}, ":")
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (string, int, int, error) {
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid cursor: %w", err)
	}
	parts := strings.Split(string(data), ":")
	if len(parts) != 3 {
		return "", 0, 0, fmt.Errorf("invalid cursor")
	}
	offset, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid cursor offset: %w", err)
	}
	limit, err := strconv.Atoi(parts[2])
	if err != nil || limit <= 0 {
		return "", 0, 0, fmt.Errorf("invalid cursor limit")
	}
	return parts[0], offset, limit, nil
}
