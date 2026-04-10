package tui

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"


	"github.com/charmbracelet/lipgloss"
	"github.com/jefrnc/sekd/internal/analysis"
	"github.com/jefrnc/sekd/internal/config"
	"regexp"

	"github.com/jefrnc/sekd/internal/edgar"
	"github.com/jefrnc/sekd/internal/watchlist"
	"github.com/jefrnc/sekd/internal/history"
)

func renderBanner(version string) string {
	var b strings.Builder

	cyan := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render
	dim := lipgloss.NewStyle().Foreground(ColorDim).Render

	b.WriteString("\n")
	b.WriteString(cyan("   ███████╗███████╗██╗  ██╗██████╗ ") + "\n")
	b.WriteString(cyan("   ██╔════╝██╔════╝██║ ██╔╝██╔══██╗") + "\n")
	b.WriteString(cyan("   ███████╗█████╗  █████╔╝ ██║  ██║") + "  " + dim("SEC Decoded") + "\n")
	b.WriteString(cyan("   ╚════██║██╔══╝  ██╔═██╗ ██║  ██║") + "  " + dim("Due Diligence CLI for US Stocks") + "\n")
	b.WriteString(cyan("   ███████║███████╗██║  ██╗██████╔╝") + "\n")
	b.WriteString(cyan("   ╚══════╝╚══════╝╚═╝  ╚═╝╚═════╝ ") + "\n")
	b.WriteString("\n")

	info := StyleInfo.Render(fmt.Sprintf("  v%s  •  SEC EDGAR + XBRL + Finviz", version))
	ai := DetectAIStatus()
	if ai != "" {
		info += "  •  " + StyleSuccess.Render("AI: "+ai)
	} else {
		info += "  •  " + StyleInfo.Render("AI: not configured")
	}
	b.WriteString(info + "\n\n")

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	quote := quotes[r.Intn(len(quotes))]
	b.WriteString(StyleQuote.Render(fmt.Sprintf("  \"%s\"", quote)) + "\n\n")

	b.WriteString(StyleInfo.Render("  Type a ticker to start, /help for commands, ctrl+c to exit.") + "\n")

	return b.String()
}

