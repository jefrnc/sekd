package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jefrnc/sekd/internal/cache"
	"github.com/jefrnc/sekd/internal/report"
	"github.com/spf13/cobra"
)

var (
	jsonOutput     bool
	markdownOutput bool
	noCache        bool
)

var reportCmd = &cobra.Command{
	Use:   "report [ticker]",
	Short: "Generate a due diligence report for a US stock ticker",
	Long:  "Fetches SEC EDGAR filings, XBRL data, and market info to produce an automated DD report.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ticker := strings.ToUpper(args[0])

		c, err := cache.New()
		if err != nil {
			return fmt.Errorf("initializing cache: %w", err)
		}

		builder := report.NewBuilder(c)
		ctx := context.Background()

		fmt.Fprintf(os.Stderr, "Fetching data for %s...\n", ticker)

		r, err := builder.Build(ctx, ticker)
		if err != nil {
			return fmt.Errorf("building report for %s: %w", ticker, err)
		}

		switch {
		case jsonOutput:
			return report.RenderJSON(r)
		case markdownOutput:
			report.RenderMarkdown(r)
		default:
			report.RenderTerminal(r)
		}
		return nil
	},
}

func init() {
	reportCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output report as JSON")
	reportCmd.Flags().BoolVar(&markdownOutput, "md", false, "Output report as Markdown")
	reportCmd.Flags().BoolVar(&noCache, "no-cache", false, "Bypass cache")
	rootCmd.AddCommand(reportCmd)
}
