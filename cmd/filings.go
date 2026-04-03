package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jefrnc/sekd/internal/analysis"
	"github.com/jefrnc/sekd/internal/cache"
	"github.com/jefrnc/sekd/internal/edgar"
	"github.com/spf13/cobra"
)

var (
	formType        string
	analyze         bool
	listOnly        bool
	filingIdx       int
	maxChars        int
	filingsJSON     bool
	filingsMD       bool
)

var filingsCmd = &cobra.Command{
	Use:   "filings [ticker]",
	Short: "Browse and analyze SEC filing documents",
	Long: `List SEC filings for a ticker, view filing content as clean text,
or analyze with AI (requires OPENAI_API_KEY or ANTHROPIC_API_KEY).

Examples:
  sekd filings SOUN                       # List recent filings
  sekd filings SOUN --type S-3             # List only S-3 filings
  sekd filings SOUN --type S-3 --read 0    # Read first S-3 filing
  sekd filings SOUN --type S-3 --read 0 --analyze  # Analyze with AI`,
	Args: cobra.ExactArgs(1),
	RunE: runFilings,
}

func init() {
	filingsCmd.Flags().StringVar(&formType, "type", "", "Filter by form type (S-3, 424B5, 10-K, 10-Q, 8-K)")
	filingsCmd.Flags().BoolVar(&analyze, "analyze", false, "Analyze filing content with AI (requires API key)")
	filingsCmd.Flags().IntVar(&filingIdx, "read", -1, "Read filing at index (from list)")
	filingsCmd.Flags().IntVar(&maxChars, "max-chars", 5000, "Max characters to display (0 = unlimited)")
	filingsCmd.Flags().BoolVar(&filingsJSON, "json", false, "Output as JSON")
	filingsCmd.Flags().BoolVar(&filingsMD, "md", false, "Output as Markdown")
	rootCmd.AddCommand(filingsCmd)
}

func runFilings(cmd *cobra.Command, args []string) error {
	ticker := strings.ToUpper(args[0])
	ctx := context.Background()

	c, err := cache.New()
	if err != nil {
		return err
	}

	client := edgar.NewClient(c)

	fmt.Fprintf(os.Stderr, "Resolving %s...\n", ticker)
	cik, companyName, err := client.ResolveCIK(ctx, ticker)
	if err != nil {
		return err
	}

	// Determine form types to search
	formTypes := []string{"S-3", "S-3/A", "424B5", "424B2", "424B3", "10-K", "10-Q", "8-K", "SC 13D", "S-1", "F-3"}
	if formType != "" {
		formTypes = []string{formType}
		// Include amendments
		if !strings.HasSuffix(formType, "/A") {
			formTypes = append(formTypes, formType+"/A")
		}
	}

	filings, err := client.ListFilings(ctx, cik, formTypes, 20)
	if err != nil {
		return err
	}

	if len(filings) == 0 {
		fmt.Println("No filings found.")
		return nil
	}

	// If no --read flag, show the list
	if filingIdx < 0 {
		if filingsJSON {
			return renderFilingsListJSON(ticker, companyName, cik, filings)
		}
		if filingsMD {
			return renderFilingsListMD(ticker, companyName, filings)
		}
		return renderFilingsList(ticker, companyName, filings)
	}

	// Read specific filing
	if filingIdx >= len(filings) {
		return fmt.Errorf("index %d out of range (0-%d)", filingIdx, len(filings)-1)
	}

	filing := filings[filingIdx]
	fmt.Fprintf(os.Stderr, "Downloading %s filed %s...\n", filing.Form, filing.FilingDate.Format("2006-01-02"))

	doc, err := client.GetFilingDocument(ctx, cik, filing)
	if err != nil {
		return fmt.Errorf("downloading filing: %w", err)
	}

	doc.Ticker = ticker
	doc.CompanyName = companyName

	// Analyze with AI if requested
	if analyze {
		aiResult, err := runAnalysis(ctx, doc)
		if err != nil {
			return err
		}
		switch {
		case filingsJSON:
			return renderAnalysisJSON(doc, aiResult)
		case filingsMD:
			renderAnalysisMD(doc, aiResult)
			return nil
		default:
			renderAnalysisTerminal(doc, aiResult)
			return nil
		}
	}

	// Just show the text
	switch {
	case filingsJSON:
		return renderFilingTextJSON(doc)
	case filingsMD:
		renderFilingTextMD(doc)
		return nil
	default:
		return renderFilingText(doc)
	}
}

