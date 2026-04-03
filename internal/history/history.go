package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxEntries = 200

type Entry struct {
	Input     string `json:"input"`
	Timestamp int64  `json:"ts"`
	Type      string `json:"type"` // "ticker", "command", "config"
	SessionID string `json:"sid,omitempty"`
}

type History struct {
	path      string
	sessionID string
	mu        sync.Mutex
	pending   []Entry
}

func New(sessionID string) (*History, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".sekd")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &History{
		path:      filepath.Join(dir, "history.jsonl"),
		sessionID: sessionID,
	}, nil
}

func (h *History) Add(input, entryType string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	h.pending = append(h.pending, Entry{
		Input:     input,
		Timestamp: time.Now().UnixMilli(),
		Type:      entryType,
		SessionID: h.sessionID,
	})
}

func (h *History) Flush() error {
	h.mu.Lock()
	entries := h.pending
	h.pending = nil
	h.mu.Unlock()

	if len(entries) == 0 {
		return nil
	}

	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			continue
		}
		f.Write(data)
		f.Write([]byte("\n"))
	}

	return h.trimIfNeeded()
}

// GetAll returns all history entries, newest first.
func (h *History) GetAll() []Entry {
	h.Flush()
	entries := h.readFile()
	// Reverse: newest first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries
}

// GetInputs returns deduplicated input strings, newest first.
// Used for up-arrow navigation.
func (h *History) GetInputs() []string {
	entries := h.GetAll()
	seen := make(map[string]bool)
	var inputs []string
	for _, e := range entries {
		lower := strings.ToLower(e.Input)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		inputs = append(inputs, e.Input)
	}
	return inputs
}

// Search returns entries matching a query string (case-insensitive).
func (h *History) Search(query string) []Entry {
	entries := h.GetAll()
	query = strings.ToLower(query)
	var results []Entry
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Input), query) {
			results = append(results, e)
		}
	}
	return results
}

// GetTickers returns unique tickers searched, newest first.
func (h *History) GetTickers() []string {
	entries := h.GetAll()
	seen := make(map[string]bool)
	var tickers []string
	for _, e := range entries {
		if e.Type != "ticker" {
			continue
		}
		upper := strings.ToUpper(e.Input)
		if seen[upper] {
			continue
		}
		seen[upper] = true
		tickers = append(tickers, upper)
	}
	return tickers
}

func (h *History) readFile() []Entry {
	f, err := os.Open(h.path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var e Entry
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

func (h *History) trimIfNeeded() error {
	entries := h.readFile()
	if len(entries) <= maxEntries {
		return nil
	}

	// Keep newest maxEntries
	entries = entries[len(entries)-maxEntries:]

	f, err := os.OpenFile(h.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, e := range entries {
		data, _ := json.Marshal(e)
		f.Write(data)
		f.Write([]byte("\n"))
	}
	return nil
}
