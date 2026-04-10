package watchlist

import (
	"os"
	"path/filepath"
	"testing"
)

func testWatchlist(t *testing.T) *Watchlist {
	t.Helper()
	dir := t.TempDir()
	return &Watchlist{path: filepath.Join(dir, "watchlist.json")}
}

func TestAddAndTickers(t *testing.T) {
	w := testWatchlist(t)
	w.Add("SOUN", "")
	w.Add("MARA", "bitcoin miner")
	w.Add("AAPL", "")

	tickers := w.Tickers()
	if len(tickers) != 3 {
		t.Fatalf("expected 3, got %d", len(tickers))
	}
}

func TestUpdateSnapshot(t *testing.T) {
	w := testWatchlist(t)
	w.Add("SOUN", "")

	before := w.Entries[0]
	if before.HasSnapshot() {
		t.Error("fresh entry should not report HasSnapshot")
	}

	ok := w.UpdateSnapshot("SOUN", 55, "D", []string{"High Dilution", "High Short Interest"}, "0001840856-25-000001", "2025-01-28")
	if !ok {
		t.Fatal("UpdateSnapshot returned false for existing ticker")
	}

	after := w.Entries[0]
	if !after.HasSnapshot() {
		t.Error("entry should report HasSnapshot after update")
	}
	if after.LastScore != 55 || after.LastGrade != "D" {
		t.Errorf("score/grade not saved: %+v", after)
	}
	if after.LastAccession != "0001840856-25-000001" {
		t.Errorf("accession not saved: %q", after.LastAccession)
	}
	if len(after.LastFlags) != 2 {
		t.Errorf("flags not saved: %v", after.LastFlags)
	}
	if after.LastScannedAt == 0 {
		t.Error("LastScannedAt should be set")
	}
}

func TestUpdateSnapshot_UnknownTicker(t *testing.T) {
	w := testWatchlist(t)
	if w.UpdateSnapshot("NOPE", 0, "", nil, "", "") {
		t.Error("expected false for unknown ticker")
	}
}

func TestLoad_CorruptFileIsBackedUpNotDestroyed(t *testing.T) {
	// Regression test for a data-loss footgun: Load used to silently
	// discard corrupt JSON, and the next Save would overwrite the
	// original file destroying whatever the user had. Now Load must
	// back up the corrupt file and refuse to auto-save over it.
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	sekdDir := filepath.Join(dir, ".sekd")
	if err := os.MkdirAll(sekdDir, 0700); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	realPath := filepath.Join(sekdDir, "watchlist.json")
	if err := os.WriteFile(realPath, []byte("this is not valid json {{{"), 0600); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	wl, err := Load()
	if err == nil {
		t.Fatal("Load should return an error for corrupt JSON")
	}
	if wl == nil {
		t.Fatal("Load should still return a safe empty Watchlist on error")
	}
	if len(wl.Entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(wl.Entries))
	}

	matches, globErr := filepath.Glob(filepath.Join(sekdDir, "watchlist.json.broken-*"))
	if globErr != nil {
		t.Fatalf("glob: %v", globErr)
	}
	if len(matches) == 0 {
		t.Error("expected a backup .broken-* file to exist after corrupt load")
	}

	// Save on the returned Watchlist must refuse to touch disk because
	// the path is intentionally empty to prevent overwriting.
	if saveErr := wl.Save(); saveErr == nil {
		t.Error("Save should refuse to write when path is empty")
	}
}

func TestAddDuplicate(t *testing.T) {
	w := testWatchlist(t)
	w.Add("SOUN", "")
	ok := w.Add("soun", "")
	if ok {
		t.Error("should return false for duplicate")
	}
	if len(w.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(w.Entries))
	}
}

func TestAddEmpty(t *testing.T) {
	w := testWatchlist(t)
	ok := w.Add("", "")
	if ok {
		t.Error("should return false for empty ticker")
	}
}

func TestRemove(t *testing.T) {
	w := testWatchlist(t)
	w.Add("SOUN", "")
	w.Add("MARA", "")

	ok := w.Remove("soun")
	if !ok {
		t.Error("should return true for existing ticker")
	}
	if len(w.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(w.Entries))
	}
	if w.Entries[0].Ticker != "MARA" {
		t.Errorf("expected MARA, got %s", w.Entries[0].Ticker)
	}
}

func TestRemoveNonExistent(t *testing.T) {
	w := testWatchlist(t)
	ok := w.Remove("AAPL")
	if ok {
		t.Error("should return false for non-existent")
	}
}

func TestHas(t *testing.T) {
	w := testWatchlist(t)
	w.Add("SOUN", "")

	if !w.Has("soun") {
		t.Error("should find SOUN (case insensitive)")
	}
	if w.Has("MARA") {
		t.Error("should not find MARA")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watchlist.json")

	w := &Watchlist{path: path}
	w.Add("SOUN", "ai company")
	w.Add("MARA", "")
	w.Save()

	// Verify file
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file not created")
	}

	// Load
	w2 := &Watchlist{path: path}
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Fatal("empty file")
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	// Re-create path structure for Load
	os.MkdirAll(filepath.Join(dir, ".sekd"), 0700)
	os.WriteFile(filepath.Join(dir, ".sekd", "watchlist.json"), data, 0600)
	w2, _ = Load()

	if len(w2.Entries) != 2 {
		t.Fatalf("expected 2 entries after load, got %d", len(w2.Entries))
	}
	if w2.Entries[0].Note != "ai company" {
		t.Errorf("note = %q, want 'ai company'", w2.Entries[0].Note)
	}
}

func TestUppercase(t *testing.T) {
	w := testWatchlist(t)
	w.Add("soun", "")
	if w.Entries[0].Ticker != "SOUN" {
		t.Errorf("expected uppercase, got %s", w.Entries[0].Ticker)
	}
}
