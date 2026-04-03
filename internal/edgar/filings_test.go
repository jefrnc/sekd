package edgar

import (
	"testing"
	"time"
)

func makeSubmissions(forms, dates []string) *Submissions {
	accessions := make([]string, len(forms))
	docs := make([]string, len(forms))
	for i := range forms {
		accessions[i] = "0000000000-00-00000" + string(rune('0'+i))
		docs[i] = "doc" + string(rune('0'+i)) + ".htm"
	}
	return &Submissions{
		Filings: struct {
			Recent FilingRecent `json:"recent"`
			Files  []struct {
				Name string `json:"name"`
			} `json:"files"`
		}{
			Recent: FilingRecent{
				Form:            forms,
				FilingDate:      dates,
				AccessionNumber: accessions,
				PrimaryDocument: docs,
			},
		},
	}
}

func TestFilterFilings_ByFormType(t *testing.T) {
	subs := makeSubmissions(
		[]string{"S-3", "10-K", "424B5", "8-K", "S-3"},
		[]string{"2024-06-01", "2024-05-01", "2024-04-01", "2024-03-01", "2024-02-01"},
	)

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	results := FilterFilings(subs, []string{"S-3", "424B5"}, since)

	if len(results) != 3 {
		t.Fatalf("expected 3 filings, got %d", len(results))
	}

	if results[0].Form != "S-3" {
		t.Errorf("results[0].Form = %q, want S-3", results[0].Form)
	}
	if results[1].Form != "424B5" {
		t.Errorf("results[1].Form = %q, want 424B5", results[1].Form)
	}
}

func TestFilterFilings_SinceDate(t *testing.T) {
	subs := makeSubmissions(
		[]string{"S-3", "S-3", "S-3"},
		[]string{"2024-06-01", "2024-03-01", "2023-06-01"},
	)

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	results := FilterFilings(subs, []string{"S-3"}, since)

	if len(results) != 2 {
		t.Fatalf("expected 2 filings after 2024-01-01, got %d", len(results))
	}
}

func TestFilterFilings_CaseInsensitive(t *testing.T) {
	subs := makeSubmissions(
		[]string{"s-3", "S-3"},
		[]string{"2024-06-01", "2024-05-01"},
	)

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	results := FilterFilings(subs, []string{"S-3"}, since)

	if len(results) != 2 {
		t.Fatalf("expected 2 (case insensitive), got %d", len(results))
	}
}

func TestFilterFilings_Empty(t *testing.T) {
	subs := makeSubmissions([]string{}, []string{})
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	results := FilterFilings(subs, []string{"S-3"}, since)

	if len(results) != 0 {
		t.Fatalf("expected 0 filings, got %d", len(results))
	}
}

func TestFilterFilings_MismatchedArrays(t *testing.T) {
	subs := &Submissions{
		Filings: struct {
			Recent FilingRecent `json:"recent"`
			Files  []struct {
				Name string `json:"name"`
			} `json:"files"`
		}{
			Recent: FilingRecent{
				Form:            []string{"S-3", "10-K", "424B5"},
				FilingDate:      []string{"2024-06-01"},
				AccessionNumber: []string{"0000000000-00-000000"},
				PrimaryDocument: []string{"doc.htm"},
			},
		},
	}

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	results := FilterFilings(subs, []string{"S-3", "10-K", "424B5"}, since)

	if len(results) != 1 {
		t.Fatalf("expected 1 filing (safe bound), got %d", len(results))
	}
}

func TestCountForm4Filings(t *testing.T) {
	subs := makeSubmissions(
		[]string{"4", "4", "10-K", "4", "8-K"},
		[]string{"2024-06-01", "2024-05-01", "2024-04-01", "2023-01-01", "2024-03-01"},
	)

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	count := CountForm4Filings(subs, since)

	if count != 2 {
		t.Errorf("expected 2 Form 4 filings since 2024, got %d", count)
	}
}

func TestCountForm4Filings_None(t *testing.T) {
	subs := makeSubmissions(
		[]string{"10-K", "8-K", "S-3"},
		[]string{"2024-06-01", "2024-05-01", "2024-04-01"},
	)

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	count := CountForm4Filings(subs, since)

	if count != 0 {
		t.Errorf("expected 0 Form 4 filings, got %d", count)
	}
}