// renderSlashPalette renders the filtered command list shown below the input
// when the user is typing a slash command. selected is the palette index of
// the currently highlighted row (-1 = nothing highlighted yet, Tab will pick
// the first match).
func renderSlashPalette(input string, selected int) string {
	matches := MatchSlashCommands(input)
	if len(matches) == 0 {
		return StyleInfo.Render("  no matching commands (type /help to see all)") + "\n"
	}

	// Cap to keep the palette small.
	const maxRows = 8
	shown := matches
	truncated := false
	if len(shown) > maxRows {
		shown = shown[:maxRows]
		truncated = true
	}

	var b strings.Builder
	nameWidth := 0
	for _, c := range shown {
		if n := len(c.Canonical()); n > nameWidth {
			nameWidth = n
		}
	}
	nameStyle := lipgloss.NewStyle().Foreground(ColorCyan).Width(nameWidth + 2)
	selectedStyle := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Width(nameWidth + 2)

	for i, c := range shown {
		marker := "   "
		name := nameStyle.Render(c.Canonical())
		if i == selected {
			marker = " ▸ "
			name = selectedStyle.Render(c.Canonical())
		}
		desc := StyleInfo.Render(c.Desc)
		if c.Usage != "" {
			desc = StyleInfo.Render(c.Usage + "  — " + c.Desc)
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n", marker, name, desc))
	}
	if truncated {
		b.WriteString(StyleInfo.Render(fmt.Sprintf("   … +%d more (keep typing to narrow)", len(matches)-maxRows)))
		b.WriteString("\n")
	}
	b.WriteString(StyleInfo.Render("   tab: cycle · enter: run · esc: cancel"))
	b.WriteString("\n")
	return b.String()
}

func renderHelp() string {
	var b strings.Builder

	title := StyleSection.Render("  ─── Commands ───")
	b.WriteString("\n" + title + "\n\n")

	tickerRow := lipgloss.NewStyle().Foreground(ColorCyan).Width(28).Render("  TICKER")
	b.WriteString(tickerRow + StyleInfo.Render("Run full due diligence report") + "\n")
	deepRow := lipgloss.NewStyle().Foreground(ColorCyan).Width(28).Render("  TICKER --deep")
	b.WriteString(deepRow + StyleInfo.Render("(CLI) Extract shelf/warrants/convertibles via LLM") + "\n\n")

	for _, c := range SlashCommands {
		label := "  " + c.Canonical()
		if c.Usage != "" {
			label = "  " + c.Usage
		}
		name := lipgloss.NewStyle().Foreground(ColorCyan).Width(28).Render(label)
		b.WriteString(name + StyleInfo.Render(c.Desc) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(StyleInfo.Render("  Config keys: openai-key, openai-model, anthropic-key, anthropic-model"))
	b.WriteString("\n")
	b.WriteString(StyleInfo.Render("  Tip: start typing / to see the command palette with live matches."))
	b.WriteString("\n\n")

	return b.String()
}

func renderConfigShow() string {
	var b strings.Builder

	cfg, _ := config.Load()

	b.WriteString("\n")
	b.WriteString(StyleSection.Render("  ─── Configuration ───"))
	b.WriteString("\n\n")

	label := lipgloss.NewStyle().Foreground(ColorDim).Width(20)

	rows := []struct{ k, v string }{
		{"OpenAI Key", maskOrNone(cfg.OpenAIKey)},
		{"OpenAI Model", valueOrDefault(cfg.OpenAIModel, "gpt-4o-mini")},
		{"Anthropic Key", maskOrNone(cfg.AnthropicKey)},
		{"Anthropic Model", valueOrDefault(cfg.AnthropicModel, "claude-haiku-4-5")},
	}

	for _, r := range rows {
		b.WriteString(fmt.Sprintf("  %s %s\n", label.Render(r.k), r.v))
	}

	// Check env vars too
	b.WriteString("\n")
	if os.Getenv("OPENAI_API_KEY") != "" && cfg.OpenAIKey == "" {
		b.WriteString(StyleInfo.Render("  (OpenAI key loaded from .env or environment)"))
		b.WriteString("\n")
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" && cfg.AnthropicKey == "" {
		b.WriteString(StyleInfo.Render("  (Anthropic key loaded from .env or environment)"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(StyleInfo.Render("  Stored in: ~/.sekd/config.json"))
	b.WriteString("\n")

	return b.String()
}

func maskOrNone(key string) string {
	if key == "" {
		return StyleInfo.Render("not set")
	}
	return StyleSuccess.Render(config.MaskKey(key))
}

func valueOrDefault(val, def string) string {
	if val == "" {
		return StyleInfo.Render(def + " (default)")
	}
	return val
}

func renderFilingsListBubble(ticker string, filings []edgar.Filing) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(StyleSection.Render(fmt.Sprintf("  ─── %s — SEC Filings ───", ticker)))
	b.WriteString("\n\n")

	for i, f := range filings {
		date := StyleInfo.Render(f.FilingDate.Format("2006-01-02"))
		var form string
		switch {
		case f.Form == "S-3" || strings.HasPrefix(f.Form, "S-3") || strings.HasPrefix(f.Form, "424B"):
			form = StyleFilingRed.Render(fmt.Sprintf("%-7s", f.Form))
		case f.Form == "10-K" || f.Form == "10-Q":
			form = StyleFilingGreen.Render(fmt.Sprintf("%-7s", f.Form))
		case f.Form == "8-K":
			form = StyleFilingYellow.Render(fmt.Sprintf("%-7s", f.Form))
		default:
			form = fmt.Sprintf("%-7s", f.Form)
		}
		doc := StyleInfo.Render(f.PrimaryDocument)
		b.WriteString(fmt.Sprintf("  [%2d] %s  %s  %s\n", i, date, form, doc))
	}

	b.WriteString("\n")
	b.WriteString(StyleInfo.Render("  /read N to view  •  /analyze N for AI analysis"))
	b.WriteString("\n")

	return b.String()
}

func renderAnalysisBubble(doc *edgar.FilingDocument, result *analysis.AIAnalysis) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(StyleSection.Render(fmt.Sprintf("  ─── AI Analysis: %s — %s ───", doc.Form, doc.FilingDate)))
	b.WriteString("\n\n")

	b.WriteString(StyleInfo.Render(fmt.Sprintf("  Provider: %s (%s)", result.Provider, result.Model)))
	b.WriteString("\n")
	b.WriteString(StyleInfo.Render(fmt.Sprintf("  URL: %s", doc.URL)))
	b.WriteString("\n\n")

	b.WriteString(lipgloss.NewStyle().Bold(true).Render("  Summary:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s\n", result.Summary))

	if result.OfferingAmt != "" {
		b.WriteString(fmt.Sprintf("\n  %s %s", StyleKeyword.Render("Offering:"), result.OfferingAmt))
	}
	if result.Warrants != "" {
		b.WriteString(fmt.Sprintf("\n  %s %s", StyleKeyword.Render("Warrants:"), result.Warrants))
	}
	if result.DilutionImpact != "" {
		b.WriteString(fmt.Sprintf("\n  %s %s", StyleKeyword.Render("Dilution:"), result.DilutionImpact))
	}

	if len(result.RedFlags) > 0 {
		b.WriteString("\n\n")
		b.WriteString(StyleFilingRed.Render("  Red Flags:"))
		b.WriteString("\n")
		for _, flag := range result.RedFlags {
			b.WriteString(StyleFilingRed.Render(fmt.Sprintf("    [!] %s", flag)))
			b.WriteString("\n")
		}
	}

	if len(result.KeyTerms) > 0 {
		b.WriteString("\n")
		b.WriteString(StyleSuccess.Render("  Key Terms:"))
		b.WriteString("\n")
		for _, term := range result.KeyTerms {
			b.WriteString(fmt.Sprintf("    - %s\n", term))
		}
	}

	b.WriteString("\n")
	return b.String()
}

func renderConfirmDialog(msg string, sel int) string {
	var b strings.Builder

	yesStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Background(lipgloss.Color("#333333")).
		Foreground(ColorWhite)
	noStyle := yesStyle.Copy()

	yesActiveStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Background(ColorCyan).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)
	noActiveStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Background(lipgloss.Color("#F44336")).
		Foreground(ColorWhite).
		Bold(true)

	var yesBtn, noBtn string
	if sel == 0 {
		yesBtn = yesActiveStyle.Render("▸ Yes")
		noBtn = noStyle.Render("  No")
	} else {
		yesBtn = yesStyle.Render("  Yes")
		noBtn = noActiveStyle.Render("▸ No")
	}

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorCyan).
		Padding(1, 2).
		MarginLeft(2)

	content := msg + "\n\n" + yesBtn + "  " + noBtn + "\n\n" +
		StyleInfo.Render("←→ select  •  enter confirm  •  y/n shortcut")

	b.WriteString(box.Render(content))
	return b.String()
}

func renderFilingsNav(ticker string, filings []edgar.Filing, cursor int) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(StyleSection.Render(fmt.Sprintf("  ─── %s — SEC Filings ───", ticker)))
	b.WriteString("\n\n")

	for i, f := range filings {
		date := f.FilingDate.Format("2006-01-02")

		var formStyle lipgloss.Style
		switch {
		case f.Form == "S-3" || strings.HasPrefix(f.Form, "S-3") || strings.HasPrefix(f.Form, "424B"):
			formStyle = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)
		case f.Form == "10-K" || f.Form == "10-Q":
			formStyle = lipgloss.NewStyle().Foreground(ColorGreen)
		case f.Form == "8-K":
			formStyle = lipgloss.NewStyle().Foreground(ColorYellow)
		default:
			formStyle = lipgloss.NewStyle().Foreground(ColorWhite)
		}

		prefix := "    "
		if i == cursor {
			prefix = StylePrompt.Render("  ▸ ")
			// Highlight the whole line for selected item
			line := fmt.Sprintf("%s  %s  %s", date, formStyle.Render(fmt.Sprintf("%-7s", f.Form)), f.PrimaryDocument)
			b.WriteString(prefix + lipgloss.NewStyle().Bold(true).Render(line) + "\n")
		} else {
			form := formStyle.Render(fmt.Sprintf("%-7s", f.Form))
			b.WriteString(fmt.Sprintf("%s%s  %s  %s\n", prefix, StyleInfo.Render(date), form, StyleInfo.Render(f.PrimaryDocument)))
		}
	}

	b.WriteString("\n")
	b.WriteString(StyleInfo.Render("  ↑↓ navigate  •  enter read  •  a analyze with AI  •  esc back"))
	b.WriteString("\n")

	return b.String()
}

