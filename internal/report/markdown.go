package report

import (
	"fmt"
	"strings"

	"github.com/jefrnc/sekd/internal/analysis"
)

func RenderMarkdown(r *analysis.Report) {
	fmt.Printf("# Due Diligence Report: %s\n\n", strings.ToUpper(r.Ticker))
	fmt.Printf("**%s** (CIK: %s)\n\n", r.CompanyName, r.CIK)

	fmt.Println("## Company Overview")
	fmt.Println()
	fmt.Println("| Field | Value | Field | Value |")
	fmt.Println("|-------|-------|-------|-------|")
	fmt.Printf("| Sector | %s | Industry | %s |\n", r.Sector, r.Industry)
	fmt.Printf("| Country | %s | Market Cap | %s |\n", r.Country, r.MarketCap)
	fmt.Printf("| Price | %s | Float | %s |\n", r.Price, r.Float)
	fmt.Printf("| Short Float | %s | Insider Own | %s |\n", r.ShortFloat, r.InsiderOwn)
	fmt.Printf("| Inst Own | %s | Volume | %s |\n", r.InstOwn, r.Volume)
	fmt.Printf("| Avg Volume | %s | Rel Volume | %s |\n", r.AvgVolume, r.RelVolume)
	fmt.Println()

	fmt.Println("## Dilution Analysis")
	fmt.Println()
	fmt.Println("| Metric | Value |")
	fmt.Println("|--------|-------|")
	fmt.Printf("| Dilution Rate (6M) | %.1f%% |\n", r.Dilution.DilutionRate6M)
	fmt.Printf("| Dilution Rate (12M) | %.1f%% |\n", r.Dilution.DilutionRate12M)
	fmt.Printf("| Outstanding Shares | %s |\n", formatShares(r.Dilution.OutstandingShares))
	fmt.Printf("| Authorized Shares | %s |\n", formatShares(r.Dilution.AuthorizedShares))
	fmt.Printf("| Authorized/Outstanding | %.1fx |\n", r.Dilution.AuthorizedRatio)
	fmt.Printf("| ATM Filings (2yr) | %d |\n", len(r.Dilution.ATMFilings))
	fmt.Printf("| Shelf Registrations (2yr) | %d |\n", len(r.Dilution.ShelfRegistrations))
	fmt.Println()

	if len(r.Dilution.ATMFilings) > 0 || len(r.Dilution.ShelfRegistrations) > 0 {
		fmt.Println("### Recent Filings")
		fmt.Println()
		fmt.Println("| Date | Form | Type |")
		fmt.Println("|------|------|------|")
		for _, f := range r.Dilution.ShelfRegistrations {
			fmt.Printf("| %s | %s | Shelf Registration |\n", f.Date, f.Form)
		}
		for _, f := range r.Dilution.ATMFilings {
			fmt.Printf("| %s | %s | ATM Offering |\n", f.Date, f.Form)
		}
		fmt.Println()
	}

	fmt.Printf("## Insider Activity\n\n")
	fmt.Printf("- **Form 4 filings**: %d in last %s\n\n", r.Insider.Form4Count, r.Insider.Period)

	fmt.Println("## Risk Flags")
	fmt.Println()
	if len(r.RiskFlags) > 0 {
		for _, f := range r.RiskFlags {
			icon := "⚠️"
			if f.Severity == "HIGH" {
				icon = "🔴"
			}
			fmt.Printf("- %s **%s** — %s\n", icon, f.Label, f.Description)
		}
	} else {
		fmt.Println("- ✅ No significant risk flags detected")
	}
	fmt.Println()

	fmt.Println("## DD Score")
	fmt.Println()
	fmt.Printf("**%d/100** (Grade: **%s**)\n\n", r.Score.Score, r.Score.Grade)
	fmt.Printf("> %s\n", r.Score.Summary)
}
