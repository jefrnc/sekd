package report

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jefrnc/sekd/internal/analysis"
)

func RenderTerminal(r *analysis.Report) {
	bold := color.New(color.Bold)
	red := color.New(color.FgRed, color.Bold)
	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen, color.Bold)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	bold.Printf("═══ Due Diligence Report: %s ═══\n", strings.ToUpper(r.Ticker))
	cyan.Printf("    %s (CIK: %s)\n\n", r.CompanyName, r.CIK)

	// Company Overview
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetTitle("COMPANY OVERVIEW")
	t.AppendRows([]table.Row{
		{"Sector", r.Sector, "Industry", r.Industry},
		{"Market Cap", r.MarketCap, "Price", r.Price},
		{"Float", r.Float, "Short Float", r.ShortFloat},
		{"Insider Own", r.InsiderOwn, "Inst Own", r.InstOwn},
		{"Volume", r.Volume, "Avg Volume", r.AvgVolume},
	})
	t.SetStyle(table.StyleRounded)
	t.Render()
	fmt.Println()

	// Dilution Analysis
	t2 := table.NewWriter()
	t2.SetOutputMirror(os.Stdout)
	t2.SetTitle("DILUTION ANALYSIS")
	t2.AppendHeader(table.Row{"Metric", "Value"})
	t2.AppendRows([]table.Row{
		{"Dilution Rate (6M)", fmt.Sprintf("%.1f%%", r.Dilution.DilutionRate6M)},
		{"Dilution Rate (12M)", fmt.Sprintf("%.1f%%", r.Dilution.DilutionRate12M)},
		{"Outstanding Shares", formatShares(r.Dilution.OutstandingShares)},
		{"Authorized Shares", formatShares(r.Dilution.AuthorizedShares)},
		{"Authorized/Outstanding", fmt.Sprintf("%.1fx", r.Dilution.AuthorizedRatio)},
		{"ATM Filings (2yr)", fmt.Sprintf("%d", len(r.Dilution.ATMFilings))},
		{"Shelf Registrations (2yr)", fmt.Sprintf("%d", len(r.Dilution.ShelfRegistrations))},
	})
	t2.SetStyle(table.StyleRounded)
	t2.Render()
	fmt.Println()

	// Recent ATM/Shelf filings
	if len(r.Dilution.ATMFilings) > 0 || len(r.Dilution.ShelfRegistrations) > 0 {
		t3 := table.NewWriter()
		t3.SetOutputMirror(os.Stdout)
		t3.SetTitle("RECENT FILINGS")
		t3.AppendHeader(table.Row{"Date", "Form", "Type"})
		for _, f := range r.Dilution.ShelfRegistrations {
			t3.AppendRow(table.Row{f.Date, f.Form, "Shelf Registration"})
		}
		for _, f := range r.Dilution.ATMFilings {
			t3.AppendRow(table.Row{f.Date, f.Form, "ATM Offering"})
		}
		t3.SetStyle(table.StyleRounded)
		t3.Render()
		fmt.Println()
	}

	// Insider Activity
	fmt.Printf("  Insider Transactions (Form 4): %d filings in last %s\n\n", r.Insider.Form4Count, r.Insider.Period)

	// Risk Flags
	if len(r.RiskFlags) > 0 {
		bold.Println("  RISK FLAGS:")
		for _, f := range r.RiskFlags {
			switch f.Severity {
			case "HIGH":
				red.Printf("    [!] %s — %s\n", f.Label, f.Description)
			case "MEDIUM":
				yellow.Printf("    [~] %s — %s\n", f.Label, f.Description)
			default:
				fmt.Printf("    [-] %s — %s\n", f.Label, f.Description)
			}
		}
		fmt.Println()
	} else {
		green.Println("  No significant risk flags detected")
		fmt.Println()
	}

	// Score
	bold.Print("  DD SCORE: ")
	scoreColor := green
	if r.Score.Score < 40 {
		scoreColor = red
	} else if r.Score.Score < 75 {
		scoreColor = yellow
	}
	scoreColor.Printf("%d/100 (Grade: %s)\n", r.Score.Score, r.Score.Grade)
	fmt.Printf("  %s\n\n", r.Score.Summary)
}

func formatShares(n float64) string {
	if n == 0 {
		return "N/A"
	}
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", n/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", n/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.0fK", n/1_000)
	default:
		return fmt.Sprintf("%.0f", n)
	}
}
