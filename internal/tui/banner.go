package tui

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/fatih/color"
)

var quotes = []string{
	"The trend is your friend until the end when it bends.",
	"In trading, what is comfortable is rarely profitable.",
	"Risk comes from not knowing what you're doing. — Buffett",
	"The market can stay irrational longer than you can stay solvent.",
	"Cut your losses short, let your winners run.",
	"There is no single market secret to discover. — Jack Schwager",
	"The goal of a successful trader is to make the best trades.",
	"Plan your trade and trade your plan.",
	"Float rotation is the short seller's nightmare.",
	"When in doubt, get out.",
	"The stock market is a device for transferring money from the impatient to the patient.",
	"Dilution is the silent killer of shareholder value.",
	"Never short a low float without checking the shelf.",
	"An ATM offering at 9:30 AM is not your friend.",
	"SEC filings don't lie. Management might, but filings don't.",
	"The best short setup is the one nobody else sees yet.",
}

func PrintBanner(version string) {
	cyan := color.New(color.FgCyan, color.Bold)
	dim := color.New(color.FgWhite)
	yellow := color.New(color.FgYellow)

	fmt.Println()
	cyan.Println("  ╔═══════════════════════════════════════════╗")
	cyan.Println("  ║                                           ║")
	cyan.Println("  ║   ◆  sekd                         ║")
	cyan.Println("  ║      Due Diligence CLI for US Stocks      ║")
	cyan.Println("  ║                                           ║")
	cyan.Println("  ╚═══════════════════════════════════════════╝")
	fmt.Println()

	dim.Printf("  Version %s", version)
	fmt.Print("  •  ")
	dim.Print("SEC EDGAR + XBRL + Finviz")
	fmt.Print("  •  ")
	aiStatus := DetectAIStatus()
	if aiStatus != "" {
		color.New(color.FgGreen).Printf("AI: %s", aiStatus)
	} else {
		dim.Print("AI: not configured")
	}
	fmt.Println()
	fmt.Println()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	quote := quotes[r.Intn(len(quotes))]
	yellow.Printf("  \"%s\"\n", quote)
	fmt.Println()
}

func DetectAIStatus() string {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		model := os.Getenv("OPENAI_MODEL")
		if model == "" {
			model = "gpt-4o-mini"
		}
		return "OpenAI (" + model + ")"
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		model := os.Getenv("ANTHROPIC_MODEL")
		if model == "" {
			model = "claude-haiku-4-5"
		}
		return "Anthropic (" + model + ")"
	}
	return ""
}
