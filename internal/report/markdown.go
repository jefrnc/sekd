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

	if r.Deep != nil {
		renderDeepMarkdown(r.Deep)
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

func renderDeepMarkdown(d *analysis.DeepDilution) {
	fmt.Println("## Deep Dilution Detail")
	fmt.Println()
	fmt.Println("| Metric | Value |")
	fmt.Println("|--------|-------|")
	if d.ShelfTotalUSD > 0 {
		fmt.Printf("| Shelf Total | $%s |\n", formatUSD(d.ShelfTotalUSD))
	}
	if d.ShelfUsedUSD > 0 {
		fmt.Printf("| Shelf Used | $%s |\n", formatUSD(d.ShelfUsedUSD))
	}
	if d.ShelfRemainingUSD > 0 {
		fmt.Printf("| Shelf Remaining | $%s |\n", formatUSD(d.ShelfRemainingUSD))
	}
	if d.ATMCapacityRemainingUSD > 0 {
		fmt.Printf("| ATM Capacity Remaining | $%s |\n", formatUSD(d.ATMCapacityRemainingUSD))
	}
	if d.ITMWarrantShares > 0 {
		fmt.Printf("| Warrant Shares ITM | %s |\n", formatShares(d.ITMWarrantShares))
	}
	fmt.Printf("| Sources | %d filings |\n", len(d.Sources))
	fmt.Println()

	if len(d.Warrants) > 0 {
		fmt.Println("### Warrants")
		fmt.Println()
		fmt.Println("| Strike | Shares | Expiration | ITM | Description |")
		fmt.Println("|--------|--------|------------|-----|-------------|")
		for _, w := range d.Warrants {
			itm := ""
			if w.InTheMoney {
				itm = "yes"
			}
			fmt.Printf("| %s | %s | %s | %s | %s |\n", formatUSDOrDash(w.Strike, 2), formatSharesOrDash(w.Shares), dashIfEmpty(w.Expiration), itm, w.Description)
		}
		fmt.Println()
	}

	if len(d.Convertibles) > 0 {
		fmt.Println("### Convertibles")
		fmt.Println()
		fmt.Println("| Conv. Price | Principal | Maturity | ITM | Description |")
		fmt.Println("|-------------|-----------|----------|-----|-------------|")
		for _, cv := range d.Convertibles {
			itm := ""
			if cv.InTheMoney {
				itm = "yes"
			}
			fmt.Printf("| %s | %s | %s | %s | %s |\n", formatUSDOrDash(cv.ConversionPrice, 4), formatUSDPrincipalOrDash(cv.PrincipalUSD), dashIfEmpty(cv.Maturity), itm, cv.Description)
		}
		fmt.Println()
	}

	if len(d.Notes) > 0 {
		fmt.Println("### Notes")
		fmt.Println()
		for _, n := range d.Notes {
			fmt.Printf("- %s\n", n)
		}
		fmt.Println()
	}
}
