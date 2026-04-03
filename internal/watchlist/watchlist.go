package watchlist

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Entry struct {
	Ticker  string `json:"ticker"`
	Note    string `json:"note,omitempty"`
	AddedAt int64  `json:"added_at"`
}

type Watchlist struct {
	path    string
	Entries []Entry `json:"entries"`
}

func Load() (*Watchlist, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &Watchlist{}, nil
	}
	dir := filepath.Join(home, ".sekd")
	os.MkdirAll(dir, 0700)
	path := filepath.Join(dir, "watchlist.json")

	w := &Watchlist{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		return w, nil
	}
	json.Unmarshal(data, w)
	return w, nil
}

func (w *Watchlist) Save() error {
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(w.path, data, 0600)
}

func (w *Watchlist) Add(ticker, note string) bool {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if ticker == "" {
		return false
	}
	for _, e := range w.Entries {
		if e.Ticker == ticker {
			return false // already exists
		}
	}
	w.Entries = append(w.Entries, Entry{
		Ticker:  ticker,
		Note:    note,
		AddedAt: time.Now().UnixMilli(),
	})
	return true
}

func (w *Watchlist) Remove(ticker string) bool {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	for i, e := range w.Entries {
		if e.Ticker == ticker {
			w.Entries = append(w.Entries[:i], w.Entries[i+1:]...)
			return true
		}
	}
	return false
}

func (w *Watchlist) Has(ticker string) bool {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	for _, e := range w.Entries {
		if e.Ticker == ticker {
			return true
		}
	}
	return false
}

func (w *Watchlist) Tickers() []string {
	var tickers []string
	for _, e := range w.Entries {
		tickers = append(tickers, e.Ticker)
	}
	return tickers
}
