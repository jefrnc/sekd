package analysis

import (
	"testing"
	"time"

	"github.com/jefrnc/sekd/internal/edgar"
)

func makeShares(dates []string, values []float64) []edgar.SharesDatapoint {
	var points []edgar.SharesDatapoint
	for i, d := range dates {
		t, _ := time.Parse("2006-01-02", d)
		points = append(points, edgar.SharesDatapoint{
			Shares: values[i],
			Date:   t,
			Form:   "10-Q",
		})
	}
	return points
}

func TestAnalyzeDilution_Basic(t *testing.T) {
	shares := makeShares(
		[]string{"2023-06-30", "2023-09-30", "2023-12-31", "2024-03-31", "2024-06-30"},
		[]float64{100_000_000, 110_000_000, 120_000_000, 130_000_000, 150_000_000},
	)

	da := AnalyzeDilution(shares, 500_000_000, nil, nil)

	if da.OutstandingShares != 150_000_000 {
		t.Errorf("OutstandingShares = %v, want 150M", da.OutstandingShares)
	}
	if da.AuthorizedRatio != 500_000_000.0/150_000_000.0 {
		t.Errorf("AuthorizedRatio = %v, want ~3.33", da.AuthorizedRatio)
	}
	if da.DilutionRate12M <= 0 {
		t.Errorf("DilutionRate12M = %v, want > 0", da.DilutionRate12M)
	}
}

func TestAnalyzeDilution_NegativeDilution(t *testing.T) {
	// Buyback — shares decrease
	shares := makeShares(
		[]string{"2023-06-30", "2024-06-30"},
		[]float64{200_000_000, 190_000_000},
	)

	da := AnalyzeDilution(shares, 0, nil, nil)

	if da.DilutionRate12M >= 0 {
		t.Errorf("DilutionRate12M = %v, want negative (buyback)", da.DilutionRate12M)
	}
}

func TestAnalyzeDilution_EmptyShares(t *testing.T) {
	da := AnalyzeDilution(nil, 0, nil, nil)

	if da.OutstandingShares != 0 {
		t.Errorf("OutstandingShares = %v, want 0", da.OutstandingShares)
	}
	if da.DilutionRate12M != 0 {
		t.Errorf("DilutionRate12M = %v, want 0", da.DilutionRate12M)
	}
}

func TestAnalyzeDilution_SingleDatapoint(t *testing.T) {
	shares := makeShares([]string{"2024-06-30"}, []float64{100_000_000})

	da := AnalyzeDilution(shares, 0, nil, nil)

	if da.DilutionRate12M != 0 {
		t.Errorf("DilutionRate12M = %v, want 0 (single point)", da.DilutionRate12M)
	}
}

func TestAnalyzeDilution_FilingsCounted(t *testing.T) {
	atm := []edgar.Filing{
		{FilingDate: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), Form: "424B5"},
		{FilingDate: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), Form: "424B5"},
	}
	shelf := []edgar.Filing{
		{FilingDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Form: "S-3"},
	}

	da := AnalyzeDilution(nil, 0, atm, shelf)

	if len(da.ATMFilings) != 2 {
		t.Errorf("ATMFilings count = %d, want 2", len(da.ATMFilings))
	}
	if len(da.ShelfRegistrations) != 1 {
		t.Errorf("ShelfRegistrations count = %d, want 1", len(da.ShelfRegistrations))
	}
}

func TestHasRecentATM_Recent(t *testing.T) {
	filings := []edgar.Filing{
		{FilingDate: time.Now().AddDate(0, 0, -30)},
	}
	if !HasRecentATM(filings, 90) {
		t.Error("expected true for filing 30 days ago with 90 day window")
	}
}

func TestHasRecentATM_Old(t *testing.T) {
	filings := []edgar.Filing{
		{FilingDate: time.Now().AddDate(0, 0, -120)},
	}
	if HasRecentATM(filings, 90) {
		t.Error("expected false for filing 120 days ago with 90 day window")
	}
}

func TestHasRecentATM_Empty(t *testing.T) {
	if HasRecentATM(nil, 90) {
		t.Error("expected false for nil filings")
	}
}

func TestCalcDilutionRate_50Percent(t *testing.T) {
	shares := makeShares(
		[]string{"2023-06-30", "2024-06-30"},
		[]float64{100_000_000, 150_000_000},
	)
	rate := calcDilutionRate(shares, 12)
	if rate != 50.0 {
		t.Errorf("dilution rate = %v, want 50.0", rate)
	}
}

func TestCalcDilutionRate_ZeroStart(t *testing.T) {
	shares := makeShares(
		[]string{"2023-06-30", "2024-06-30"},
		[]float64{0, 150_000_000},
	)
	rate := calcDilutionRate(shares, 12)
	if rate != 0 {
		t.Errorf("dilution rate = %v, want 0 (zero starting shares)", rate)
	}
}
