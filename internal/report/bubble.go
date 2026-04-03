package report

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jefrnc/sekd/internal/analysis"
)

var (
	cyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00BCD4")).Bold(true)
	red     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F44336")).Bold(true)
	yellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFC107"))
	green   = lipgloss.NewStyle().Foreground(lipgloss.Color("#4CAF50")).Bold(true)
	dim     = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	bold    = lipgloss.NewStyle().Bold(true)
	white   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	label   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Width(24)
	val     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	section = lipgloss.NewStyle().Foreground(lipgloss.Color("#00BCD4")).Bold(true)
)

// RenderString returns the report as a styled string (no stdout, bubbletea-safe)
func RenderString(r *analysis.Report) string {
	var b strings.Builder

	// Header
	b.WriteString("\n")
	b.WriteString(cyan.Render(fmt.Sprintf("  ═══ %s ═══", strings.ToUpper(r.Ticker))))
	b.WriteString("\n")
	b.WriteString(dim.Render(fmt.Sprintf("  %s  •  CIK: %s", r.CompanyName, r.CIK)))
	b.WriteString("\n\n")

	// Overview
	b.WriteString(section.Render("  ─── Overview ───"))
	b.WriteString("\n\n")

	rows := []struct{ k, v string }{
		{"Sector", r.Sector},
		{"Industry", r.Industry},
		{"Country", r.Country},
		{"Market Cap", r.MarketCap},
		{"Price", r.Price},
		{"Float", r.Float},
		{"Short Float", r.ShortFloat},
		{"Insider Own", r.InsiderOwn},
		{"Inst Own", r.InstOwn},
		{"Volume", r.Volume},
		{"Avg Volume", r.AvgVolume},
	}
	for _, row := range rows {
		if row.v == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", label.Render(row.k), val.Render(row.v)))
	}

	// Dilution
	b.WriteString("\n")
	b.WriteString(section.Render("  ─── Dilution Analysis ───"))
	b.WriteString("\n\n")

	dilRows := []struct{ k, v string }{
		{"Dilution (6M)", fmt.Sprintf("%.1f%%", r.Dilution.DilutionRate6M)},
		{"Dilution (12M)", fmt.Sprintf("%.1f%%", r.Dilution.DilutionRate12M)},
		{"Outstanding", formatShares(r.Dilution.OutstandingShares)},
		{"Authorized", formatShares(r.Dilution.AuthorizedShares)},
		{"Auth/Out Ratio", fmt.Sprintf("%.1fx", r.Dilution.AuthorizedRatio)},
		{"ATM Filings (2yr)", fmt.Sprintf("%d", len(r.Dilution.ATMFilings))},
		{"Shelf Regs (2yr)", fmt.Sprintf("%d", len(r.Dilution.ShelfRegistrations))},
	}

	for _, row := range dilRows {
		value := row.v
		// Color dilution rates
		if strings.Contains(row.k, "Dilution") {
			if strings.HasPrefix(value, "-") {
				value = green.Render(value)
			} else if value != "0.0%" {
				value = red.Render(value)
			}
		}
		// Color ratio
		if row.k == "Auth/Out Ratio" && r.Dilution.AuthorizedRatio > 3 {
			value = red.Render(value)
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", label.Render(row.k), value))
	}

	// Recent filings
	if len(r.Dilution.ATMFilings) > 0 || len(r.Dilution.ShelfRegistrations) > 0 {
		b.WriteString("\n")
		b.WriteString(section.Render("  ─── Recent Filings ───"))
		b.WriteString("\n\n")
		for _, f := range r.Dilution.ShelfRegistrations {
			b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
				dim.Render(f.Date),
				red.Render(fmt.Sprintf("%-7s", f.Form)),
				dim.Render("Shelf Registration"),
			))
		}
		for _, f := range r.Dilution.ATMFilings {
			b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
				dim.Render(f.Date),
				red.Render(fmt.Sprintf("%-7s", f.Form)),
				dim.Render("ATM Offering"),
			))
		}
	}

	// Insider
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s\n",
		label.Render("Insider (Form 4)"),
		val.Render(fmt.Sprintf("%d filings in last %s", r.Insider.Form4Count, r.Insider.Period)),
	))

	// Risk flags
	b.WriteString("\n")
	b.WriteString(section.Render("  ─── Risk Flags ───"))
	b.WriteString("\n\n")

	if len(r.RiskFlags) == 0 {
		b.WriteString(green.Render("  ✓ No significant risk flags detected"))
		b.WriteString("\n")
	} else {
		for _, f := range r.RiskFlags {
			switch f.Severity {
			case "HIGH":
				b.WriteString(red.Render(fmt.Sprintf("  [!] %s — %s", f.Label, f.Description)))
			case "MEDIUM":
				b.WriteString(yellow.Render(fmt.Sprintf("  [~] %s — %s", f.Label, f.Description)))
			default:
				b.WriteString(dim.Render(fmt.Sprintf("  [-] %s — %s", f.Label, f.Description)))
			}
			b.WriteString("\n")
		}
	}

	// Score
	b.WriteString("\n")
	scoreStr := fmt.Sprintf("%d/100", r.Score.Score)
	var scoreStyled string
	switch {
	case r.Score.Score >= 75:
		scoreStyled = green.Render(scoreStr)
	case r.Score.Score >= 40:
		scoreStyled = yellow.Render(scoreStr)
	default:
		scoreStyled = red.Render(scoreStr)
	}

	b.WriteString(fmt.Sprintf("  %s %s  %s\n",
		bold.Render("DD SCORE:"),
		scoreStyled,
		dim.Render(fmt.Sprintf("(Grade: %s) — %s", r.Score.Grade, r.Score.Summary)),
	))
	b.WriteString("\n")

	return b.String()
}
