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
	IsLowFloat    bool
	ShortFloatPct float64
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
