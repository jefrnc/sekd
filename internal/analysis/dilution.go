package analysis

import (
	"time"

	"github.com/jefrnc/sekd/internal/edgar"
)

func AnalyzeDilution(shares []edgar.SharesDatapoint, authorized float64, atmFilings, shelfFilings []edgar.Filing) DilutionAnalysis {
	da := DilutionAnalysis{
		AuthorizedShares: authorized,
	}

	for _, s := range shares {
		da.SharesHistory = append(da.SharesHistory, SharesEntry{
			Date:   s.Date.Format("2006-01-02"),
			Shares: s.Shares,
			Form:   s.Form,
		})
	}

	if len(shares) > 0 {
		da.OutstandingShares = shares[len(shares)-1].Shares
	}

	if authorized > 0 && da.OutstandingShares > 0 {
		da.AuthorizedRatio = authorized / da.OutstandingShares
	}

	da.DilutionRate6M = calcDilutionRate(shares, 6)
	da.DilutionRate12M = calcDilutionRate(shares, 12)

	for _, f := range atmFilings {
		da.ATMFilings = append(da.ATMFilings, FilingSummary{
			Date: f.FilingDate.Format("2006-01-02"),
			Form: f.Form,
		})
	}
	for _, f := range shelfFilings {
		da.ShelfRegistrations = append(da.ShelfRegistrations, FilingSummary{
			Date: f.FilingDate.Format("2006-01-02"),
			Form: f.Form,
		})
	}

	return da
}

func calcDilutionRate(shares []edgar.SharesDatapoint, months int) float64 {
	if len(shares) < 2 {
		return 0
	}

	latest := shares[len(shares)-1]
	cutoff := latest.Date.AddDate(0, -months, 0)

	var earliest *edgar.SharesDatapoint
	for i := range shares {
		if shares[i].Date.After(cutoff) || shares[i].Date.Equal(cutoff) {
			earliest = &shares[i]
			break
		}
	}
	if earliest == nil || earliest.Shares == 0 {
		return 0
	}

	return ((latest.Shares - earliest.Shares) / earliest.Shares) * 100
}

func HasRecentATM(atmFilings []edgar.Filing, days int) bool {
	cutoff := time.Now().AddDate(0, 0, -days)
	for _, f := range atmFilings {
		if f.FilingDate.After(cutoff) {
			return true
		}
	}
	return false
}
