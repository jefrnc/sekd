package edgar

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jefrnc/sekd/internal/cache"
	"golang.org/x/time/rate"
)

const (
	baseURL   = "https://data.sec.gov"
	searchURL = "https://efts.sec.gov/LATEST"
	tickerURL = "https://www.sec.gov/files/company_tickers.json"
	userAgent = "sekd/1.0 (jose@jose.ar)"
)

type Client struct {
	http    *http.Client
	limiter *rate.Limiter
	cache   *cache.Cache
}

func NewClient(c *cache.Cache) *Client {
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		limiter: rate.NewLimiter(rate.Limit(10), 10),
		cache:   c,
	}
}

func (c *Client) get(ctx context.Context, url string, cacheTTL time.Duration) ([]byte, error) {
	if data, ok := c.cache.Get(url, cacheTTL); ok {
		return data, nil
	}

	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SEC EDGAR returned %d for %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	c.cache.Set(url, data)
	return data, nil
}
