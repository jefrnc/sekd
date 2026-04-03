package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/jefrnc/sekd/internal/analysis"
	"github.com/jefrnc/sekd/internal/cache"
	"github.com/jefrnc/sekd/internal/edgar"
	"github.com/jefrnc/sekd/internal/finviz"
	"github.com/jefrnc/sekd/internal/report"
)

type OutputMode int

const (
	ModeTerminal OutputMode = iota
	ModeJSON
	ModeMarkdown
)

type REPL struct {
	cache    *cache.Cache
	edgar    *edgar.Client
	finviz   *finviz.Scraper
	mode     OutputMode
	lastCIK  string
	lastTicker string
	lastFilings []edgar.Filing
}

func NewREPL() (*REPL, error) {
	c, err := cache.New()
	if err != nil {
		return nil, err
	}
	return &REPL{
		cache:  c,
		edgar:  edgar.NewClient(c),
		finviz: finviz.NewScraper(c),
		mode:   ModeTerminal,
	}, nil
}

func (r *REPL) Run(version string) {
	PrintBanner(version)

	dim := color.New(color.FgWhite)
	dim.Println("  Type a ticker to start, /help for commands, /quit to exit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		PrintPrompt()
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			r.handleCommand(input)
			continue
		}

		// Treat as ticker
		r.handleTicker(input)
	}
}

func (r *REPL) handleCommand(input string) {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/quit", "/exit", "/q":
		PrintInfo("Goodbye!")
		os.Exit(0)

	case "/help", "/h":
		PrintHelp()

	case "/clear", "/cls":
		fmt.Print("\033[2J\033[H")

	case "/json":
		if r.mode == ModeJSON {
			r.mode = ModeTerminal
			PrintSuccess("Output mode: terminal")
		} else {
			r.mode = ModeJSON
			PrintSuccess("Output mode: JSON")
		}

	case "/md", "/markdown":
		if r.mode == ModeMarkdown {
			r.mode = ModeTerminal
			PrintSuccess("Output mode: terminal")
		} else {
			r.mode = ModeMarkdown
			PrintSuccess("Output mode: Markdown")
		}

	case "/filings", "/f":
		if len(parts) < 2 {
			if r.lastTicker != "" {
				r.handleFilings(r.lastTicker, "")
			} else {
				PrintError("Usage: /filings TICKER [form-type]")
			}
			return
		}
		formFilter := ""
		if len(parts) >= 3 {
			formFilter = parts[2]
		}
		r.handleFilings(strings.ToUpper(parts[1]), formFilter)

	case "/read", "/r":
		if len(parts) < 2 {
			PrintError("Usage: /read N  or  /read TICKER N")
			return
		}
		if len(parts) == 2 {
			// /read N — use last ticker
			idx, err := strconv.Atoi(parts[1])
			if err != nil {
				PrintError("Invalid index: " + parts[1])
				return
			}
			r.handleRead(r.lastTicker, idx)
		} else {
			idx, err := strconv.Atoi(parts[2])
			if err != nil {
				PrintError("Invalid index: " + parts[2])
				return
			}
			r.handleRead(strings.ToUpper(parts[1]), idx)
		}

	case "/analyze", "/a":
		if len(parts) < 2 {
			PrintError("Usage: /analyze N  or  /analyze TICKER N")
			return
		}
		if len(parts) == 2 {
			idx, err := strconv.Atoi(parts[1])
			if err != nil {
				PrintError("Invalid index: " + parts[1])
				return
			}
			r.handleAnalyze(r.lastTicker, idx)
		} else {
			idx, err := strconv.Atoi(parts[2])
			if err != nil {
				PrintError("Invalid index: " + parts[2])
				return
			}
			r.handleAnalyze(strings.ToUpper(parts[1]), idx)
		}

	default:
		PrintError("Unknown command: " + cmd + ". Type /help for commands.")
	}
}

func (r *REPL) handleTicker(ticker string) {
	ticker = strings.ToUpper(ticker)
	ctx := context.Background()

	PrintInfo("Fetching data for " + ticker + "...")

	builder := report.NewBuilder(r.cache)
	rep, err := builder.Build(ctx, ticker)
	if err != nil {
		PrintError(err.Error())
		return
	}

	switch r.mode {
	case ModeJSON:
		report.RenderJSON(rep)
	case ModeMarkdown:
		report.RenderMarkdown(rep)
	default:
		report.RenderTerminal(rep)
	}

	// Resolve and cache CIK for follow-up commands
	cik, _, _ := r.edgar.ResolveCIK(ctx, ticker)
	r.lastCIK = cik
	r.lastTicker = ticker

	// Check for dilution-related filings and prompt
	r.promptFilings(ctx, ticker, cik, rep)
}

func (r *REPL) promptFilings(ctx context.Context, ticker, cik string, rep *analysis.Report) {
	if len(rep.Dilution.ATMFilings) == 0 && len(rep.Dilution.ShelfRegistrations) == 0 {
		return
	}

	totalFilings := len(rep.Dilution.ATMFilings) + len(rep.Dilution.ShelfRegistrations)
	msg := fmt.Sprintf("Found %d dilution-related filings (ATM/shelf). View them?", totalFilings)

	if AskConfirm(msg) {
		r.handleFilings(ticker, "")
	}
}

