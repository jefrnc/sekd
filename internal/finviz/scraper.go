package finviz

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jefrnc/sekd/internal/cache"
)

// quoteCacheTTL controls how long a Finviz quote response is considered
// fresh. Quote-level data (price, volume) drifts within the day, so we
// keep this short. Sector / industry / float move slowly, but we still
// re-fetch them on the same TTL to keep the code simple.
const quoteCacheTTL = 15 * time.Minute

type Scraper struct {
	http  *http.Client
	cache *cache.Cache
}

func NewScraper(c *cache.Cache) *Scraper {
	return &Scraper{
		http:  &http.Client{Timeout: 15 * time.Second},
		cache: c,
	}
}

// get fetches a URL with cache-aware semantics: a cached response within
// TTL is returned immediately, otherwise a fresh HTTP GET populates the
// cache and returns the body. Respects the cache's bypass mode set by
// --no-cache.
func (s *Scraper) get(ctx context.Context, url string) ([]byte, error) {
	if s.cache != nil {
		if data, ok := s.cache.Get(url, quoteCacheTTL); ok {
			return data, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching finviz: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finviz returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading finviz body: %w", err)
	}

	if s.cache != nil {
		_ = s.cache.Set(url, data)
	}
	return data, nil
}

func (s *Scraper) GetQuote(ctx context.Context, ticker string) (*Quote, error) {
	url := fmt.Sprintf("https://finviz.com/quote.ashx?t=%s&ty=c&p=d&b=1", strings.ToUpper(ticker))

	body, err := s.get(ctx, url)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parsing finviz HTML: %w", err)
	}

	snapshot := parseSnapshotTable(doc)

	sector, industry, country := parseSectorInfo(doc)

	return &Quote{
		Ticker:        strings.ToUpper(ticker),
		Company:       getCompanyName(doc),
		Sector:        sector,
		Industry:      industry,
		Country:       country,
		MarketCap:     snapshot["Market capitalization"],
		Price:         snapshot["Current stock price"],
		Float:         snapshot["Shares float"],
		ShortFloat:    snapshot["Short interest share"],
		ShortInterest: snapshot["Short interest"],
		InsiderOwn:    snapshot["Insider ownership"],
		InstOwn:       snapshot["Institutional ownership"],
		AvgVolume:     snapshot["Average volume (3 month)"],
		Volume:        snapshot["Volume"],
		RelVolume:     snapshot["Relative volume"],
	}, nil
}

// parseSnapshotTable extracts data from Finviz's snapshot table.
// Labels are in data-boxover-html attributes, values are in the next sibling td.
func parseSnapshotTable(doc *goquery.Document) map[string]string {
	result := make(map[string]string)

	doc.Find("table.snapshot-table2 td[data-boxover-html]").Each(func(i int, cell *goquery.Selection) {
		label, exists := cell.Attr("data-boxover-html")
		if !exists {
			return
		}
		valueCell := cell.Next()
		if valueCell.Length() == 0 {
			return
		}
		value := strings.TrimSpace(valueCell.Text())
		result[label] = value
	})

	return result
}

func parseSectorInfo(doc *goquery.Document) (sector, industry, country string) {
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}
		if strings.Contains(href, "f=sec_") {
			sector = text
		} else if strings.Contains(href, "f=ind_") {
			industry = text
		} else if strings.Contains(href, "f=geo_") {
			country = text
		}
	})
	return
}

func getCompanyName(doc *goquery.Document) string {
	name := doc.Find("h2.quote-header_ticker-wrapper_company a").First().Text()
	if name = strings.TrimSpace(name); name != "" {
		return name
	}
	title := doc.Find("title").First().Text()
	if idx := strings.Index(title, " Stock "); idx > 0 {
		return strings.TrimSpace(title[:idx])
	}
	return title
}
