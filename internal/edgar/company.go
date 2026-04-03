package edgar

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (c *Client) ResolveCIK(ctx context.Context, ticker string) (string, string, error) {
	data, err := c.get(ctx, tickerURL, 7*24*time.Hour)
	if err != nil {
		return "", "", fmt.Errorf("fetching ticker list: %w", err)
	}

	var tickers map[string]CompanyTicker
	if err := json.Unmarshal(data, &tickers); err != nil {
		return "", "", fmt.Errorf("parsing ticker list: %w", err)
	}

	upper := strings.ToUpper(ticker)
	for _, t := range tickers {
		if strings.ToUpper(t.Ticker) == upper {
			cik := fmt.Sprintf("%010d", t.CIK)
			return cik, t.Title, nil
		}
	}
	return "", "", fmt.Errorf("ticker %q not found in SEC database", ticker)
}

func (c *Client) GetSubmissions(ctx context.Context, cik string) (*Submissions, error) {
	url := fmt.Sprintf("%s/submissions/CIK%s.json", baseURL, cik)
	data, err := c.get(ctx, url, 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("fetching submissions: %w", err)
	}

	var subs Submissions
	if err := json.Unmarshal(data, &subs); err != nil {
		return nil, fmt.Errorf("parsing submissions: %w", err)
	}
	return &subs, nil
}
