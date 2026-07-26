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
	ProjectName string    `json:"projectName"`
	FromVersion string    `json:"fromVersion,omitempty"`
	ToVersion   string    `json:"toVersion,omitempty"`
	Archive     string    `json:"archive,omitempty"`
	Backup      string    `json:"backup,omitempty"`
	Setup       bool      `json:"setup"`
	Status      string    `json:"status"`
	Message     string    `json:"message,omitempty"`
}

func Append(path string, entry Entry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.Status == "" {
		entry.Status = "success"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("Historienordner kann nicht erstellt werden: %w", err)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("Historieneintrag kann nicht serialisiert werden: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("Historie kann nicht geöffnet werden: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("Historie kann nicht geschrieben werden: %w", err)
	}
	return file.Sync()
}

func List(path string, limit int) ([]Entry, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Historie kann nicht gelesen werden: %w", err)
	}
	defer file.Close()

	entries := make([]Entry, 0)
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		value := bytes.TrimSpace(scanner.Bytes())
		if len(value) == 0 {
			continue
		}
		var entry Entry
		if err := json.Unmarshal(value, &entry); err != nil {
			return nil, fmt.Errorf("Historie enthält ungültiges JSON in Zeile %d: %w", line, err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Historie kann nicht vollständig gelesen werden: %w", err)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}
