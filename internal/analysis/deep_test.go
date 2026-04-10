package analysis

import (
	"testing"
)

func TestParseDeepExtract_CleanJSON(t *testing.T) {
	raw := `{"shelf_total_usd":100000000,"shelf_used_usd":25000000,"shelf_remaining_usd":0,"atm_capacity_remaining_usd":50000000,"warrants":[{"strike":2.5,"shares":1000000,"expiration":"2027-01-01","description":"Series A"}],"convertibles":[],"notes":["floor price reset at $1"]}`
	ex, err := parseDeepExtract(raw)
	if err != nil {
		t.Fatalf("parseDeepExtract error: %v", err)
	}
	if ex.ShelfTotalUSD != 100_000_000 {
		t.Errorf("ShelfTotalUSD = %v, want 100M", ex.ShelfTotalUSD)
	}
	if ex.ATMCapacityRemainingUSD != 50_000_000 {
		t.Errorf("ATMCapacityRemainingUSD = %v, want 50M", ex.ATMCapacityRemainingUSD)
	}
	if len(ex.Warrants) != 1 || ex.Warrants[0].Strike != 2.5 {
		t.Errorf("expected 1 warrant with strike 2.5, got %+v", ex.Warrants)
	}
	if len(ex.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(ex.Notes))
	}
}

func TestParseDeepExtract_WrappedInProse(t *testing.T) {
	raw := "Here is the data:\n```json\n{\"shelf_total_usd\":5000000,\"warrants\":[],\"convertibles\":[]}\n```\nHope this helps."
	ex, err := parseDeepExtract(raw)
	if err != nil {
		t.Fatalf("parseDeepExtract error: %v", err)
	}
	if ex.ShelfTotalUSD != 5_000_000 {
		t.Errorf("ShelfTotalUSD = %v, want 5M", ex.ShelfTotalUSD)
	}
}

func TestParseDeepExtract_Invalid(t *testing.T) {
	if _, err := parseDeepExtract("not json at all"); err == nil {
		t.Error("expected error on non-JSON input")
	}
}

func TestMergeDeep_FirstNonZeroWins(t *testing.T) {
	// Ordered most recent first — newer shelf_total should win over older one.
	extracts := []*DeepExtract{
		{ShelfTotalUSD: 200_000_000, ShelfUsedUSD: 0},
		{ShelfTotalUSD: 100_000_000, ShelfUsedUSD: 30_000_000},
	}
	sources := []DeepSource{{Form: "S-3", FilingDate: "2025-01-01"}, {Form: "S-3", FilingDate: "2023-01-01"}}
	merged := MergeDeep(extracts, sources)
	if merged == nil {
		t.Fatal("MergeDeep returned nil")
	}
	if merged.ShelfTotalUSD != 200_000_000 {
		t.Errorf("ShelfTotalUSD = %v, want 200M (newest wins)", merged.ShelfTotalUSD)
	}
	if merged.ShelfUsedUSD != 30_000_000 {
		t.Errorf("ShelfUsedUSD = %v, want 30M (pulled from older since newer was 0)", merged.ShelfUsedUSD)
	}
}

func TestMergeDeep_DerivesRemaining(t *testing.T) {
	extracts := []*DeepExtract{
		{ShelfTotalUSD: 100_000_000, ShelfUsedUSD: 40_000_000, ShelfRemainingUSD: 0},
	}
	merged := MergeDeep(extracts, nil)
	if merged.ShelfRemainingUSD != 60_000_000 {
		t.Errorf("ShelfRemainingUSD derived = %v, want 60M", merged.ShelfRemainingUSD)
	}
}

func TestMergeDeep_DedupesWarrants(t *testing.T) {
	extracts := []*DeepExtract{
		{Warrants: []Warrant{{Strike: 2.5, Shares: 1_000_000, Expiration: "2027-01-01"}}},
		{Warrants: []Warrant{
			{Strike: 2.5, Shares: 1_000_000, Expiration: "2027-01-01"}, // dup
			{Strike: 5.0, Shares: 500_000, Expiration: "2028-06-01"},   // new
		}},
	}
	merged := MergeDeep(extracts, nil)
	if len(merged.Warrants) != 2 {
		t.Errorf("expected 2 warrants after dedupe, got %d: %+v", len(merged.Warrants), merged.Warrants)
	}
}

func TestMergeDeep_DropsEmptyEntries(t *testing.T) {
	extracts := []*DeepExtract{
		{Warrants: []Warrant{{Strike: 0, Shares: 0}}}, // junk row the LLM might return
	}
	merged := MergeDeep(extracts, nil)
	if len(merged.Warrants) != 0 {
		t.Errorf("expected empty warrants, got %+v", merged.Warrants)
	}
}

func TestMergeDeep_Empty(t *testing.T) {
	if MergeDeep(nil, nil) != nil {
		t.Error("MergeDeep(nil) should return nil")
	}
}

