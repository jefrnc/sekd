package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jefrnc/sekd/internal/cache"
	"github.com/jefrnc/sekd/internal/edgar"
)

// DeepPromptVersion is bumped whenever the extraction prompt changes so that
// cached extracts from an older prompt version are ignored.
const DeepPromptVersion = "v1"

type Warrant struct {
	Strike      float64 `json:"strike"`
	Shares      float64 `json:"shares"`
	Expiration  string  `json:"expiration,omitempty"`
	Description string  `json:"description,omitempty"`
	InTheMoney  bool    `json:"in_the_money,omitempty"`
}

type Convertible struct {
	ConversionPrice float64 `json:"conversion_price"`
	PrincipalUSD    float64 `json:"principal_usd"`
	Maturity        string  `json:"maturity,omitempty"`
	Description     string  `json:"description,omitempty"`
	InTheMoney      bool    `json:"in_the_money,omitempty"`
}

type DeepSource struct {
	AccessionNumber string `json:"accession_number"`
	Form            string `json:"form"`
	FilingDate      string `json:"filing_date"`
	URL             string `json:"url,omitempty"`
}

type DeepDilution struct {
	ShelfTotalUSD           float64       `json:"shelf_total_usd,omitempty"`
	ShelfUsedUSD            float64       `json:"shelf_used_usd,omitempty"`
	ShelfRemainingUSD       float64       `json:"shelf_remaining_usd,omitempty"`
	ATMCapacityRemainingUSD float64       `json:"atm_capacity_remaining_usd,omitempty"`
	Warrants                []Warrant     `json:"warrants,omitempty"`
	Convertibles            []Convertible `json:"convertibles,omitempty"`
	Notes                   []string      `json:"notes,omitempty"`
	Sources                 []DeepSource  `json:"sources,omitempty"`
	// ITMWarrantShares is the sum of `shares` across warrants whose strike is
	// at or below the current market price. Computed locally, not by the LLM.
	ITMWarrantShares float64 `json:"itm_warrant_shares,omitempty"`
}

// DeepExtract is a single filing's extraction payload. Exported for tests.
type DeepExtract struct {
	ShelfTotalUSD           float64       `json:"shelf_total_usd"`
	ShelfUsedUSD            float64       `json:"shelf_used_usd"`
	ShelfRemainingUSD       float64       `json:"shelf_remaining_usd"`
	ATMCapacityRemainingUSD float64       `json:"atm_capacity_remaining_usd"`
	Warrants                []Warrant     `json:"warrants"`
	Convertibles            []Convertible `json:"convertibles"`
	Notes                   []string      `json:"notes"`
}

const deepPromptHeader = `You are a financial analyst extracting structured dilution data from a single SEC filing.

Read the filing content below and extract ONLY facts explicitly stated in this specific document. If a field is not present, use 0 for numbers and an empty array for lists. Do not guess, do not extrapolate, do not repeat historical warrants that have already been exercised or expired.

Return a valid JSON object matching EXACTLY this schema — no prose, no markdown fences, no explanation:
{
  "shelf_total_usd": 0,
  "shelf_used_usd": 0,
  "shelf_remaining_usd": 0,
  "atm_capacity_remaining_usd": 0,
  "warrants": [
    {"strike": 0, "shares": 0, "expiration": "YYYY-MM-DD", "description": ""}
  ],
  "convertibles": [
    {"conversion_price": 0, "principal_usd": 0, "maturity": "YYYY-MM-DD", "description": ""}
  ],
  "notes": []
}

Field guidance:
- shelf_total_usd: aggregate dollar ceiling of the shelf registration statement (S-3 total).
- shelf_used_usd: dollars already taken down against the shelf via prior takedowns referenced in the filing.
- shelf_remaining_usd: explicit remaining capacity if stated; otherwise 0 and the caller will derive it.
- atm_capacity_remaining_usd: remaining amount available under any At-The-Market sales agreement referenced in the filing.
- warrants: outstanding warrants only. strike in USD, shares = underlying share count.
- convertibles: outstanding convertible notes, preferred stock, or debentures. conversion_price in USD per share, principal_usd = total principal outstanding.
- notes: up to 3 short strings flagging unusual terms (ratchet provisions, floor price resets, MFN clauses, near-term maturities, toxic conversion features).

Filing content:
`

