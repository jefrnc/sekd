package tui

import (
	"strings"
)

// SlashCommand is one entry in the interactive command palette. The first
// element of Names is the canonical form; the rest are aliases.
type SlashCommand struct {
	Names []string
	Desc  string
	Usage string
}

// Canonical returns the primary name, including the leading slash.
func (c SlashCommand) Canonical() string {
	if len(c.Names) == 0 {
		return ""
	}
	return c.Names[0]
}

// SlashCommands is the single source of truth for available slash commands.
// renderHelp and the palette suggestion logic both read from this list.
var SlashCommands = []SlashCommand{
	{Names: []string{"/help", "/h"}, Desc: "Show all commands"},
	{Names: []string{"/last"}, Desc: "Re-run last ticker"},
	{Names: []string{"/compare"}, Desc: "Side-by-side compare two tickers", Usage: "/compare T1 T2"},

	{Names: []string{"/filings", "/f"}, Desc: "List SEC filings", Usage: "/filings TICKER [form-type]"},
	{Names: []string{"/read", "/r"}, Desc: "Read filing at index N", Usage: "/read N"},
	{Names: []string{"/analyze", "/a"}, Desc: "Analyze filing with AI", Usage: "/analyze N"},

	{Names: []string{"/watchlist", "/wl", "/w"}, Desc: "Show watchlist"},
	{Names: []string{"/watchlist add"}, Desc: "Add ticker to watchlist", Usage: "/watchlist add TICKER [note]"},
	{Names: []string{"/watchlist remove"}, Desc: "Remove ticker from watchlist", Usage: "/watchlist remove TICKER"},
	{Names: []string{"/watchlist scan"}, Desc: "Re-scan all watched tickers and show deltas vs last snapshot"},

	{Names: []string{"/history"}, Desc: "Show command history"},
	{Names: []string{"/recent"}, Desc: "Show recent tickers"},
	{Names: []string{"/session"}, Desc: "Show current session info"},

	{Names: []string{"/config", "/c"}, Desc: "Show configuration"},
	{Names: []string{"/config set"}, Desc: "Set a config value", Usage: "/config set KEY VALUE"},
	{Names: []string{"/config clear"}, Desc: "Clear a config value", Usage: "/config clear KEY"},

	{Names: []string{"/export"}, Desc: "Export last output to a file", Usage: "/export [path]"},
	{Names: []string{"/copy"}, Desc: "Copy last output to clipboard"},

	{Names: []string{"/json"}, Desc: "Toggle JSON output mode"},
	{Names: []string{"/md", "/markdown"}, Desc: "Toggle Markdown output mode"},
	{Names: []string{"/clear", "/cls"}, Desc: "Clear the screen"},
	{Names: []string{"/quit", "/exit", "/q"}, Desc: "Exit"},
}

// MatchSlashCommands returns the commands whose canonical name or aliases
// start with the given prefix. Matching is case-insensitive. The returned
// slice is ordered by canonical name to keep the palette stable.
func MatchSlashCommands(prefix string) []SlashCommand {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" || !strings.HasPrefix(prefix, "/") {
		return nil
	}
	var out []SlashCommand
	seen := make(map[string]bool)
	for _, c := range SlashCommands {
		canon := c.Canonical()
		if seen[canon] {
			continue
		}
		for _, name := range c.Names {
			if strings.HasPrefix(strings.ToLower(name), prefix) {
				out = append(out, c)
				seen[canon] = true
				break
			}
		}
	}
	return out
}
