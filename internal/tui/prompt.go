package tui

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

func PrintPrompt() {
	color.New(color.FgCyan, color.Bold).Print("  ◆ ")
}

func PrintHelp() {
	bold := color.New(color.Bold)
	dim := color.New(color.FgWhite)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	bold.Println("  Commands:")
	fmt.Println()

	cyan.Print("    TICKER")
	dim.Println("              Run full due diligence report (e.g. SOUN, MARA)")

	cyan.Print("    /filings TICKER")
	dim.Println("     List SEC filings for a ticker")

	cyan.Print("    /read TICKER N")
	dim.Println("      Read filing at index N")

	cyan.Print("    /analyze TICKER N")
	dim.Println("   Analyze filing at index N with AI")

	cyan.Print("    /json")
	dim.Println("               Toggle JSON output mode")

	cyan.Print("    /md")
	dim.Println("                 Toggle Markdown output mode")

	cyan.Print("    /clear")
	dim.Println("              Clear screen")

	cyan.Print("    /help")
	dim.Println("               Show this help")

	cyan.Print("    /quit")
	dim.Println("               Exit")

	fmt.Println()
}

func AskConfirm(question string) bool {
	cyan := color.New(color.FgCyan, color.Bold)
	dim := color.New(color.FgWhite)

	fmt.Println()
	cyan.Print("  ┌ ")
	fmt.Println(question)
	cyan.Print("  └ ")
	dim.Print("(y/n) ")

	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes" || input == "s" || input == "si"
}

func AskChoice(question string, options []string) int {
	cyan := color.New(color.FgCyan, color.Bold)
	dim := color.New(color.FgWhite)

	fmt.Println()
	cyan.Print("  ┌ ")
	fmt.Println(question)

	for i, opt := range options {
		cyan.Printf("  │ ")
		fmt.Printf("[%d] %s\n", i, opt)
	}

	cyan.Print("  └ ")
	dim.Print("choice: ")

	var input int
	_, err := fmt.Scanln(&input)
	if err != nil || input < 0 || input >= len(options) {
		return -1
	}
	return input
}

func PrintSection(title string) {
	fmt.Println()
	color.New(color.FgCyan, color.Bold).Printf("  ─── %s ───\n", title)
	fmt.Println()
}

func PrintError(msg string) {
	color.New(color.FgRed).Printf("  ✗ %s\n", msg)
}

func PrintSuccess(msg string) {
	color.New(color.FgGreen).Printf("  ✓ %s\n", msg)
}

func PrintInfo(msg string) {
	color.New(color.FgWhite).Printf("  → %s\n", msg)
}
