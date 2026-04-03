package history

import (
	"os"
	"path/filepath"
	"testing"
)

func testHistory(t *testing.T) *History {
	t.Helper()
	dir := t.TempDir()
	return &History{
		path:      filepath.Join(dir, "history.jsonl"),
		sessionID: "test-session",
	}
}

func TestAddAndFlush(t *testing.T) {
	h := testHistory(t)
	h.Add("SOUN", "ticker")
	h.Add("MARA", "ticker")
	h.Add("/filings SOUN", "command")

	if err := h.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	data, err := os.ReadFile(h.path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// 3 entries = 3 lines
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 3 {
		t.Errorf("expected 3 lines, got %d", lines)
	}
}

func TestAddEmpty(t *testing.T) {
	h := testHistory(t)
	h.Add("", "ticker")
	h.Add("  ", "ticker")
	h.Flush()

	entries := h.readFile()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty input, got %d", len(entries))
	}
}

func TestGetAll(t *testing.T) {
	h := testHistory(t)
	h.Add("SOUN", "ticker")
	h.Add("MARA", "ticker")
	h.Add("AAPL", "ticker")

	entries := h.GetAll()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Newest first
	if entries[0].Input != "AAPL" {
		t.Errorf("expected AAPL first (newest), got %s", entries[0].Input)
	}
	if entries[2].Input != "SOUN" {
		t.Errorf("expected SOUN last (oldest), got %s", entries[2].Input)
	}
}

func TestGetInputs_Deduped(t *testing.T) {
	h := testHistory(t)
	h.Add("SOUN", "ticker")
	h.Add("MARA", "ticker")
	h.Add("soun", "ticker") // duplicate different case
	h.Add("AAPL", "ticker")

	inputs := h.GetInputs()
	if len(inputs) != 3 {
		t.Errorf("expected 3 unique inputs, got %d: %v", len(inputs), inputs)
	}
}

func TestGetTickers(t *testing.T) {
	h := testHistory(t)
	h.Add("SOUN", "ticker")
	h.Add("/filings SOUN", "command")
	h.Add("MARA", "ticker")
	h.Add("/help", "command")

	tickers := h.GetTickers()
	if len(tickers) != 2 {
		t.Errorf("expected 2 tickers, got %d: %v", len(tickers), tickers)
	}
	if tickers[0] != "MARA" {
		t.Errorf("expected MARA first (newest), got %s", tickers[0])
	}
}

func TestSearch(t *testing.T) {
	h := testHistory(t)
	h.Add("SOUN", "ticker")
	h.Add("/filings SOUN", "command")
	h.Add("MARA", "ticker")
	h.Add("/analyze MARA", "command")

	results := h.Search("soun")
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'soun', got %d", len(results))
	}

	results = h.Search("mara")
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'mara', got %d", len(results))
	}

	results = h.Search("zzz")
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'zzz', got %d", len(results))
	}
}

func TestTrim(t *testing.T) {
	h := testHistory(t)

	// Add more than maxEntries
	for i := 0; i < maxEntries+50; i++ {
		h.Add("TICKER", "ticker")
	}
	h.Flush()

	entries := h.readFile()
	if len(entries) != maxEntries {
		t.Errorf("expected %d entries after trim, got %d", maxEntries, len(entries))
	}
}

func TestFlushEmpty(t *testing.T) {
	h := testHistory(t)
	if err := h.Flush(); err != nil {
		t.Errorf("Flush empty should not error: %v", err)
	}
}

func TestSessionID(t *testing.T) {
	h := testHistory(t)
	h.Add("SOUN", "ticker")
	h.Flush()

	entries := h.readFile()
	if entries[0].SessionID != "test-session" {
		t.Errorf("SessionID = %q, want test-session", entries[0].SessionID)
	}
}

func TestFilePermissions(t *testing.T) {
	h := testHistory(t)
	h.Add("SOUN", "ticker")
	h.Flush()

	info, err := os.Stat(h.path)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions = %o, want 600", info.Mode().Perm())
	}
}