func (r *REPL) handleFilings(ticker string, formFilter string) {
	ctx := context.Background()

	if ticker == "" {
		PrintError("No ticker specified. Usage: /filings TICKER")
		return
	}

	cik := r.lastCIK
	if r.lastTicker != ticker {
		var err error
		cik, _, err = r.edgar.ResolveCIK(ctx, ticker)
		if err != nil {
			PrintError(err.Error())
			return
		}
		r.lastCIK = cik
		r.lastTicker = ticker
	}

	formTypes := []string{"S-3", "S-3/A", "424B5", "424B2", "424B3", "10-K", "10-Q", "8-K", "SC 13D", "S-1", "F-3"}
	if formFilter != "" {
		formTypes = []string{formFilter}
		if !strings.HasSuffix(formFilter, "/A") {
			formTypes = append(formTypes, formFilter+"/A")
		}
	}

	PrintInfo("Loading filings for " + ticker + "...")

	filings, err := r.edgar.ListFilings(ctx, cik, formTypes, 20)
	if err != nil {
		PrintError(err.Error())
		return
	}

	r.lastFilings = filings

	if len(filings) == 0 {
		PrintInfo("No filings found.")
		return
	}

	PrintSection(ticker + " — SEC Filings")

	for i, f := range filings {
		formColor := color.New(color.FgWhite)
		switch {
		case f.Form == "S-3" || strings.HasPrefix(f.Form, "424B"):
			formColor = color.New(color.FgRed, color.Bold)
		case f.Form == "10-K" || f.Form == "10-Q":
			formColor = color.New(color.FgGreen)
		case f.Form == "8-K":
			formColor = color.New(color.FgYellow)
		}

		fmt.Printf("  [%2d] %s  ", i, f.FilingDate.Format("2006-01-02"))
		formColor.Printf("%-7s", f.Form)
		fmt.Printf("  %s\n", f.PrimaryDocument)
	}

	fmt.Println()
	dim := color.New(color.FgWhite)
	dim.Println("  Use /read N to view, /analyze N to analyze with AI")
	fmt.Println()
}

func (r *REPL) handleRead(ticker string, idx int) {
	ctx := context.Background()

	if r.lastFilings == nil || r.lastTicker != ticker {
		PrintInfo("Loading filings first...")
		r.handleFilings(ticker, "")
		if r.lastFilings == nil {
			return
		}
	}

	if idx < 0 || idx >= len(r.lastFilings) {
		PrintError(fmt.Sprintf("Index %d out of range (0-%d)", idx, len(r.lastFilings)-1))
		return
	}

	filing := r.lastFilings[idx]
	PrintInfo(fmt.Sprintf("Downloading %s filed %s...", filing.Form, filing.FilingDate.Format("2006-01-02")))

	doc, err := r.edgar.GetFilingDocument(ctx, r.lastCIK, filing)
	if err != nil {
		PrintError(err.Error())
		return
	}

	doc.Ticker = ticker

	text := doc.CleanText
	if len(text) > 5000 {
		text = text[:5000]
	}

	PrintSection(fmt.Sprintf("%s — %s", doc.Form, doc.FilingDate))
	fmt.Printf("  URL: %s\n\n", doc.URL)
	fmt.Println(text)

	if len(doc.CleanText) > 5000 {
		fmt.Println()
		color.New(color.FgYellow).Println("  [Truncated at 5000 chars]")
	}

	// Offer AI analysis
	if DetectAIStatus() != "" {
		if AskConfirm("Analyze this filing with AI?") {
			r.runAIAnalysis(ctx, doc)
		}
	}
}

func (r *REPL) handleAnalyze(ticker string, idx int) {
	ctx := context.Background()

	if r.lastFilings == nil || r.lastTicker != ticker {
		PrintInfo("Loading filings first...")
		r.handleFilings(ticker, "")
		if r.lastFilings == nil {
			return
		}
	}

	if idx < 0 || idx >= len(r.lastFilings) {
		PrintError(fmt.Sprintf("Index %d out of range (0-%d)", idx, len(r.lastFilings)-1))
		return
	}

	filing := r.lastFilings[idx]
	PrintInfo(fmt.Sprintf("Downloading %s filed %s...", filing.Form, filing.FilingDate.Format("2006-01-02")))

	doc, err := r.edgar.GetFilingDocument(ctx, r.lastCIK, filing)
	if err != nil {
		PrintError(err.Error())
		return
	}

	doc.Ticker = ticker
	r.runAIAnalysis(ctx, doc)
}

func (r *REPL) runAIAnalysis(ctx context.Context, doc *edgar.FilingDocument) {
	if DetectAIStatus() == "" {
		PrintError("No AI provider configured. Set OPENAI_API_KEY or ANTHROPIC_API_KEY")
		return
	}

	PrintInfo("Analyzing with AI...")

	result, err := analysis.AnalyzeFiling(ctx, doc)
	if err != nil {
		PrintError(err.Error())
		return
	}

	bold := color.New(color.Bold)
	red := color.New(color.FgRed, color.Bold)
	green := color.New(color.FgGreen)
	dim := color.New(color.FgWhite)

	PrintSection("AI Analysis")

	dim.Printf("  Provider: %s (%s)\n\n", result.Provider, result.Model)

	bold.Println("  Summary:")
	fmt.Printf("  %s\n", result.Summary)

	if result.OfferingAmt != "" {
		fmt.Printf("\n  Offering Amount: %s\n", result.OfferingAmt)
	}
	if result.Warrants != "" {
		fmt.Printf("  Warrants: %s\n", result.Warrants)
	}
	if result.DilutionImpact != "" {
		fmt.Printf("  Dilution Impact: %s\n", result.DilutionImpact)
	}

	if len(result.RedFlags) > 0 {
		fmt.Println()
		red.Println("  Red Flags:")
		for _, flag := range result.RedFlags {
			red.Printf("    [!] %s\n", flag)
		}
	}

	if len(result.KeyTerms) > 0 {
		fmt.Println()
		green.Println("  Key Terms:")
		for _, term := range result.KeyTerms {
			fmt.Printf("    - %s\n", term)
		}
	}
	fmt.Println()
}
