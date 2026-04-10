package report

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/jefrnc/sekd/internal/analysis"
	"github.com/jefrnc/sekd/internal/cache"
	"github.com/jefrnc/sekd/internal/edgar"
	"github.com/jefrnc/sekd/internal/finviz"
	"golang.org/x/sync/errgroup"
)

type Builder struct {
	edgar  *edgar.Client
	finviz *finviz.Scraper
	cache  *cache.Cache
}

// BuildOptions controls optional enrichment for a report build.
type BuildOptions struct {
	// Deep enables LLM-based extraction of shelf capacity, ATM remaining,
	// warrants, and convertibles from recent S-3 / 10-Q / 424B5 filings.
	// Requires an AI provider (OpenAI or Anthropic) to be configured.
	Deep bool
}

func NewBuilder(c *cache.Cache) *Builder {
	return &Builder{
		edgar:  edgar.NewClient(c),
		finviz: finviz.NewScraper(c),
		cache:  c,
	}
}

func (b *Builder) Build(ctx context.Context, ticker string) (*analysis.Report, error) {
	return b.BuildWithOptions(ctx, ticker, BuildOptions{})
}

func (b *Builder) BuildWithOptions(ctx context.Context, ticker string, opts BuildOptions) (*analysis.Report, error) {
	cik, companyName, err := b.edgar.ResolveCIK(ctx, ticker)
	if err != nil {
		return nil, err
	}

	var (
		subs      *edgar.Submissions
		shares    []edgar.SharesDatapoint
		authorized float64
		quote     *finviz.Quote
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		subs, err = b.edgar.GetSubmissions(gctx, cik)
		return err
	})

	g.Go(func() error {
		var err error
		shares, err = b.edgar.GetSharesOutstanding(gctx, cik)
		if err != nil {
			shares = nil // non-fatal
		}
		return nil
	})

	g.Go(func() error {
		var err error
		authorized, err = b.edgar.GetSharesAuthorized(gctx, cik)
		if err != nil {
			authorized = 0
		}
		return nil
	})

	g.Go(func() error {
		var err error
		quote, err = b.finviz.GetQuote(gctx, ticker)
		if err != nil {
			quote = &finviz.Quote{Ticker: ticker}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	since2y := time.Now().AddDate(-2, 0, 0)
	since6m := time.Now().AddDate(0, -6, 0)

	atmFilings := edgar.FilterFilings(subs, []string{"424B5", "424B2"}, since2y)
	shelfFilings := edgar.FilterFilings(subs, []string{"S-3", "S-3/A"}, since2y)
	form4Count := edgar.CountForm4Filings(subs, since6m)

	dilution := analysis.AnalyzeDilution(shares, authorized, atmFilings, shelfFilings)

	floatVal, _ := analysis.ParseFloatString(quote.Float)
	shortPct := analysis.ParsePercentString(quote.ShortFloat)
	priceVal := parsePriceString(quote.Price)
	marketCap, _ := analysis.ParseFloatString(quote.MarketCap)

	quoteData := analysis.QuoteData{
		Float:         quote.Float,
		FloatShares:   floatVal,
		IsLowFloat:    floatVal > 0 && floatVal < 10_000_000,
		ShortFloatPct: shortPct,
		PriceUSD:      priceVal,
	}

	riskFlags := analysis.EvaluateRiskFlags(dilution, form4Count, quoteData)

	var deep *analysis.DeepDilution
	if opts.Deep {
		d, err := b.buildDeep(ctx, cik, priceVal)
		if err != nil {
			fmt.Fprintf(os.Stderr, "deep analysis failed: %v\n", err)
		} else {
			deep = d
			riskFlags = append(riskFlags, analysis.EvaluateDeepRiskFlags(deep, quoteData, marketCap)...)
		}
	}

	score := analysis.CalculateScore(riskFlags)

	latestAccession, latestForm, latestDate := latestFiling(subs)

	return &analysis.Report{
		Ticker:      ticker,
		CompanyName: companyName,
		CIK:        cik,
		Sector:     quote.Sector,
		Industry:   quote.Industry,
		Country:    quote.Country,
		MarketCap:  quote.MarketCap,
		Price:      quote.Price,
		Float:      quote.Float,
		ShortFloat: quote.ShortFloat,
		InsiderOwn: quote.InsiderOwn,
		InstOwn:    quote.InstOwn,
		Volume:     quote.Volume,
		AvgVolume:  quote.AvgVolume,
		RelVolume:  quote.RelVolume,
		Dilution:   dilution,
		Insider: analysis.InsiderSummary{
			Form4Count: form4Count,
			Period:     "6 months",
		},
		RiskFlags:        riskFlags,
		Score:            score,
		Deep:             deep,
		LatestAccession:  latestAccession,
		LatestFilingForm: latestForm,
		LatestFilingDate: latestDate,
	}, nil
}

// latestFiling returns the accession number, form type, and filing date of
// the most recent filing in the submissions feed. Used so that watchlist
// scan can detect "anything new since last time" cheaply.
func latestFiling(subs *edgar.Submissions) (accession, form, date string) {
	if subs == nil {
		return "", "", ""
	}
	r := subs.Filings.Recent
	if len(r.AccessionNumber) == 0 {
		return "", "", ""
	}
	// The EDGAR submissions feed is ordered most-recent first.
	accession = r.AccessionNumber[0]
	if len(r.Form) > 0 {
		form = r.Form[0]
	}
	if len(r.FilingDate) > 0 {
		date = r.FilingDate[0]
	}
	return
}

// buildDeep selects relevant filings (active shelves, recent takedowns, latest
// 10-Q) and runs the LLM extractor against each, then merges the results.
// Each filing's extract is cached indefinitely by accession number.
func (b *Builder) buildDeep(ctx context.Context, cik string, currentPrice float64) (*analysis.DeepDilution, error) {
	shelves, err := b.edgar.ListFilings(ctx, cik, []string{"S-3", "S-3/A"}, 2)
	if err != nil {
		return nil, fmt.Errorf("listing shelves: %w", err)
	}
	takedowns, err := b.edgar.ListFilings(ctx, cik, []string{"424B5", "424B2", "424B3"}, 5)
	if err != nil {
		return nil, fmt.Errorf("listing takedowns: %w", err)
	}
	// Only include takedowns filed in the last 180 days
	cutoff := time.Now().AddDate(0, -6, 0)
	var recentTakedowns []edgar.Filing
	for _, f := range takedowns {
		if f.FilingDate.After(cutoff) {
			recentTakedowns = append(recentTakedowns, f)
		}
	}
	latest10Q, err := b.edgar.ListFilings(ctx, cik, []string{"10-Q"}, 1)
	if err != nil {
		latest10Q = nil
	}

	var filings []edgar.Filing
	filings = append(filings, shelves...)
	filings = append(filings, recentTakedowns...)
	filings = append(filings, latest10Q...)

	// Most recent first — merge logic takes the first non-zero value per field.
	sort.Slice(filings, func(i, j int) bool {
		return filings[i].FilingDate.After(filings[j].FilingDate)
	})

	if len(filings) == 0 {
		return nil, fmt.Errorf("no relevant filings found for deep analysis")
	}

	fmt.Fprintf(os.Stderr, "  deep: extracting from %d filings...\n", len(filings))

	var extracts []*analysis.DeepExtract
	var sources []analysis.DeepSource
	cacheHits, llmCalls := 0, 0
	for _, f := range filings {
		doc, err := b.edgar.GetFilingDocument(ctx, cik, f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  deep: skipping %s (%s): %v\n", f.Form, f.AccessionNumber, err)
			continue
		}
		ex, fromCache, err := analysis.AnalyzeDeepFiling(ctx, b.cache, doc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  deep: LLM failed on %s: %v\n", f.Form, err)
			continue
		}
		if fromCache {
			cacheHits++
		} else {
			llmCalls++
		}
		extracts = append(extracts, ex)
		sources = append(sources, analysis.DeepSource{
			AccessionNumber: f.AccessionNumber,
			Form:            f.Form,
			FilingDate:      f.FilingDate.Format("2006-01-02"),
			URL:             doc.URL,
		})
	}
	fmt.Fprintf(os.Stderr, "  deep: %d from cache, %d fresh LLM calls\n", cacheHits, llmCalls)

	deep := analysis.MergeDeep(extracts, sources)
	if deep != nil {
		deep.MarkInTheMoney(currentPrice)
	}
	return deep, nil
}

func parsePriceString(s string) float64 {
	if s == "" || s == "-" {
		return 0
	}
	// Finviz prices come as plain "12.34"; strip $ just in case
	clean := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || c == '.' {
			clean = append(clean, c)
		}
	}
	var v float64
	fmt.Sscanf(string(clean), "%f", &v)
	return v
}
