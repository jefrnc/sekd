package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jefrnc/sekd/internal/tui"
	"github.com/spf13/cobra"
)

var interactiveCmd = &cobra.Command{
	Use:     "interactive",
	Aliases: []string{"i", "shell"},
	Short:   "Start interactive mode",
	Long:    "Launch an interactive session with ticker lookup, filing browsing, and AI analysis.",
	RunE: func(cmd *cobra.Command, args []string) error {
		model, err := tui.NewModel(Version)
		if err != nil {
			return fmt.Errorf("initializing TUI: %w", err)
		}

		p := tea.NewProgram(model)
		if _, err := p.Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}