func renderFilingsList(ticker, company string, filings []edgar.Filing) error {
	bold := color.New(color.Bold)
	bold.Printf("\n%s — %s — SEC Filings\n\n", ticker, company)

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "Date", "Form", "Document"})

	for i, f := range filings {
		t.AppendRow(table.Row{
			i,
			f.FilingDate.Format("2006-01-02"),
			f.Form,
			f.PrimaryDocument,
		})
	}
	t.SetStyle(table.StyleRounded)
	t.Render()

	fmt.Println()
	color.New(color.FgCyan).Println("  Use --read N to view a filing. Example:")
	fmt.Printf("  sekd filings %s --read 0\n", ticker)
	fmt.Printf("  sekd filings %s --read 0 --analyze\n\n", ticker)
	return nil
}

func renderFilingText(doc *edgar.FilingDocument) error {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	bold.Printf("═══ %s — %s ═══\n", doc.Form, doc.FilingDate)
	cyan.Printf("    %s (%s)\n", doc.CompanyName, doc.Ticker)
	fmt.Printf("    URL: %s\n\n", doc.URL)

	text := doc.CleanText
	if maxChars > 0 && len(text) > maxChars {
		text = text[:maxChars]
		fmt.Println(text)
		fmt.Println()
		color.New(color.FgYellow).Printf("    [Truncated at %d chars. Use --max-chars 0 for full text]\n\n", maxChars)
	} else {
		fmt.Println(text)
	}

	return nil
}

func runAnalysis(ctx context.Context, doc *edgar.FilingDocument) (*analysis.AIAnalysis, error) {
	provider, _ := analysis.DetectProvider()
	if provider == "" {
		return nil, fmt.Errorf("no AI provider configured.\nSet OPENAI_API_KEY or ANTHROPIC_API_KEY environment variable")
	}

	fmt.Fprintf(os.Stderr, "Analyzing with %s...\n", provider)
	result, err := analysis.AnalyzeFiling(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}
	return result, nil
}