// AnalyzeDeepFiling extracts structured dilution data from a single filing,
// caching the result indefinitely keyed by accession number + prompt version.
// The fromCache return value is true when the result was served from cache
// rather than freshly extracted via the LLM.
func AnalyzeDeepFiling(ctx context.Context, c *cache.Cache, doc *edgar.FilingDocument) (*DeepExtract, bool, error) {
	cacheKey := fmt.Sprintf("deep:%s:%s", doc.AccessionNumber, DeepPromptVersion)
	if data, ok := c.Get(cacheKey, 10*365*24*time.Hour); ok {
		var ex DeepExtract
		if json.Unmarshal(data, &ex) == nil {
			return &ex, true, nil
		}
	}

	text := doc.CleanText
	if len(text) > 40000 {
		text = text[:40000] + "\n\n[... truncated ...]"
	}
	prompt := deepPromptHeader + text

	raw, _, _, err := CallLLMJSON(ctx, prompt, 2048)
	if err != nil {
		return nil, false, err
	}

	ex, err := parseDeepExtract(raw)
	if err != nil {
		return nil, false, err
	}

	if data, err := json.Marshal(ex); err == nil {
		_ = c.Set(cacheKey, data)
	}
	return ex, false, nil
}

func parseDeepExtract(content string) (*DeepExtract, error) {
	content = strings.TrimSpace(content)
	if idx := strings.Index(content, "{"); idx >= 0 {
		if end := strings.LastIndex(content, "}"); end > idx {
			content = content[idx : end+1]
		}
	}
	var ex DeepExtract
	if err := json.Unmarshal([]byte(content), &ex); err != nil {
		return nil, fmt.Errorf("parsing deep extract JSON: %w", err)
	}
	return &ex, nil
}

// MergeDeep combines per-filing extracts into a consolidated DeepDilution.
// `extracts` must be ordered with the most recent filing first; the first
// non-zero value wins for shelf numbers so newer filings override older ones.
// `sources` is the filing metadata in matching order.
func MergeDeep(extracts []*DeepExtract, sources []DeepSource) *DeepDilution {
	if len(extracts) == 0 {
		return nil
	}
	out := &DeepDilution{Sources: sources}
	seenWarrant := make(map[string]bool)
	seenConv := make(map[string]bool)

	for _, ex := range extracts {
		if ex == nil {
			continue
		}
		if out.ShelfTotalUSD == 0 && ex.ShelfTotalUSD > 0 {
			out.ShelfTotalUSD = ex.ShelfTotalUSD
		}
		if out.ShelfUsedUSD == 0 && ex.ShelfUsedUSD > 0 {
			out.ShelfUsedUSD = ex.ShelfUsedUSD
		}
		if out.ShelfRemainingUSD == 0 && ex.ShelfRemainingUSD > 0 {
			out.ShelfRemainingUSD = ex.ShelfRemainingUSD
		}
		if out.ATMCapacityRemainingUSD == 0 && ex.ATMCapacityRemainingUSD > 0 {
			out.ATMCapacityRemainingUSD = ex.ATMCapacityRemainingUSD
		}
		for _, w := range ex.Warrants {
			if w.Strike == 0 && w.Shares == 0 {
				continue
			}
			k := fmt.Sprintf("%.4f|%.0f|%s", w.Strike, w.Shares, w.Expiration)
			if !seenWarrant[k] {
				seenWarrant[k] = true
				out.Warrants = append(out.Warrants, w)
			}
		}
		for _, cv := range ex.Convertibles {
			if cv.ConversionPrice == 0 && cv.PrincipalUSD == 0 {
				continue
			}
			k := fmt.Sprintf("%.4f|%.0f|%s", cv.ConversionPrice, cv.PrincipalUSD, cv.Maturity)
			if !seenConv[k] {
				seenConv[k] = true
				out.Convertibles = append(out.Convertibles, cv)
			}
		}
		for _, n := range ex.Notes {
			if n = strings.TrimSpace(n); n != "" {
				out.Notes = append(out.Notes, n)
			}
		}
	}

	if out.ShelfRemainingUSD == 0 && out.ShelfTotalUSD > 0 && out.ShelfUsedUSD > 0 {
		if d := out.ShelfTotalUSD - out.ShelfUsedUSD; d > 0 {
			out.ShelfRemainingUSD = d
		}
	}
	return out
}

// MarkInTheMoney flags warrants/convertibles whose strike or conversion price
// is at or below the current market price, and populates ITMWarrantShares.
func (d *DeepDilution) MarkInTheMoney(currentPrice float64) {
	if d == nil || currentPrice <= 0 {
		return
	}
	var itmShares float64
	for i := range d.Warrants {
		if d.Warrants[i].Strike > 0 && currentPrice >= d.Warrants[i].Strike {
			d.Warrants[i].InTheMoney = true
			itmShares += d.Warrants[i].Shares
		}
	}
	for i := range d.Convertibles {
		if d.Convertibles[i].ConversionPrice > 0 && currentPrice >= d.Convertibles[i].ConversionPrice {
			d.Convertibles[i].InTheMoney = true
		}
	}
	d.ITMWarrantShares = itmShares
}
