package analysis

import (
	"testing"
)

func TestParseFloatString(t *testing.T) {
	tests := []struct {
		input string
		want  float64
		ok    bool
	}{
		{"14.66B", 14_660_000_000, true},
		{"381.81M", 381_810_000, true},
		{"5.5K", 5_500, true},
		{"1234", 1234, true},
		{"1,234,567", 1_234_567, true},
		{"", 0, false},
		{"-", 0, false},
		{"N/A", 0, false},
	}

	for _, tt := range tests {
		got, ok := ParseFloatString(tt.input)
		if ok != tt.ok {
			t.Errorf("ParseFloatString(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("ParseFloatString(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParsePercentString(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"35.82%", 35.82},
		{"0.85%", 0.85},
		{"100%", 100},
		{"", 0},
		{"N/A", 0},
	}

	for _, tt := range tests {
		got := ParsePercentString(tt.input)
		if got != tt.want {
			t.Errorf("ParsePercentString(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestEvaluateRiskFlags_HighDilution(t *testing.T) {
	da := DilutionAnalysis{DilutionRate12M: 50.0}
	flags := EvaluateRiskFlags(da, 0, QuoteData{})

	found := false
	for _, f := range flags {
		if f.Flag == FlagHighDilution {
			found = true
			if f.Severity != "HIGH" {
				t.Errorf("HighDilution severity = %q, want HIGH", f.Severity)
			}
			if f.Points != 25 {
				t.Errorf("HighDilution points = %d, want 25", f.Points)
			}
		}
	}
	if !found {
		t.Error("expected HighDilution flag for 50% dilution rate")
	}
}

func TestEvaluateRiskFlags_NoDilution(t *testing.T) {
	da := DilutionAnalysis{DilutionRate12M: 5.0}
	flags := EvaluateRiskFlags(da, 0, QuoteData{})

	for _, f := range flags {
		if f.Flag == FlagHighDilution {
			t.Error("should not flag HighDilution for 5% rate")
		}
	}
}

func TestEvaluateRiskFlags_LowFloat(t *testing.T) {
	da := DilutionAnalysis{}
	q := QuoteData{Float: "5M", IsLowFloat: true, ShortFloatPct: 5}
	flags := EvaluateRiskFlags(da, 0, q)

	found := false
	for _, f := range flags {
		if f.Flag == FlagLowFloat {
			found = true
		}
	}
	if !found {
		t.Error("expected LowFloat flag")
	}
}

func TestEvaluateRiskFlags_HighShortInterest(t *testing.T) {
	da := DilutionAnalysis{}
	q := QuoteData{ShortFloatPct: 35}
	flags := EvaluateRiskFlags(da, 0, q)

	found := false
	for _, f := range flags {
		if f.Flag == FlagHighShortInt {
			found = true
		}
	}
	if !found {
		t.Error("expected HighShortInterest flag for 35%")
	}
}

func TestEvaluateRiskFlags_NoShortInterestBelow20(t *testing.T) {
	da := DilutionAnalysis{}
	q := QuoteData{ShortFloatPct: 15}
	flags := EvaluateRiskFlags(da, 0, q)

	for _, f := range flags {
		if f.Flag == FlagHighShortInt {
			t.Error("should not flag HighShortInterest for 15%")
		}
	}
}

func TestEvaluateRiskFlags_MassiveAuthorized(t *testing.T) {
	da := DilutionAnalysis{AuthorizedRatio: 5.0}
	flags := EvaluateRiskFlags(da, 0, QuoteData{})

	found := false
	for _, f := range flags {
		if f.Flag == FlagMassiveAuthorized {
			found = true
		}
	}
	if !found {
		t.Error("expected MassiveAuthorized flag for 5.0x ratio")
	}
}

func TestEvaluateRiskFlags_ShelfRegistration(t *testing.T) {
	da := DilutionAnalysis{
		ShelfRegistrations: []FilingSummary{
			{Date: "2024-06-01", Form: "S-3"},
		},
	}
	flags := EvaluateRiskFlags(da, 0, QuoteData{})

	found := false
	for _, f := range flags {
		if f.Flag == FlagShelfReg {
			found = true
		}
	}
	if !found {
		t.Error("expected ShelfReg flag")
	}
}

func TestEvaluateRiskFlags_CleanCompany(t *testing.T) {
	da := DilutionAnalysis{
		DilutionRate12M: -2,
		AuthorizedRatio: 1.5,
	}
	q := QuoteData{ShortFloatPct: 3, IsLowFloat: false}
	flags := EvaluateRiskFlags(da, 10, q)

	if len(flags) != 0 {
		t.Errorf("expected 0 flags for clean company, got %d: %v", len(flags), flags)
	}
}
