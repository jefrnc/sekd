package tui

import (
	"sort"
	"testing"

	"github.com/jefrnc/sekd/internal/analysis"
)

func TestDiffFlags_NewAndRemoved(t *testing.T) {
	prev := []string{"High Dilution", "Low Float"}
	cur := []string{"High Dilution", "Recent ATM", "Warrants In The Money"}

	added, removed := diffFlags(prev, cur)
	sort.Strings(added)
	sort.Strings(removed)

	wantAdded := []string{"Recent ATM", "Warrants In The Money"}
	wantRemoved := []string{"Low Float"}

	if len(added) != len(wantAdded) {
		t.Fatalf("added = %v, want %v", added, wantAdded)
	}
	for i := range added {
		if added[i] != wantAdded[i] {
			t.Errorf("added[%d] = %q, want %q", i, added[i], wantAdded[i])
		}
	}
	if len(removed) != 1 || removed[0] != wantRemoved[0] {
		t.Errorf("removed = %v, want %v", removed, wantRemoved)
	}
}

func TestDiffFlags_NoChange(t *testing.T) {
	prev := []string{"A", "B", "C"}
	cur := []string{"A", "B", "C"}
	added, removed := diffFlags(prev, cur)
	if len(added) != 0 || len(removed) != 0 {
		t.Errorf("expected no delta, got added=%v removed=%v", added, removed)
	}
}

func TestDiffFlags_EmptyPrev(t *testing.T) {
	// First-scan scenario: everything is "new" vs the empty baseline.
	added, removed := diffFlags(nil, []string{"High Dilution"})
	if len(removed) != 0 {
		t.Errorf("removed should be empty, got %v", removed)
	}
	if len(added) != 1 || added[0] != "High Dilution" {
		t.Errorf("added should contain the one flag, got %v", added)
	}
}

func TestDiffFlags_OrderIndependent(t *testing.T) {
	// diffFlags treats its inputs as sets — order shouldn't matter.
	added, removed := diffFlags([]string{"B", "A"}, []string{"A", "B"})
	if len(added) != 0 || len(removed) != 0 {
		t.Errorf("same set in different order should produce no delta, got added=%v removed=%v", added, removed)
	}
}

func TestCurrentFlagLabels(t *testing.T) {
	r := &analysis.Report{
		RiskFlags: []analysis.RiskFlagDetail{
			{Label: "High Dilution"},
			{Label: "Low Float"},
		},
	}
	labels := currentFlagLabels(r)
	if len(labels) != 2 || labels[0] != "High Dilution" || labels[1] != "Low Float" {
		t.Errorf("unexpected labels: %v", labels)
	}
}

func TestMatchSlashCommands_Prefix(t *testing.T) {
	matches := MatchSlashCommands("/wa")
	foundWatchlist := false
	for _, c := range matches {
		if c.Canonical() == "/watchlist" {
			foundWatchlist = true
		}
	}
	if !foundWatchlist {
		t.Errorf("expected /watchlist in matches for /wa, got %v", canonicals(matches))
	}
}

func TestMatchSlashCommands_Alias(t *testing.T) {
	matches := MatchSlashCommands("/q")
	// /q is an alias for /quit — we want the canonical /quit to appear.
	foundQuit := false
	for _, c := range matches {
		if c.Canonical() == "/quit" {
			foundQuit = true
		}
	}
	if !foundQuit {
		t.Errorf("expected /quit matched via /q alias, got %v", canonicals(matches))
	}
}

func TestMatchSlashCommands_NoDupesByCanonical(t *testing.T) {
	// /w is an alias for /watchlist — even though there are multiple /watchlist
	// variants in the registry, /w should still return the watchlist family
	// without duplicating the same canonical name.
	matches := MatchSlashCommands("/w")
	seen := make(map[string]int)
	for _, c := range matches {
		seen[c.Canonical()]++
	}
	for canon, count := range seen {
		if count > 1 {
			t.Errorf("canonical %s appeared %d times, expected 1", canon, count)
		}
	}
}

func TestMatchSlashCommands_EmptyInput(t *testing.T) {
	if m := MatchSlashCommands(""); m != nil {
		t.Errorf("expected nil for empty input, got %v", m)
	}
	if m := MatchSlashCommands("soun"); m != nil {
		t.Errorf("expected nil for non-slash input, got %v", m)
	}
}

func TestMatchSlashCommands_UnknownPrefix(t *testing.T) {
	if m := MatchSlashCommands("/xyzzy"); len(m) != 0 {
		t.Errorf("expected empty matches for /xyzzy, got %v", canonicals(m))
	}
}

func canonicals(cs []SlashCommand) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Canonical()
	}
	return out
}
