package report

import (
	"context"
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
}

func NewBuilder(c *cache.Cache) *Builder {
	return &Builder{
		edgar:  edgar.NewClient(c),
		finviz: finviz.NewScraper(c),
	}
}

func (b *Builder) Build(ctx context.Context, ticker string) (*analysis.Report, error) {
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

	quoteData := analysis.QuoteData{
		Float:         quote.Float,
		IsLowFloat:    floatVal > 0 && floatVal < 10_000_000,
		ShortFloatPct: shortPct,
	}

	riskFlags := analysis.EvaluateRiskFlags(dilution, form4Count, quoteData)
	score := analysis.CalculateScore(riskFlags)

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
		RiskFlags: riskFlags,
		Score:     score,
	}, nil
}
