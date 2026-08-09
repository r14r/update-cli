package history

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Entry struct {
	Timestamp   time.Time `json:"timestamp"`
	Action      string    `json:"action"`
	Phase       string    `json:"phase,omitempty"`
	ProjectName string    `json:"projectName"`
	FromVersion string    `json:"fromVersion,omitempty"`
	ToVersion   string    `json:"toVersion,omitempty"`
	Source      string    `json:"source,omitempty"`
	Backup      string    `json:"backup,omitempty"`
	Setup       bool      `json:"setup"`
	Status      string    `json:"status"`
	Message     string    `json:"message,omitempty"`
}

func Append(path string, e Entry) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	if e.Status == "" {
		e.Status = "success"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("Historie kann nicht geöffnet werden: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
func List(path string, limit int) ([]Entry, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := []Entry{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		v := bytes.TrimSpace(sc.Bytes())
		if len(v) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(v, &e); err != nil {
			return nil, fmt.Errorf("Historie enthält ungültiges JSON in Zeile %d: %w", line, err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp.After(out[j].Timestamp) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
