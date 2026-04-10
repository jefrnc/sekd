package cmd

import (
	"fmt"
	"os"

	"github.com/jefrnc/sekd/internal/config"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func init() {
	godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		// Config is corrupt — warn loudly so the user knows their API keys
		// aren't being read. Continue with an empty config so the rest of
		// the tool still works for features that don't need a key.
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		fmt.Fprintln(os.Stderr, "         fix or delete the file, then re-run. sekd will continue with defaults for now.")
	}
	cfg.Apply()
}

var rootCmd = &cobra.Command{
	Use:   "sekd",
	Short: "Due diligence CLI for US small-cap stocks",
	Long:  "Automated due diligence reports using public SEC EDGAR filings, XBRL data, and market information.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return interactiveCmd.RunE(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