func renderAnalysisTerminal(doc *edgar.FilingDocument, result *analysis.AIAnalysis) {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed, color.Bold)
	green := color.New(color.FgGreen)

	fmt.Println()
	bold.Printf("═══ AI Analysis: %s — %s ═══\n", doc.Form, doc.FilingDate)
	cyan.Printf("    %s (%s)\n", doc.CompanyName, doc.Ticker)
	fmt.Printf("    Provider: %s (%s)\n", result.Provider, result.Model)
	fmt.Printf("    URL: %s\n\n", doc.URL)

	bold.Println("  Summary:")
	fmt.Printf("  %s\n\n", result.Summary)

	if result.OfferingAmt != "" {
		fmt.Printf("  Offering Amount: %s\n", result.OfferingAmt)
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

func renderAnalysisJSON(doc *edgar.FilingDocument, result *analysis.AIAnalysis) error {
	output := map[string]interface{}{
		"ticker":       doc.Ticker,
		"company":      doc.CompanyName,
		"cik":          doc.CIK,
		"form":         doc.Form,
		"filing_date":  doc.FilingDate,
		"url":          doc.URL,
		"analysis":     result,
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func renderAnalysisMD(doc *edgar.FilingDocument, result *analysis.AIAnalysis) {
	fmt.Printf("# AI Analysis: %s — %s\n\n", doc.Form, doc.FilingDate)
	fmt.Printf("**%s** (%s) | CIK: %s\n\n", doc.CompanyName, doc.Ticker, doc.CIK)
	fmt.Printf("Provider: %s (%s)\n\n", result.Provider, result.Model)
	fmt.Printf("URL: %s\n\n", doc.URL)

	fmt.Println("## Summary")
	fmt.Println()
	fmt.Printf("%s\n\n", result.Summary)

	fmt.Println("## Details")
	fmt.Println()
	fmt.Println("| Field | Value |")
	fmt.Println("|-------|-------|")
	if result.OfferingAmt != "" {
		fmt.Printf("| Offering Amount | %s |\n", result.OfferingAmt)
	}
	if result.Warrants != "" {
		fmt.Printf("| Warrants | %s |\n", result.Warrants)
	}
	if result.DilutionImpact != "" {
		fmt.Printf("| Dilution Impact | %s |\n", result.DilutionImpact)
	}
	fmt.Println()

	if len(result.RedFlags) > 0 {
		fmt.Println("## Red Flags")
		fmt.Println()
		for _, flag := range result.RedFlags {
			fmt.Printf("- 🔴 %s\n", flag)
		}
		fmt.Println()
	}

	if len(result.KeyTerms) > 0 {
		fmt.Println("## Key Terms")
		fmt.Println()
		for _, term := range result.KeyTerms {
			fmt.Printf("- %s\n", term)
		}
		fmt.Println()
	}
}

func renderFilingsListJSON(ticker, company, cik string, filings []edgar.Filing) error {
	type entry struct {
		Index   int    `json:"index"`
		Date    string `json:"date"`
		Form    string `json:"form"`
		Document string `json:"document"`
		Accession string `json:"accession"`
	}
	var entries []entry
	for i, f := range filings {
		entries = append(entries, entry{
			Index:    i,
			Date:     f.FilingDate.Format("2006-01-02"),
			Form:     f.Form,
			Document: f.PrimaryDocument,
			Accession: f.AccessionNumber,
		})
	}
	output := map[string]interface{}{
		"ticker":  ticker,
		"company": company,
		"cik":     cik,
		"filings": entries,
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func renderFilingsListMD(ticker, company string, filings []edgar.Filing) error {
	fmt.Printf("# %s — %s — SEC Filings\n\n", ticker, company)
	fmt.Println("| # | Date | Form | Document |")
	fmt.Println("|---|------|------|----------|")
	for i, f := range filings {
		fmt.Printf("| %d | %s | %s | %s |\n", i, f.FilingDate.Format("2006-01-02"), f.Form, f.PrimaryDocument)
	}
	fmt.Println()
	return nil
}

func renderFilingTextJSON(doc *edgar.FilingDocument) error {
	text := doc.CleanText
	if maxChars > 0 && len(text) > maxChars {
		text = text[:maxChars]
	}
	output := map[string]interface{}{
		"ticker":      doc.Ticker,
		"company":     doc.CompanyName,
		"cik":         doc.CIK,
		"form":        doc.Form,
		"filing_date": doc.FilingDate,
		"url":         doc.URL,
		"content":     text,
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func renderFilingTextMD(doc *edgar.FilingDocument) {
	fmt.Printf("# %s — %s\n\n", doc.Form, doc.FilingDate)
	fmt.Printf("**%s** (%s)\n\n", doc.CompanyName, doc.Ticker)
	fmt.Printf("URL: %s\n\n", doc.URL)
	fmt.Println("---")
	fmt.Println()

	text := doc.CleanText
	if maxChars > 0 && len(text) > maxChars {
		text = text[:maxChars]
		fmt.Println(text)
		fmt.Printf("\n\n*[Truncated at %d chars. Use --max-chars 0 for full text]*\n", maxChars)
	} else {
		fmt.Println(text)
	}
}
