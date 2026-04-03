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
	if cfg, err := config.Load(); err == nil {
		cfg.Apply()
	}
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
