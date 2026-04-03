package edgar

import (
	"strings"
	"time"
)

func FilterFilings(subs *Submissions, formTypes []string, since time.Time) []Filing {
	recent := subs.Filings.Recent
	typeSet := make(map[string]bool)
	for _, ft := range formTypes {
		typeSet[strings.ToUpper(ft)] = true
	}

	var results []Filing
	for i := range recent.Form {
		if i >= len(recent.FilingDate) || i >= len(recent.AccessionNumber) {
			break
		}
		if !typeSet[strings.ToUpper(recent.Form[i])] {
			continue
		}
		date, err := time.Parse("2006-01-02", recent.FilingDate[i])
		if err != nil || date.Before(since) {
			continue
		}
		primaryDoc := ""
		if i < len(recent.PrimaryDocument) {
			primaryDoc = recent.PrimaryDocument[i]
		}
		results = append(results, Filing{
			AccessionNumber: recent.AccessionNumber[i],
			FilingDate:      date,
			Form:            recent.Form[i],
			PrimaryDocument: primaryDoc,
		})
	}
	return results
}

func CountForm4Filings(subs *Submissions, since time.Time) int {
	count := 0
	recent := subs.Filings.Recent
	for i := range recent.Form {
		if i >= len(recent.FilingDate) {
			break
		}
		if strings.ToUpper(recent.Form[i]) != "4" {
			continue
		}
		date, err := time.Parse("2006-01-02", recent.FilingDate[i])
		if err != nil || date.Before(since) {
			continue
		}
		count++
	}
	return count
}