func TestMarkInTheMoney(t *testing.T) {
	d := &DeepDilution{
		Warrants: []Warrant{
			{Strike: 2.0, Shares: 1_000_000}, // ITM
			{Strike: 5.0, Shares: 500_000},   // ITM (at price)
			{Strike: 10.0, Shares: 2_000_000}, // OTM
		},
		Convertibles: []Convertible{
			{ConversionPrice: 3.0, PrincipalUSD: 10_000_000}, // ITM
			{ConversionPrice: 8.0, PrincipalUSD: 5_000_000},  // OTM
		},
	}
	d.MarkInTheMoney(5.0)

	if !d.Warrants[0].InTheMoney || !d.Warrants[1].InTheMoney {
		t.Error("expected warrants at strike 2 and 5 to be ITM at price 5")
	}
	if d.Warrants[2].InTheMoney {
		t.Error("warrant at strike 10 should not be ITM at price 5")
	}
	if d.ITMWarrantShares != 1_500_000 {
		t.Errorf("ITMWarrantShares = %v, want 1.5M", d.ITMWarrantShares)
	}
	if !d.Convertibles[0].InTheMoney || d.Convertibles[1].InTheMoney {
		t.Error("convertible ITM flags wrong")
	}
}

func TestMarkInTheMoney_ZeroPrice(t *testing.T) {
	d := &DeepDilution{Warrants: []Warrant{{Strike: 2.0, Shares: 1_000_000}}}
	d.MarkInTheMoney(0)
	if d.Warrants[0].InTheMoney || d.ITMWarrantShares != 0 {
		t.Error("zero price should leave everything unmarked")
	}
}

func TestMarkInTheMoney_NilReceiver(t *testing.T) {
	var d *DeepDilution
	d.MarkInTheMoney(5.0) // must not panic
}

func TestEvaluateDeepRiskFlags_WarrantsITMHigh(t *testing.T) {
	d := &DeepDilution{ITMWarrantShares: 2_000_000}
	q := QuoteData{FloatShares: 10_000_000} // 20% of float → HIGH
	flags := EvaluateDeepRiskFlags(d, q, 0)

	var got *RiskFlagDetail
	for i := range flags {
		if flags[i].Flag == FlagWarrantsITM {
			got = &flags[i]
		}
	}
	if got == nil {
		t.Fatal("expected WarrantsITM flag")
	}
	if got.Severity != "HIGH" {
		t.Errorf("severity = %q, want HIGH", got.Severity)
	}
}

func TestEvaluateDeepRiskFlags_WarrantsITMMedium(t *testing.T) {
	d := &DeepDilution{ITMWarrantShares: 200_000}
	q := QuoteData{FloatShares: 10_000_000} // 2% → MEDIUM
	flags := EvaluateDeepRiskFlags(d, q, 0)

	for _, f := range flags {
		if f.Flag == FlagWarrantsITM && f.Severity != "MEDIUM" {
			t.Errorf("severity = %q, want MEDIUM", f.Severity)
		}
	}
}

func TestEvaluateDeepRiskFlags_LargeShelf(t *testing.T) {
	d := &DeepDilution{ShelfRemainingUSD: 60_000_000}
	flags := EvaluateDeepRiskFlags(d, QuoteData{}, 100_000_000) // 60% of mcap

	found := false
	for _, f := range flags {
		if f.Flag == FlagShelfCapacity {
			found = true
			if f.Severity != "HIGH" {
				t.Errorf("severity = %q, want HIGH", f.Severity)
			}
		}
	}
	if !found {
		t.Error("expected LargeShelfRemaining flag when shelf >= 50% of market cap")
	}
}

func TestEvaluateDeepRiskFlags_MediumShelf(t *testing.T) {
	d := &DeepDilution{ShelfRemainingUSD: 30_000_000}
	flags := EvaluateDeepRiskFlags(d, QuoteData{}, 100_000_000) // 30% → MEDIUM

	var got *RiskFlagDetail
	for i := range flags {
		if flags[i].Flag == FlagShelfCapacity {
			got = &flags[i]
		}
	}
	if got == nil {
		t.Fatal("expected ShelfCapacity flag at 30% of market cap")
	}
	if got.Severity != "MEDIUM" {
		t.Errorf("severity = %q, want MEDIUM", got.Severity)
	}
}

func TestEvaluateDeepRiskFlags_SmallShelfNoFlag(t *testing.T) {
	d := &DeepDilution{ShelfRemainingUSD: 10_000_000}
	flags := EvaluateDeepRiskFlags(d, QuoteData{}, 100_000_000) // 10% of mcap

	for _, f := range flags {
		if f.Flag == FlagShelfCapacity {
			t.Error("should not flag LargeShelfRemaining at 10% of market cap")
		}
	}
}

func TestEvaluateDeepRiskFlags_NilDeep(t *testing.T) {
	flags := EvaluateDeepRiskFlags(nil, QuoteData{}, 0)
	if flags != nil {
		t.Errorf("expected nil flags for nil deep, got %+v", flags)
	}
}
