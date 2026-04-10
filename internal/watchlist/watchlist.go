package watchlist

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Entry struct {
	Ticker  string `json:"ticker"`
	Note    string `json:"note,omitempty"`
	AddedAt int64  `json:"added_at"`

	// Snapshot fields populated by /watchlist scan. Zero values on entries
	// that have never been scanned — compare paths must handle that.
	LastScore     int      `json:"last_score,omitempty"`
	LastGrade     string   `json:"last_grade,omitempty"`
	LastFlags     []string `json:"last_flags,omitempty"`
	LastAccession string   `json:"last_accession,omitempty"`
	LastFilingDate string  `json:"last_filing_date,omitempty"`
	LastScannedAt int64    `json:"last_scanned_at,omitempty"`
}

// HasSnapshot reports whether this entry has ever been scanned, which
// determines whether a diff can be computed vs the current state.
func (e *Entry) HasSnapshot() bool {
	return e.LastScannedAt > 0
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
	if err := os.MkdirAll(dir, 0700); err != nil {
		return &Watchlist{path: filepath.Join(dir, "watchlist.json")}, nil
	}
	path := filepath.Join(dir, "watchlist.json")

	w := &Watchlist{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		// No file yet — return an empty watchlist ready for the first Add.
		return w, nil
	}
	if err := json.Unmarshal(data, w); err != nil {
		// The file exists but is not valid JSON. Refusing to overwrite it
		// is the only safe choice — otherwise the next Save would destroy
		// whatever data the user had. Back it up, return an empty in-memory
		// watchlist with no path (so Save will no-op), and surface the error.
		backup := fmt.Sprintf("%s.broken-%d", path, time.Now().Unix())
		_ = os.Rename(path, backup)
		return &Watchlist{}, fmt.Errorf("watchlist file %s was corrupt and has been moved to %s; starting with an empty list: %w", path, backup, err)
	}
	w.path = path
	return w, nil
}

func (w *Watchlist) Save() error {
	// If the watchlist was loaded from a corrupt file we intentionally
	// leave path empty to prevent Save from overwriting user data.
	if w.path == "" {
		return fmt.Errorf("watchlist has no path; refusing to save (was the file corrupt on load?)")
	}
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

// UpdateSnapshot writes the latest scan result into the matching entry so
// the next scan can compute a diff. Returns false if the ticker isn't in
// the watchlist.
func (w *Watchlist) UpdateSnapshot(ticker string, score int, grade string, flags []string, lastAccession, lastFilingDate string) bool {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	for i := range w.Entries {
		if w.Entries[i].Ticker == ticker {
			w.Entries[i].LastScore = score
			w.Entries[i].LastGrade = grade
			w.Entries[i].LastFlags = flags
			w.Entries[i].LastAccession = lastAccession
			w.Entries[i].LastFilingDate = lastFilingDate
			w.Entries[i].LastScannedAt = time.Now().UnixMilli()
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
