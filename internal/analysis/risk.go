package analysis

import (
	"fmt"
	"strings"
	"time"

	"github.com/jefrnc/sekd/internal/edgar"
)

func EvaluateRiskFlags(da DilutionAnalysis, insiderCount int, quote QuoteData) []RiskFlagDetail {
	var flags []RiskFlagDetail

	if da.DilutionRate12M > 20 {
		flags = append(flags, RiskFlagDetail{
			Flag:        FlagHighDilution,
			Label:       "High Dilution",
			Description: fmt.Sprintf("%.1f%% share increase in 12 months", da.DilutionRate12M),
			Severity:    "HIGH",
			Points:      25,
		})
	}

	if HasRecentATM(filingsToConcrete(da.ATMFilings), 90) {
		flags = append(flags, RiskFlagDetail{
			Flag:        FlagRecentATM,
			Label:       "Recent ATM Offering",
			Description: "ATM/shelf filing in the last 90 days",
			Severity:    "HIGH",
			Points:      20,
		})
	}

	if len(da.ShelfRegistrations) > 0 {
		flags = append(flags, RiskFlagDetail{
			Flag:        FlagShelfReg,
			Label:       "Active Shelf Registration",
			Description: fmt.Sprintf("%d S-3 filings in the last 2 years", len(da.ShelfRegistrations)),
			Severity:    "MEDIUM",
			Points:      10,
		})
	}

	if da.AuthorizedRatio > 3 {
		flags = append(flags, RiskFlagDetail{
			Flag:        FlagMassiveAuthorized,
			Label:       "Massive Authorized Shares",
			Description: fmt.Sprintf("Authorized/Outstanding ratio: %.1fx", da.AuthorizedRatio),
			Severity:    "HIGH",
			Points:      15,
		})
	}

	if quote.IsLowFloat {
		flags = append(flags, RiskFlagDetail{
			Flag:        FlagLowFloat,
			Label:       "Low Float",
			Description: fmt.Sprintf("Float: %s", quote.Float),
			Severity:    "MEDIUM",
			Points:      5,
		})
	}

	if quote.ShortFloatPct > 20 {
		flags = append(flags, RiskFlagDetail{
			Flag:        FlagHighShortInt,
			Label:       "High Short Interest",
			Description: fmt.Sprintf("Short float: %.1f%%", quote.ShortFloatPct),
			Severity:    "MEDIUM",
			Points:      10,
		})
	}

	return flags
}

type QuoteData struct {
	Float         string
	FloatShares   float64
	IsLowFloat    bool
	ShortFloatPct float64
	PriceUSD      float64
}

// EvaluateDeepRiskFlags returns additional risk flags derived from deep
// dilution data (warrants ITM, large shelf remaining). Only called when
// --deep data is present; otherwise these flags are skipped entirely.
func EvaluateDeepRiskFlags(d *DeepDilution, quote QuoteData, marketCap float64) []RiskFlagDetail {
	if d == nil {
		return nil
	}
	var flags []RiskFlagDetail

	if d.ITMWarrantShares > 0 {
		severity := "MEDIUM"
		points := 10
		desc := fmt.Sprintf("%s warrant shares at or below current price", formatShareCount(d.ITMWarrantShares))
		if quote.FloatShares > 0 {
			pct := d.ITMWarrantShares / quote.FloatShares * 100
			desc = fmt.Sprintf("%s warrant shares ITM (%.1f%% of float)", formatShareCount(d.ITMWarrantShares), pct)
			if pct >= 5 {
				severity = "HIGH"
				points = 20
			}
		}
		flags = append(flags, RiskFlagDetail{
			Flag:        FlagWarrantsITM,
			Label:       "Warrants In The Money",
			Description: desc,
			Severity:    severity,
			Points:      points,
		})
	}

	if d.ShelfRemainingUSD > 0 && marketCap > 0 {
		ratio := d.ShelfRemainingUSD / marketCap
		if ratio >= 0.25 {
			severity := "MEDIUM"
			points := 10
			if ratio >= 0.5 {
				severity = "HIGH"
				points = 15
			}
			flags = append(flags, RiskFlagDetail{
				Flag:        FlagShelfCapacity,
				Label:       "Large Shelf Capacity Remaining",
				Description: fmt.Sprintf("$%s shelf remaining (%.0f%% of market cap)", formatUSDCompact(d.ShelfRemainingUSD), ratio*100),
				Severity:    severity,
				Points:      points,
			})
		}
	}

	return flags
}

func formatShareCount(n float64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", n/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.0fK", n/1_000)
	default:
		return fmt.Sprintf("%.0f", n)
	}
}

func formatUSDCompact(n float64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", n/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", n/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.0fK", n/1_000)
	default:
		return fmt.Sprintf("%.0f", n)
	}
}

func ParseFloatString(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0, false
	}
	s = strings.ReplaceAll(s, ",", "")
	multiplier := 1.0
	if strings.HasSuffix(s, "M") {
		multiplier = 1_000_000
		s = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "B") {
		multiplier = 1_000_000_000
		s = strings.TrimSuffix(s, "B")
	} else if strings.HasSuffix(s, "K") {
		multiplier = 1_000
		s = strings.TrimSuffix(s, "K")
	}
	var val float64
	_, err := fmt.Sscanf(s, "%f", &val)
	if err != nil {
		return 0, false
	}
	return val * multiplier, true
}

func ParsePercentString(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	var val float64
	fmt.Sscanf(s, "%f", &val)
	return val
}

func filingsToConcrete(summaries []FilingSummary) []edgar.Filing {
	var filings []edgar.Filing
	for _, s := range summaries {
		t, err := time.Parse("2006-01-02", s.Date)
		if err != nil {
			continue
		}
		filings = append(filings, edgar.Filing{
			FilingDate: t,
			Form:       s.Form,
		})
	}
	return filings
}