func renderHistory(entries []history.Entry, limit int) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(StyleSection.Render("  ─── History ───"))
	b.WriteString("\n\n")

	if len(entries) == 0 {
		b.WriteString(StyleInfo.Render("  No history yet"))
		b.WriteString("\n")
		return b.String()
	}

	if len(entries) > limit {
		entries = entries[:limit]
	}

	for _, e := range entries {
		ts := time.UnixMilli(e.Timestamp).Format("01/02 15:04")
		typeStyle := StyleInfo
		if e.Type == "ticker" {
			typeStyle = lipgloss.NewStyle().Foreground(ColorCyan)
		}
		b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
			StyleInfo.Render(ts),
			typeStyle.Render(fmt.Sprintf("%-8s", e.Type)),
			e.Input,
		))
	}

	b.WriteString("\n")
	b.WriteString(StyleInfo.Render("  Use ↑↓ arrows to navigate history in the prompt"))
	b.WriteString("\n")
	return b.String()
}

func renderRecentTickers(tickers []string) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(StyleSection.Render("  ─── Recent Tickers ───"))
	b.WriteString("\n\n")

	if len(tickers) == 0 {
		b.WriteString(StyleInfo.Render("  No tickers yet"))
		b.WriteString("\n")
		return b.String()
	}

	for i, t := range tickers {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			StyleInfo.Render(fmt.Sprintf("[%d]", i)),
			lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render(t),
		))
	}

	b.WriteString("\n")
	return b.String()
}

