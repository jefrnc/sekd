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
	"github.com/jefrnc/sekd/internal/edgar"
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

func renderHelp() string {
	var b strings.Builder

	title := StyleSection.Render("  ─── Commands ───")
	b.WriteString("\n" + title + "\n\n")

	cmds := []struct{ cmd, desc string }{
		{"TICKER", "Run full due diligence report"},
		{"/filings TICKER", "List SEC filings"},
		{"/filings TICKER S-3", "Filter by form type"},
		{"/read N", "Read filing at index N"},
		{"/analyze N", "Analyze filing with AI"},
		{"", ""},
		{"/config", "Show current configuration"},
		{"/config set KEY VAL", "Set a config value"},
		{"/config clear KEY", "Remove a config value"},
		{"", ""},
		{"/json", "Toggle JSON output"},
		{"/md", "Toggle Markdown output"},
		{"/clear", "Clear screen"},
		{"/help", "Show this help"},
		{"/quit", "Exit"},
	}

	for _, c := range cmds {
		if c.cmd == "" {
			b.WriteString("\n")
			continue
		}
		cmd := lipgloss.NewStyle().Foreground(ColorCyan).Width(22).Render("  " + c.cmd)
		desc := StyleInfo.Render(c.desc)
		b.WriteString(cmd + desc + "\n")
	}

	b.WriteString("\n")
	b.WriteString(StyleInfo.Render("  Config keys: openai-key, openai-model, anthropic-key, anthropic-model"))
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
