package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Session struct {
	ID           string `json:"id"`
	LastTicker   string `json:"last_ticker,omitempty"`
	LastCIK      string `json:"last_cik,omitempty"`
	LastOutput   string `json:"last_output,omitempty"`
	OutputMode   string `json:"output_mode,omitempty"`
	LastScore    *Score `json:"last_score,omitempty"`
	UpdatedAt    int64  `json:"updated_at"`
}

type Score struct {
	Value int    `json:"value"`
	Grade string `json:"grade"`
}

func sessionPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".sekd")
	os.MkdirAll(dir, 0700)
	return filepath.Join(dir, "session.json"), nil
}

// Save persists the current session state.
func Save(s *Session) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	s.UpdatedAt = time.Now().UnixMilli()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Load restores the last session. Returns nil if no session exists
// or if it's older than maxAge.
func Load(maxAge time.Duration) *Session {
	path, err := sessionPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var s Session
	if json.Unmarshal(data, &s) != nil {
		return nil
	}
	// Check staleness
	age := time.Since(time.UnixMilli(s.UpdatedAt))
	if age > maxAge {
		return nil
	}
	return &s
}

// Clear removes the saved session.
func Clear() {
	path, _ := sessionPath()
	if path != "" {
		os.Remove(path)
	}
}
