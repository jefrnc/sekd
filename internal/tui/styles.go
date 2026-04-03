package tui

import "github.com/charmbracelet/lipgloss"

var (
	ColorCyan    = lipgloss.Color("#00BCD4")
	ColorGreen   = lipgloss.Color("#4CAF50")
	ColorRed     = lipgloss.Color("#F44336")
	ColorYellow  = lipgloss.Color("#FFC107")
	ColorDim     = lipgloss.Color("#888888")
	ColorWhite   = lipgloss.Color("#FFFFFF")
	ColorOrange  = lipgloss.Color("#FF9800")

	StyleBanner = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	StylePrompt = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	StyleError = lipgloss.NewStyle().
			Foreground(ColorRed)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorGreen)

	StyleInfo = lipgloss.NewStyle().
			Foreground(ColorDim)

	StyleQuote = lipgloss.NewStyle().
			Foreground(ColorYellow).
			Italic(true)

	StyleSection = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	StyleFilingRed = lipgloss.NewStyle().
			Foreground(ColorRed).
			Bold(true)

	StyleFilingGreen = lipgloss.NewStyle().
			Foreground(ColorGreen)

	StyleFilingYellow = lipgloss.NewStyle().
			Foreground(ColorYellow)

	StyleConfirmBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorCyan).
			Padding(0, 1).
			MarginLeft(2)

	StyleKeyword = lipgloss.NewStyle().
			Foreground(ColorOrange).
			Bold(true)
)
