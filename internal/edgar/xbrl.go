package edgar

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

var sharesOutstandingKeys = []string{
	"CommonStockSharesOutstanding",
	"WeightedAverageNumberOfShareOutstandingBasic",
	"WeightedAverageNumberOfSharesOutstandingBasic",
	"CommonStockSharesIssued",
}

func (c *Client) GetSharesOutstanding(ctx context.Context, cik string) ([]SharesDatapoint, error) {
	url := fmt.Sprintf("https://data.sec.gov/api/xbrl/companyfacts/CIK%s.json", cik)
	data, err := c.get(ctx, url, 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("fetching XBRL facts: %w", err)
	}

	var facts CompanyFacts
	if err := json.Unmarshal(data, &facts); err != nil {
		return nil, fmt.Errorf("parsing XBRL facts: %w", err)
	}

	for _, key := range sharesOutstandingKeys {
		entry, ok := facts.Facts.USGAAP[key]
		if !ok {
			continue
		}
		points := extractDatapoints(entry)
		if len(points) > 0 {
			return points, nil
		}
	}

	return nil, fmt.Errorf("no shares outstanding data found for CIK %s", cik)
}

func (c *Client) GetSharesAuthorized(ctx context.Context, cik string) (float64, error) {
	url := fmt.Sprintf("https://data.sec.gov/api/xbrl/companyfacts/CIK%s.json", cik)
	data, err := c.get(ctx, url, 24*time.Hour)
	if err != nil {
		return 0, err
	}

	var facts CompanyFacts
	if err := json.Unmarshal(data, &facts); err != nil {
		return 0, err
	}

	entry, ok := facts.Facts.USGAAP["CommonStockSharesAuthorized"]
	if !ok {
		return 0, nil
	}

	points := extractDatapoints(entry)
	if len(points) == 0 {
		return 0, nil
	}
	return points[len(points)-1].Shares, nil
}

func extractDatapoints(entry FactEntry) []SharesDatapoint {
	var points []SharesDatapoint
	unitKey := "shares"
	dps, ok := entry.Units[unitKey]
	if !ok {
		for k, v := range entry.Units {
			unitKey = k
			dps = v
			break
		}
	}

	// Use the most recently filed value for each end date, skipping zeros
	type candidate struct {
		dp    FactDatapoint
		filed time.Time
	}
	best := make(map[string]candidate)
	for _, dp := range dps {
		if dp.Val <= 0 {
			continue
		}
		endDate := dp.End
		filedDate, _ := time.Parse("2006-01-02", dp.Filed)
		if prev, ok := best[endDate]; !ok || filedDate.After(prev.filed) {
			best[endDate] = candidate{dp: dp, filed: filedDate}
		}
	}

	for endDate, c := range best {
		ed, err := time.Parse("2006-01-02", endDate)
		if err != nil {
			continue
		}
		points = append(points, SharesDatapoint{
			Shares: c.dp.Val,
			Date:   ed,
			Form:   c.dp.Form,
			Filed:  c.filed,
		})
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Date.Before(points[j].Date)
	})
	return points
}