func renderWatchlist(wl *watchlist.Watchlist) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(StyleSection.Render("  ─── Watchlist ───"))
	b.WriteString("\n\n")

	if len(wl.Entries) == 0 {
		b.WriteString(StyleInfo.Render("  Empty. Use /watchlist add TICKER to add."))
		b.WriteString("\n")
		return b.String()
	}

	for _, e := range wl.Entries {
		ts := time.UnixMilli(e.AddedAt).Format("01/02")
		ticker := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Width(8).Render(e.Ticker)
		note := ""
		if e.Note != "" {
			note = StyleInfo.Render(" — " + e.Note)
		}
		b.WriteString(fmt.Sprintf("  %s  %s%s\n", StyleInfo.Render(ts), ticker, note))
	}

	b.WriteString("\n")
	b.WriteString(StyleInfo.Render("  Type a ticker to run DD. /watchlist remove TICKER to delete."))
	b.WriteString("\n")
	return b.String()
}

// renderScanResults renders the output of /watchlist scan: a per-ticker
// summary of what changed since the last snapshot, with unchanged tickers
// collapsed into a single line so the interesting ones stand out.
func renderScanResults(results []scanResult) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(StyleSection.Render(fmt.Sprintf("  ─── Watchlist Scan (%d tickers) ───", len(results))))
	b.WriteString("\n\n")

	tickerStyle := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Width(8)
	unchanged := 0
	hadDelta := false

	for _, r := range results {
		if r.err != nil {
			b.WriteString(fmt.Sprintf("  %s %s %s\n",
				StyleError.Render("✗"),
				tickerStyle.Render(r.ticker),
				StyleError.Render(r.err.Error()),
			))
			hadDelta = true
			continue
		}

		// First-time scan — no previous snapshot to diff against.
		if r.prev == nil || !r.prev.HasSnapshot() {
			b.WriteString(fmt.Sprintf("  %s %s %s (%s %d) — first scan, snapshot saved\n",
				StyleInfo.Render("•"),
				tickerStyle.Render(r.ticker),
				StyleInfo.Render("baseline:"),
				StyleInfo.Render(r.cur.Score.Grade),
				r.cur.Score.Score,
			))
			hadDelta = true
			continue
		}

		// No deltas at all — collapse to a counter.
		if !r.hasNew && r.scoreDelta == 0 && len(r.newFlags) == 0 && len(r.removedFlags) == 0 {
			unchanged++
			continue
		}

		hadDelta = true
		// Severity marker: new filing or negative score delta = warning.
		marker := StyleInfo.Render("•")
		if r.hasNew || r.scoreDelta < 0 || len(r.newFlags) > 0 {
			marker = StyleFilingYellow.Render("⚠")
		}
		if r.scoreDelta <= -15 {
			marker = StyleFilingRed.Render("🔴")
		}

		scoreBit := fmt.Sprintf("%s %d", r.cur.Score.Grade, r.cur.Score.Score)
		if r.scoreDelta != 0 {
			sign := "+"
			if r.scoreDelta < 0 {
				sign = ""
			}
			scoreBit = fmt.Sprintf("%s %d (%s%d vs last)", r.cur.Score.Grade, r.cur.Score.Score, sign, r.scoreDelta)
		}

		b.WriteString(fmt.Sprintf("  %s %s %s\n", marker, tickerStyle.Render(r.ticker), StyleInfo.Render(scoreBit)))

		if r.hasNew {
			form := r.cur.LatestFilingForm
			date := r.cur.LatestFilingDate
			b.WriteString(fmt.Sprintf("      %s new filing: %s on %s\n", StyleFilingYellow.Render("→"), form, date))
		}
		if len(r.newFlags) > 0 {
			b.WriteString(fmt.Sprintf("      %s new flags: %s\n", StyleFilingRed.Render("+"), strings.Join(r.newFlags, ", ")))
		}
		if len(r.removedFlags) > 0 {
			b.WriteString(fmt.Sprintf("      %s cleared flags: %s\n", StyleFilingGreen.Render("−"), strings.Join(r.removedFlags, ", ")))
		}
	}

	if unchanged > 0 {
		b.WriteString("\n")
		b.WriteString(StyleSuccess.Render(fmt.Sprintf("  ✓ %d unchanged", unchanged)))
		b.WriteString("\n")
	}
	if !hadDelta && unchanged == 0 {
		b.WriteString(StyleInfo.Render("  No tickers in watchlist."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(StyleInfo.Render("  Snapshots updated. Type a flagged ticker to see full DD, or /filings TICKER."))
	b.WriteString("\n")
	return b.String()
}

func renderCompare(r1, r2 *analysis.Report) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(StyleSection.Render(fmt.Sprintf("  ─── Compare: %s vs %s ───", r1.Ticker, r2.Ticker)))
	b.WriteString("\n\n")

	label := lipgloss.NewStyle().Foreground(ColorDim).Width(22)
	col1 := lipgloss.NewStyle().Foreground(ColorCyan).Width(16)
	col2 := lipgloss.NewStyle().Foreground(ColorOrange).Width(16)

	// Header
	b.WriteString(fmt.Sprintf("  %s %s %s\n",
		label.Render(""),
		col1.Bold(true).Render(r1.Ticker),
		col2.Bold(true).Render(r2.Ticker),
	))
	b.WriteString(fmt.Sprintf("  %s %s %s\n", label.Render(""), strings.Repeat("─", 14), strings.Repeat("─", 14)))

	rows := []struct {
		k, v1, v2 string
	}{
		{"Sector", r1.Sector, r2.Sector},
		{"Market Cap", r1.MarketCap, r2.MarketCap},
		{"Price", r1.Price, r2.Price},
		{"Float", r1.Float, r2.Float},
		{"Short Float", r1.ShortFloat, r2.ShortFloat},
		{"Insider Own", r1.InsiderOwn, r2.InsiderOwn},
		{"Inst Own", r1.InstOwn, r2.InstOwn},
		{"Dilution (12M)", fmt.Sprintf("%.1f%%", r1.Dilution.DilutionRate12M), fmt.Sprintf("%.1f%%", r2.Dilution.DilutionRate12M)},
		{"Auth/Out Ratio", fmt.Sprintf("%.1fx", r1.Dilution.AuthorizedRatio), fmt.Sprintf("%.1fx", r2.Dilution.AuthorizedRatio)},
		{"ATM Filings", fmt.Sprintf("%d", len(r1.Dilution.ATMFilings)), fmt.Sprintf("%d", len(r2.Dilution.ATMFilings))},
		{"Shelf Regs", fmt.Sprintf("%d", len(r1.Dilution.ShelfRegistrations)), fmt.Sprintf("%d", len(r2.Dilution.ShelfRegistrations))},
		{"Risk Flags", fmt.Sprintf("%d", len(r1.RiskFlags)), fmt.Sprintf("%d", len(r2.RiskFlags))},
		{"DD Score", fmt.Sprintf("%d (%s)", r1.Score.Score, r1.Score.Grade), fmt.Sprintf("%d (%s)", r2.Score.Score, r2.Score.Grade)},
	}

	for _, r := range rows {
		v1 := col1.Render(r.v1)
		v2 := col2.Render(r.v2)
		b.WriteString(fmt.Sprintf("  %s %s %s\n", label.Render(r.k), v1, v2))
	}

	// Risk flags detail
	b.WriteString("\n")
	if len(r1.RiskFlags) > 0 || len(r2.RiskFlags) > 0 {
		b.WriteString(fmt.Sprintf("  %s\n", StyleSection.Render("Risk Flags:")))
		allFlags := make(map[string]bool)
		r1Flags := make(map[string]string)
		r2Flags := make(map[string]string)
		for _, f := range r1.RiskFlags {
			allFlags[f.Label] = true
			r1Flags[f.Label] = f.Severity
		}
		for _, f := range r2.RiskFlags {
			allFlags[f.Label] = true
			r2Flags[f.Label] = f.Severity
		}
		for flag := range allFlags {
			s1 := StyleInfo.Render("  —")
			s2 := StyleInfo.Render("  —")
			if sev, ok := r1Flags[flag]; ok {
				if sev == "HIGH" {
					s1 = StyleFilingRed.Render("  [!]")
				} else {
					s1 = StyleFilingYellow.Render("  [~]")
				}
			}
			if sev, ok := r2Flags[flag]; ok {
				if sev == "HIGH" {
					s2 = StyleFilingRed.Render("  [!]")
				} else {
					s2 = StyleFilingYellow.Render("  [~]")
				}
			}
			b.WriteString(fmt.Sprintf("  %s %s %s\n", label.Render(flag), col1.Render(s1), col2.Render(s2)))
		}
	}

	b.WriteString("\n")
	return b.String()
}

func renderSessionInfo(m *Model) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(StyleSection.Render("  ─── Session ───"))
	b.WriteString("\n\n")

	label := lipgloss.NewStyle().Foreground(ColorDim).Width(18)

	ticker := "—"
	if m.lastTicker != "" {
		ticker = m.lastTicker
	}
	b.WriteString(fmt.Sprintf("  %s %s\n", label.Render("Last Ticker"), ticker))

	score := "—"
	if m.lastScore != nil {
		score = fmt.Sprintf("%d (%s)", m.lastScore.Score, m.lastScore.Grade)
	}
	b.WriteString(fmt.Sprintf("  %s %s\n", label.Render("Last Score"), score))

	b.WriteString(fmt.Sprintf("  %s %s\n", label.Render("Output Mode"), m.outputMode))

	filings := "—"
	if m.lastFilings != nil {
		filings = fmt.Sprintf("%d loaded", len(m.lastFilings))
	}
	b.WriteString(fmt.Sprintf("  %s %s\n", label.Render("Filings"), filings))

	wl, _ := watchlist.Load()
	b.WriteString(fmt.Sprintf("  %s %d tickers\n", label.Render("Watchlist"), len(wl.Entries)))

	histEntries := m.history.GetAll()
	b.WriteString(fmt.Sprintf("  %s %d entries\n", label.Render("History"), len(histEntries)))

	b.WriteString("\n")
	b.WriteString(StyleInfo.Render("  /session clear to reset"))
	b.WriteString("\n")

	return b.String()
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// captureOutput captures stdout from a function call
func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}
