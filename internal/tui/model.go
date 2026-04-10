package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jefrnc/sekd/internal/analysis"
	"github.com/jefrnc/sekd/internal/cache"
	"github.com/jefrnc/sekd/internal/clipboard"
	"github.com/jefrnc/sekd/internal/config"
	"github.com/jefrnc/sekd/internal/edgar"
	"github.com/jefrnc/sekd/internal/history"
	"github.com/jefrnc/sekd/internal/notify"
	"github.com/jefrnc/sekd/internal/report"
	"github.com/jefrnc/sekd/internal/session"
	"github.com/jefrnc/sekd/internal/watchlist"
)

type state int

const (
	stateInput state = iota
	stateLoading
	stateResult
	stateConfirm
	stateFilings
	stateAnalyzing
)

// Messages
type reportDoneMsg struct {
	report   *analysis.Report
	output   string
	cik      string
	err      error
	elapsed  time.Duration
}

type progressMsg string

type filingsDoneMsg struct {
	filings []edgar.Filing
	output  string
	cik     string
	err     error
}

type analysisDoneMsg struct {
	output string
	err    error
}

type readDoneMsg struct {
	output string
	doc    *edgar.FilingDocument
	err    error
}

type Model struct {
	textInput   textinput.Model
	spinner     spinner.Model
	state       state
	version     string
	output      string
	statusMsg   string
	confirmMsg    string
	confirmYes    tea.Cmd
	confirmSel    int // 0 = Yes, 1 = No
	width       int
	height      int
	banner      string

	// Data
	cache       *cache.Cache
	edgarClient *edgar.Client
	lastTicker  string
	lastCIK     string
	lastFilings []edgar.Filing
	lastReport  *analysis.Report
	lastDoc     *edgar.FilingDocument
	outputMode  string // "terminal", "json", "md"
	startTime   time.Time
	filCursor   int    // cursor position in filings list
	filScroll   int    // scroll offset for filings list
	history     *history.History
	histItems   []string // cached history inputs for up/down nav
	histIdx     int      // current position in history (-1 = new input)
	histDraft   string   // what the user was typing before navigating history
	viewport    viewport.Model
	vpReady     bool
	lastScore   *analysis.DDScore
	suggestions []string // ticker suggestions for tab completion
	sugIdx      int      // current suggestion index (-1 = none)
	cmdSugIdx   int      // slash-command palette selection (-1 = none, so Tab picks first)
	tipIdx      int      // rotating tip index
}

func NewModel(version string) (Model, error) {
	c, err := cache.New()
	if err != nil {
		return Model{}, err
	}

	ti := textinput.New()
	ti.Placeholder = "Enter ticker or /command..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Prompt = "  ◆ "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorWhite)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorCyan)

	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
	hist, _ := history.New(sessionID)

	m := Model{
		textInput:   ti,
		spinner:     s,
		state:       stateInput,
		version:     version,
		banner:      renderBanner(version),
		cache:       c,
		history:     hist,
		histIdx:     -1,
		edgarClient: edgar.NewClient(c),
		outputMode:  "terminal",
	}

	// Restore previous session (last 24h)
	if prev := session.Load(24 * time.Hour); prev != nil {
		m.lastTicker = prev.LastTicker
		m.lastCIK = prev.LastCIK
		m.outputMode = prev.OutputMode
		if prev.LastScore != nil {
			m.lastScore = &analysis.DDScore{
				Score: prev.LastScore.Value,
				Grade: prev.LastScore.Grade,
			}
		}
	}

	return m, nil
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

		// Filing list navigation
		if m.state == stateFilings {
			switch msg.String() {
			case "up", "k":
				if m.filCursor > 0 {
					m.filCursor--
				}
				return m, nil
			case "down", "j":
				if m.filCursor < len(m.lastFilings)-1 {
					m.filCursor++
				}
				return m, nil
			case "enter":
				// Open selected filing
				m.state = stateLoading
				f := m.lastFilings[m.filCursor]
				m.statusMsg = fmt.Sprintf("Downloading %s filed %s...", f.Form, f.FilingDate.Format("2006-01-02"))
				return m, m.readFilingCmd(m.lastTicker, m.filCursor)
			case "a":
				// Analyze selected filing
				if DetectAIStatus() != "" {
					m.state = stateLoading
					f := m.lastFilings[m.filCursor]
					m.statusMsg = fmt.Sprintf("Analyzing %s with AI...", f.Form)
					return m, m.analyzeFilingCmd(m.lastTicker, m.filCursor)
				}
				return m, nil
			case "esc", "q":
				m.lastFilings = nil
				m.output = ""
				m.state = stateInput
				return m, nil
			}
			return m, nil
		}

		// Confirm dialog
		if m.state == stateConfirm {
			switch msg.String() {
			case "y", "Y", "s", "S":
				m.state = stateLoading
				return m, m.confirmYes
			case "n", "N", "esc":
				m.confirmMsg = ""
				m.goBack()
				return m, nil
			case "left", "right", "tab", "h", "l":
				m.confirmSel = 1 - m.confirmSel // toggle 0↔1
				return m, nil
			case "enter":
				if m.confirmSel == 0 {
					m.state = stateLoading
					return m, m.confirmYes
				}
				m.confirmMsg = ""
				m.goBack()
				return m, nil
			}
			return m, nil
		}

		// Input-capable states: stateInput and stateResult
		if m.state == stateInput || m.state == stateResult {
			switch msg.String() {
			case "enter":
				return m.handleEnter()
			case "up":
				return m.historyUp()
			case "down":
				return m.historyDown()
			case "tab":
				return m.handleTab()
			case "esc":
				if m.state == stateResult {
					m.output = ""
					m.state = stateInput
					return m, nil
				}
				// stateInput — confirm exit
				m.state = stateConfirm
				m.confirmSel = 0
				m.confirmMsg = "Exit sekd? (y/n)"
				m.confirmYes = tea.Quit
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 1 // leave room for status bar
		m.vpReady = true

	case reportDoneMsg:
		if msg.err != nil {
			m.output = StyleError.Render("  ✗ " + msg.err.Error())
			m.state = stateResult
		} else {
			elapsed := fmt.Sprintf("%.1fs", msg.elapsed.Seconds())
			output := msg.output + StyleInfo.Render(fmt.Sprintf("  Done in %s", elapsed)) + "\n"
			m.lastReport = msg.report
			m.lastScore = &msg.report.Score
			m.lastCIK = msg.cik
			m.setResult(output)
			m.saveSession()
			go notify.Notify("sekd", fmt.Sprintf("%s DD complete — Score: %d %s", m.lastTicker, msg.report.Score.Score, msg.report.Score.Grade))
			if m.lastReport != nil && (len(m.lastReport.Dilution.ATMFilings) > 0 || len(m.lastReport.Dilution.ShelfRegistrations) > 0) {
				total := len(m.lastReport.Dilution.ATMFilings) + len(m.lastReport.Dilution.ShelfRegistrations)
				m.state = stateConfirm
				m.confirmSel = 0
				m.confirmMsg = fmt.Sprintf("Found %d dilution-related filings. View them? (y/n)", total)
				m.confirmYes = m.loadFilingsCmd(m.lastTicker, "")
			}
		}
		return m, nil

	case filingsDoneMsg:
		if msg.cik != "" {
			m.lastCIK = msg.cik
		}
		if msg.err != nil {
			m.output = StyleError.Render("  ✗ " + msg.err.Error())
			m.state = stateResult
		} else {
			m.lastFilings = msg.filings
			m.filCursor = 0
			m.filScroll = 0
			m.state = stateFilings
		}
		return m, nil

	case analysisDoneMsg:
		if msg.err != nil {
			m.output = StyleError.Render("  ✗ " + msg.err.Error())
			if m.lastFilings != nil {
				m.state = stateFilings
			} else {
				m.state = stateResult
			}
		} else {
			m.output = msg.output
			m.state = stateResult
		}
		return m, nil

	case readDoneMsg:
		if msg.err != nil {
			m.output = StyleError.Render("  ✗ " + msg.err.Error())
			if m.lastFilings != nil {
				m.state = stateFilings
			} else {
				m.state = stateResult
			}
		} else {
			m.lastDoc = msg.doc
			m.output = msg.output
			if DetectAIStatus() != "" {
				m.state = stateConfirm
				m.confirmSel = 0
				m.confirmMsg = "Analyze this filing with AI? (y/n)"
				m.confirmYes = m.analyzeDocCmd(msg.doc)
			} else {
				m.state = stateResult
			}
		}
		return m, nil

	case compareDoneMsg:
		if msg.err != nil {
			m.output = StyleError.Render("  ✗ " + msg.err.Error())
			m.state = stateResult
		} else {
			elapsed := fmt.Sprintf("%.1fs", msg.elapsed.Seconds())
			m.setResult(msg.output + StyleInfo.Render(fmt.Sprintf("  Done in %s", elapsed)) + "\n")
		}
		return m, nil

	case watchlistScanDoneMsg:
		if msg.err != nil {
			m.output = StyleError.Render("  ✗ " + msg.err.Error())
			m.state = stateResult
		} else {
			elapsed := fmt.Sprintf("%.1fs", msg.elapsed.Seconds())
			m.setResult(msg.output + StyleInfo.Render(fmt.Sprintf("  Scan completed in %s", elapsed)) + "\n")
		}
		return m, nil

	case progressMsg:
		m.statusMsg = string(msg)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if m.state == stateInput || m.state == stateResult {
		prevValue := m.textInput.Value()
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		// Reset palette selection whenever the text actually changed so Tab
		// starts cycling from the first match again.
		if m.textInput.Value() != prevValue {
			m.cmdSugIdx = -1
		}
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	var content strings.Builder

	switch m.state {
	case stateInput:
		if m.output == "" {
			content.WriteString(m.banner)
		} else {
			content.WriteString(m.output)
			content.WriteString("\n")
		}
		content.WriteString("\n")
		content.WriteString(m.textInput.View())
		if m.sugIdx >= 0 && m.sugIdx < len(m.suggestions) {
			content.WriteString("  ")
			content.WriteString(StyleInfo.Render(m.suggestions[m.sugIdx]))
		}
		content.WriteString("\n")

		// Slash command palette: show filtered list when input starts with /
		currentInput := strings.TrimSpace(m.textInput.Value())
		if strings.HasPrefix(currentInput, "/") {
			content.WriteString(renderSlashPalette(currentInput, m.cmdSugIdx))
		} else if len(tips) > 0 {
			tip := tips[m.tipIdx%len(tips)]
			content.WriteString(StyleInfo.Render(fmt.Sprintf("  tip: %s", tip)))
			content.WriteString("\n")
		}

	case stateLoading:
		if m.output != "" {
			content.WriteString(m.output)
			content.WriteString("\n")
		}
		content.WriteString(fmt.Sprintf("\n  %s %s\n", m.spinner.View(), m.statusMsg))

	case stateConfirm:
		if m.output != "" {
			content.WriteString(m.output)
			content.WriteString("\n")
		}
		content.WriteString("\n")
		content.WriteString(renderConfirmDialog(m.confirmMsg, m.confirmSel))
		content.WriteString("\n")

	case stateFilings:
		if m.output != "" {
			content.WriteString(m.output)
			content.WriteString("\n")
		}
		content.WriteString(renderFilingsNav(m.lastTicker, m.lastFilings, m.filCursor))

	case stateResult:
		// Show output with prompt below so user can keep typing
		if m.output != "" {
			content.WriteString(m.output)
			content.WriteString("\n")
		}
		content.WriteString("\n")
		content.WriteString(m.textInput.View())
		content.WriteString("\n")
	}

	return m.layoutWithStatusBar(content.String())
}

func (m Model) layoutWithStatusBar(content string) string {
	statusBar := m.renderStatusBar()
	lines := strings.Count(content, "\n") + 1
	available := m.height - 1
	padding := ""
	if available > lines {
		padding = strings.Repeat("\n", available-lines)
	}
	return content + padding + statusBar
}

func (m *Model) saveSession() {
	s := &session.Session{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		LastTicker: m.lastTicker,
		LastCIK:    m.lastCIK,
		OutputMode: m.outputMode,
	}
	if m.lastScore != nil {
		s.LastScore = &session.Score{
			Value: m.lastScore.Score,
			Grade: m.lastScore.Grade,
		}
	}
	go session.Save(s)
}

func (m *Model) setResult(output string) {
	m.output = output
	m.state = stateResult
	if m.vpReady {
		hint := "\n" + StyleInfo.Render("  esc back  •  ↑↓ scroll  •  /help commands") + "\n"
		m.viewport.SetContent(output + hint)
		m.viewport.GotoTop()
	}
}

func (m Model) renderStatusBar() string {
	if m.width == 0 {
		return ""
	}

	barStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1a1a2e")).
		Foreground(lipgloss.Color("#888888")).
		Width(m.width)

	itemStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1a1a2e")).
		Foreground(lipgloss.Color("#888888"))

	activeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1a1a2e")).
		Foreground(lipgloss.Color("#00BCD4"))

	// Left side: ticker + mode
	var left []string

	if m.lastTicker != "" {
		left = append(left, activeStyle.Render(" "+m.lastTicker+" "))
	}

	mode := m.outputMode
	if mode == "terminal" {
		mode = "tty"
	}
	left = append(left, itemStyle.Render(" "+mode+" "))

	// AI status
	ai := DetectAIStatus()
	if ai != "" {
		left = append(left, itemStyle.Render(" AI:on "))
	}

	// Score
	if m.lastScore != nil {
		scoreStyle := itemStyle
		switch {
		case m.lastScore.Score >= 75:
			scoreStyle = lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2e")).Foreground(lipgloss.Color("#4CAF50"))
		case m.lastScore.Score >= 40:
			scoreStyle = lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2e")).Foreground(lipgloss.Color("#FFC107"))
		default:
			scoreStyle = lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2e")).Foreground(lipgloss.Color("#F44336"))
		}
		left = append(left, scoreStyle.Render(fmt.Sprintf(" %d %s ", m.lastScore.Score, m.lastScore.Grade)))
	}

	// Watchlist count
	if wl, err := watchlist.Load(); err == nil && len(wl.Entries) > 0 {
		left = append(left, itemStyle.Render(fmt.Sprintf(" wl:%d ", len(wl.Entries))))
	}

	// Scroll position
	if m.state == stateResult && m.vpReady {
		pct := m.viewport.ScrollPercent()
		left = append(left, itemStyle.Render(fmt.Sprintf(" %d%% ", int(pct*100))))
	}

	leftStr := strings.Join(left, itemStyle.Render("│"))

	// Right side: version + help hint
	right := itemStyle.Render(fmt.Sprintf(" v%s  /help ", m.version))

	// Fill middle with spaces
	gap := m.width - lipgloss.Width(leftStr) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	middle := itemStyle.Render(strings.Repeat(" ", gap))

	return barStyle.Render(leftStr + middle + right)
}

func (m *Model) handleEnter() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textInput.Value())
	m.textInput.SetValue("")

	// Reset history navigation and suggestions
	m.histIdx = -1
	m.histItems = nil
	m.histDraft = ""
	m.suggestions = nil
	m.sugIdx = -1
	m.cmdSugIdx = -1
	m.tipIdx++

	if input == "" {
		return m, nil
	}

	if strings.HasPrefix(input, "/") {
		m.history.Add(input, "command")
		go m.history.Flush()
		return m.handleCommand(input)
	}

	// Treat as ticker
	ticker := strings.ToUpper(input)
	m.lastTicker = ticker
	m.history.Add(ticker, "ticker")
	go m.history.Flush()
	m.state = stateLoading
	m.startTime = time.Now()
	m.statusMsg = "Resolving " + ticker + "..."
	return m, m.runReportCmd(ticker)
}

func (m *Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/quit", "/exit", "/q":
		return m, tea.Quit

	case "/help", "/h":
		m.output = renderHelp()
		m.state = stateInput
		return m, nil

	case "/clear", "/cls":
		m.output = ""
		m.state = stateInput
		return m, nil

	case "/json":
		if m.outputMode == "json" {
			m.outputMode = "terminal"
		} else {
			m.outputMode = "json"
		}
		m.output = StyleSuccess.Render(fmt.Sprintf("  ✓ Output mode: %s", m.outputMode))
		m.state = stateInput
		return m, nil

	case "/md", "/markdown":
		if m.outputMode == "md" {
			m.outputMode = "terminal"
		} else {
			m.outputMode = "md"
		}
		m.output = StyleSuccess.Render(fmt.Sprintf("  ✓ Output mode: %s", m.outputMode))
		m.state = stateInput
		return m, nil

	case "/filings", "/f":
		ticker := m.lastTicker
		formFilter := ""
		if len(parts) >= 2 {
			ticker = strings.ToUpper(parts[1])
		}
		if len(parts) >= 3 {
			formFilter = parts[2]
		}
		if ticker == "" {
			m.output = StyleError.Render("  ✗ Usage: /filings TICKER [form-type]")
			m.state = stateInput
			return m, nil
		}
		m.lastTicker = ticker
		m.state = stateLoading
		m.statusMsg = "Loading filings for " + ticker + "..."
		return m, m.loadFilingsCmd(ticker, formFilter)

	case "/read", "/r":
		return m.handleReadCmd(parts)

	case "/analyze", "/a":
		return m.handleAnalyzeCmd(parts)

	case "/config", "/c":
		return m.handleConfigCmd(parts)

	case "/history":
		m.output = renderHistory(m.history.GetAll(), 20)
		m.state = stateInput
		return m, nil

	case "/recent":
		tickers := m.history.GetTickers()
		if len(tickers) > 10 {
			tickers = tickers[:10]
		}
		m.output = renderRecentTickers(tickers)
		m.state = stateInput
		return m, nil

	case "/export":
		return m.handleExport(parts)

	case "/watchlist", "/wl", "/w":
		return m.handleWatchlist(parts)

	case "/last":
		if m.lastTicker == "" {
			m.output = StyleError.Render("  ✗ No previous ticker")
			m.state = stateInput
			return m, nil
		}
		m.history.Add(m.lastTicker, "ticker")
		go m.history.Flush()
		m.state = stateLoading
		m.startTime = time.Now()
		m.statusMsg = "Fetching data for " + m.lastTicker + "..."
		return m, m.runReportCmd(m.lastTicker)

	case "/compare":
		return m.handleCompare(parts)

	case "/copy":
		if m.output == "" {
			m.output = StyleError.Render("  ✗ Nothing to copy")
			m.state = stateInput
			return m, nil
		}
		if err := clipboard.Copy(m.output); err != nil {
			m.output = StyleError.Render("  ✗ Clipboard failed: " + err.Error())
		} else {
			m.output = StyleSuccess.Render("  ✓ Copied to clipboard")
		}
		m.state = stateInput
		return m, nil

	case "/session":
		if len(parts) >= 2 && parts[1] == "clear" {
			session.Clear()
			m.output = StyleSuccess.Render("  ✓ Session cleared")
			m.state = stateInput
			return m, nil
		}
		m.output = renderSessionInfo(m)
		m.state = stateInput
		return m, nil

	default:
		m.output = StyleError.Render("  ✗ Unknown command: " + cmd + ". Type /help")
		m.state = stateInput
		return m, nil
	}
}

func (m *Model) handleConfigCmd(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		// /config — show current config
		m.output = renderConfigShow()
		m.state = stateInput
		return m, nil
	}

	action := strings.ToLower(parts[1])

	switch action {
	case "show":
		m.output = renderConfigShow()
		m.state = stateInput
		return m, nil

	case "set":
		if len(parts) < 4 {
			m.output = StyleError.Render("  ✗ Usage: /config set <key> <value>\n  Keys: openai-key, openai-model, anthropic-key, anthropic-model")
			m.state = stateInput
			return m, nil
		}
		key := parts[2]
		value := strings.Join(parts[3:], " ")

		cfg, _ := config.Load()
		if !cfg.Set(key, value) {
			m.output = StyleError.Render("  ✗ Unknown key: " + key + "\n  Keys: openai-key, openai-model, anthropic-key, anthropic-model")
			m.state = stateInput
			return m, nil
		}
		if err := cfg.Save(); err != nil {
			m.output = StyleError.Render("  ✗ Failed to save: " + err.Error())
			m.state = stateInput
			return m, nil
		}
		cfg.Apply()

		m.output = StyleSuccess.Render(fmt.Sprintf("  ✓ %s updated", key))
		// Refresh banner to show new AI status
		m.banner = renderBanner(m.version)
		m.state = stateInput
		return m, nil

	case "clear", "delete", "remove":
		if len(parts) < 3 {
			m.output = StyleError.Render("  ✗ Usage: /config clear <key>")
			m.state = stateInput
			return m, nil
		}
		key := parts[2]

		cfg, _ := config.Load()
		if !cfg.Clear(key) {
			m.output = StyleError.Render("  ✗ Unknown key: " + key)
			m.state = stateInput
			return m, nil
		}
		if err := cfg.Save(); err != nil {
			m.output = StyleError.Render("  ✗ Failed to save: " + err.Error())
			m.state = stateInput
			return m, nil
		}
		// Clear from env too
		switch strings.ToLower(key) {
		case "openai-key", "openai_key", "openai":
			os.Unsetenv("OPENAI_API_KEY")
		case "anthropic-key", "anthropic_key", "anthropic":
			os.Unsetenv("ANTHROPIC_API_KEY")
		}

		m.output = StyleSuccess.Render(fmt.Sprintf("  ✓ %s cleared", key))
		m.banner = renderBanner(m.version)
		m.state = stateInput
		return m, nil

	default:
		m.output = StyleError.Render("  ✗ Usage: /config [show|set|clear]\n  /config set openai-key sk-...\n  /config clear openai-key")
		m.state = stateInput
		return m, nil
	}
}

func (m *Model) handleExport(parts []string) (tea.Model, tea.Cmd) {
	if m.output == "" {
		m.output = StyleError.Render("  ✗ Nothing to export. Run a report first.")
		m.state = stateInput
		return m, nil
	}

	filename := ""
	if len(parts) >= 2 {
		filename = parts[1]
	} else if m.lastTicker != "" {
		filename = strings.ToLower(m.lastTicker) + "-dd.md"
	} else {
		filename = "sekd-export.md"
	}

	// Strip ANSI codes for file export
	clean := stripAnsi(m.output)
	if err := os.WriteFile(filename, []byte(clean), 0644); err != nil {
		m.output = StyleError.Render("  ✗ Export failed: " + err.Error())
		m.state = stateInput
		return m, nil
	}

	cwd, _ := os.Getwd()
	m.output = StyleSuccess.Render(fmt.Sprintf("  ✓ Exported to %s/%s", cwd, filename))
	m.state = stateInput
	return m, nil
}

func (m *Model) handleWatchlist(parts []string) (tea.Model, tea.Cmd) {
	wl, err := watchlist.Load()
	if err != nil {
		// Surface corrupt-file errors so the user understands why their
		// list looks empty. Loading always returns a safe empty Watchlist
		// on error, so subsequent operations still work.
		m.output = StyleError.Render("  ✗ "+err.Error()) + "\n"
		m.state = stateInput
		return m, nil
	}

	if len(parts) < 2 {
		// Show watchlist
		m.output = renderWatchlist(wl)
		m.state = stateInput
		return m, nil
	}

	action := strings.ToLower(parts[1])

	switch action {
	case "add":
		if len(parts) < 3 {
			m.output = StyleError.Render("  ✗ Usage: /watchlist add TICKER [note]")
			m.state = stateInput
			return m, nil
		}
		ticker := strings.ToUpper(parts[2])
		note := ""
		if len(parts) > 3 {
			note = strings.Join(parts[3:], " ")
		}
		if wl.Add(ticker, note) {
			wl.Save()
			m.output = StyleSuccess.Render(fmt.Sprintf("  ✓ %s added to watchlist", ticker))
		} else {
			m.output = StyleInfo.Render(fmt.Sprintf("  %s already in watchlist", ticker))
		}
		m.state = stateInput
		return m, nil

	case "scan":
		if len(wl.Entries) == 0 {
			m.output = StyleError.Render("  ✗ Watchlist is empty. Add tickers first: /watchlist add TICKER")
			m.state = stateInput
			return m, nil
		}
		m.state = stateLoading
		m.startTime = time.Now()
		m.statusMsg = fmt.Sprintf("Scanning %d watched tickers...", len(wl.Entries))
		return m, m.watchlistScanCmd(wl)

	case "remove", "rm", "del":
		if len(parts) < 3 {
			m.output = StyleError.Render("  ✗ Usage: /watchlist remove TICKER")
			m.state = stateInput
			return m, nil
		}
		ticker := strings.ToUpper(parts[2])
		if wl.Remove(ticker) {
			wl.Save()
			m.output = StyleSuccess.Render(fmt.Sprintf("  ✓ %s removed from watchlist", ticker))
		} else {
			m.output = StyleError.Render(fmt.Sprintf("  ✗ %s not in watchlist", ticker))
		}
		m.state = stateInput
		return m, nil

	default:
		// Treat as ticker shortcut: /wl SOUN = add
		ticker := strings.ToUpper(action)
		if wl.Add(ticker, "") {
			wl.Save()
			m.output = StyleSuccess.Render(fmt.Sprintf("  ✓ %s added to watchlist", ticker))
		} else {
			m.output = StyleInfo.Render(fmt.Sprintf("  %s already in watchlist", ticker))
		}
		m.state = stateInput
		return m, nil
	}
}

type compareDoneMsg struct {
	output  string
	err     error
	elapsed time.Duration
}

type watchlistScanDoneMsg struct {
	output  string
	err     error
	elapsed time.Duration
}

// scanResult is the per-ticker outcome produced while scanning a watchlist.
type scanResult struct {
	ticker      string
	prev        *watchlist.Entry
	cur         *analysis.Report
	err         error
	hasNew      bool   // new filing since last scan
	scoreDelta  int    // cur.Score.Score - prev.LastScore
	newFlags    []string
	removedFlags []string
}

func (m *Model) handleCompare(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 3 {
		m.output = StyleError.Render("  ✗ Usage: /compare TICKER1 TICKER2")
		m.state = stateInput
		return m, nil
	}
	t1 := strings.ToUpper(parts[1])
	t2 := strings.ToUpper(parts[2])
	m.state = stateLoading
	m.startTime = time.Now()
	m.statusMsg = fmt.Sprintf("Comparing %s vs %s...", t1, t2)
	return m, m.compareCmd(t1, t2)
}

// watchlistScanCmd rebuilds a fresh report for every ticker in the watchlist,
// compares each one against the previously stored snapshot, updates the
// snapshot on disk, and returns a rendered delta report.
func (m *Model) watchlistScanCmd(wl *watchlist.Watchlist) tea.Cmd {
	start := m.startTime
	return func() tea.Msg {
		ctx := context.Background()
		builder := report.NewBuilder(m.cache)

		results := make([]scanResult, 0, len(wl.Entries))
		// Work on copies so we update the file once at the end.
		entries := make([]watchlist.Entry, len(wl.Entries))
		copy(entries, wl.Entries)

		for i := range entries {
			prev := entries[i]
			rep, err := builder.Build(ctx, prev.Ticker)
			if err != nil {
				results = append(results, scanResult{ticker: prev.Ticker, err: err})
				continue
			}
			r := scanResult{ticker: prev.Ticker, prev: &prev, cur: rep}
			if prev.HasSnapshot() {
				if rep.LatestAccession != "" && rep.LatestAccession != prev.LastAccession {
					r.hasNew = true
				}
				r.scoreDelta = rep.Score.Score - prev.LastScore
				r.newFlags, r.removedFlags = diffFlags(prev.LastFlags, currentFlagLabels(rep))
			}
			results = append(results, r)

			// Update in-memory snapshot to write back at the end.
			wl.UpdateSnapshot(prev.Ticker, rep.Score.Score, rep.Score.Grade,
				currentFlagLabels(rep), rep.LatestAccession, rep.LatestFilingDate)
		}

		// Persist updated snapshots.
		_ = wl.Save()

		output := renderScanResults(results)
		return watchlistScanDoneMsg{output: output, elapsed: time.Since(start)}
	}
}

// currentFlagLabels returns the label of every active risk flag on a report
// in a deterministic, comparable form.
func currentFlagLabels(r *analysis.Report) []string {
	out := make([]string, 0, len(r.RiskFlags))
	for _, f := range r.RiskFlags {
		out = append(out, f.Label)
	}
	return out
}

// diffFlags returns (added, removed) between two flag label slices.
func diffFlags(prev, cur []string) (added, removed []string) {
	prevSet := make(map[string]bool, len(prev))
	for _, p := range prev {
		prevSet[p] = true
	}
	curSet := make(map[string]bool, len(cur))
	for _, c := range cur {
		curSet[c] = true
		if !prevSet[c] {
			added = append(added, c)
		}
	}
	for _, p := range prev {
		if !curSet[p] {
			removed = append(removed, p)
		}
	}
	return
}

func (m *Model) compareCmd(t1, t2 string) tea.Cmd {
	start := m.startTime
	return func() tea.Msg {
		ctx := context.Background()
		builder := report.NewBuilder(m.cache)

		rep1, err := builder.Build(ctx, t1)
		if err != nil {
			return compareDoneMsg{err: fmt.Errorf("%s: %w", t1, err), elapsed: time.Since(start)}
		}

		rep2, err := builder.Build(ctx, t2)
		if err != nil {
			return compareDoneMsg{err: fmt.Errorf("%s: %w", t2, err), elapsed: time.Since(start)}
		}

		output := renderCompare(rep1, rep2)
		return compareDoneMsg{output: output, elapsed: time.Since(start)}
	}
}

func (m *Model) handleTab() (tea.Model, tea.Cmd) {
	raw := strings.TrimSpace(m.textInput.Value())
	if raw == "" {
		return m, nil
	}

	// Slash commands: cycle through palette matches.
	if strings.HasPrefix(raw, "/") {
		matches := MatchSlashCommands(raw)
		if len(matches) == 0 {
			return m, nil
		}
		m.cmdSugIdx++
		if m.cmdSugIdx >= len(matches) {
			m.cmdSugIdx = 0
		}
		m.textInput.SetValue(matches[m.cmdSugIdx].Canonical() + " ")
		m.textInput.CursorEnd()
		return m, nil
	}

	input := strings.ToUpper(raw)

	// Build suggestions from watchlist + recent tickers
	if m.suggestions == nil || !strings.HasPrefix(strings.ToUpper(m.suggestions[0]), input[:1]) {
		var candidates []string
		seen := make(map[string]bool)

		// Watchlist first
		if wl, err := watchlist.Load(); err == nil {
			for _, t := range wl.Tickers() {
				if !seen[t] {
					candidates = append(candidates, t)
					seen[t] = true
				}
			}
		}
		// Then recent
		for _, t := range m.history.GetTickers() {
			if !seen[t] {
				candidates = append(candidates, t)
				seen[t] = true
			}
		}
		m.suggestions = candidates
		m.sugIdx = -1
	}

	// Filter by current input
	var matches []string
	for _, s := range m.suggestions {
		if strings.HasPrefix(s, input) && s != input {
			matches = append(matches, s)
		}
	}

	if len(matches) == 0 {
		m.sugIdx = -1
		return m, nil
	}

	// Cycle through matches
	m.sugIdx++
	if m.sugIdx >= len(matches) {
		m.sugIdx = 0
	}

	m.textInput.SetValue(matches[m.sugIdx])
	m.textInput.CursorEnd()
	// Store filtered matches for the ghost display
	m.suggestions = matches
	return m, nil
}

func (m *Model) historyUp() (tea.Model, tea.Cmd) {
	if m.histItems == nil {
		m.histItems = m.history.GetInputs()
	}
	if len(m.histItems) == 0 {
		return m, nil
	}
	if m.histIdx == -1 {
		m.histDraft = m.textInput.Value()
	}
	if m.histIdx < len(m.histItems)-1 {
		m.histIdx++
	}
	m.textInput.SetValue(m.histItems[m.histIdx])
	m.textInput.CursorEnd()
	return m, nil
}

func (m *Model) historyDown() (tea.Model, tea.Cmd) {
	if m.histIdx <= -1 {
		return m, nil
	}
	m.histIdx--
	if m.histIdx == -1 {
		m.textInput.SetValue(m.histDraft)
	} else {
		m.textInput.SetValue(m.histItems[m.histIdx])
	}
	m.textInput.CursorEnd()
	return m, nil
}

func (m *Model) goBack() {
	if m.lastFilings != nil {
		m.state = stateFilings
		m.output = "" // clean for filings view
	} else {
		m.state = stateInput
	}
}

func (m *Model) handleReadCmd(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m.output = StyleError.Render("  ✗ Usage: /read N")
		m.goBack()
		return m, nil
	}

	ticker := m.lastTicker
	idxStr := parts[1]
	if len(parts) >= 3 {
		ticker = strings.ToUpper(parts[1])
		idxStr = parts[2]
	}

	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		m.output = StyleError.Render("  ✗ Invalid index: " + idxStr)
		m.goBack()
		return m, nil
	}

	if m.lastFilings == nil {
		m.output = StyleError.Render("  ✗ Run /filings first")
		m.state = stateInput
		return m, nil
	}

	if idx < 0 || idx >= len(m.lastFilings) {
		m.output = StyleError.Render(fmt.Sprintf("  ✗ Index %d out of range (0-%d)", idx, len(m.lastFilings)-1))
		m.goBack()
		return m, nil
	}

	m.lastTicker = ticker
	filing := m.lastFilings[idx]
	m.state = stateLoading
	m.statusMsg = fmt.Sprintf("Downloading %s filed %s...", filing.Form, filing.FilingDate.Format("2006-01-02"))
	return m, m.readFilingCmd(ticker, idx)
}

func (m *Model) handleAnalyzeCmd(parts []string) (tea.Model, tea.Cmd) {
	if DetectAIStatus() == "" {
		m.output = StyleError.Render("  ✗ No AI configured. Set OPENAI_API_KEY or ANTHROPIC_API_KEY")
		m.goBack()
		return m, nil
	}

	if len(parts) < 2 {
		m.output = StyleError.Render("  ✗ Usage: /analyze N")
		m.goBack()
		return m, nil
	}

	ticker := m.lastTicker
	idxStr := parts[1]
	if len(parts) >= 3 {
		ticker = strings.ToUpper(parts[1])
		idxStr = parts[2]
	}

	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		m.output = StyleError.Render("  ✗ Invalid index: " + idxStr)
		m.goBack()
		return m, nil
	}

	if m.lastFilings == nil {
		m.output = StyleError.Render("  ✗ Run /filings first")
		m.state = stateInput
		return m, nil
	}

	if idx < 0 || idx >= len(m.lastFilings) {
		m.output = StyleError.Render(fmt.Sprintf("  ✗ Index %d out of range (0-%d)", idx, len(m.lastFilings)-1))
		m.goBack()
		return m, nil
	}

	m.lastTicker = ticker
	filing := m.lastFilings[idx]
	m.state = stateLoading
	m.statusMsg = fmt.Sprintf("Analyzing %s filed %s...", filing.Form, filing.FilingDate.Format("2006-01-02"))
	return m, m.analyzeFilingCmd(ticker, idx)
}

// --- Async commands ---

func (m *Model) runReportCmd(ticker string) tea.Cmd {
	start := m.startTime
	return func() tea.Msg {
		ctx := context.Background()
		builder := report.NewBuilder(m.cache)
		rep, err := builder.Build(ctx, ticker)
		if err != nil {
			return reportDoneMsg{err: err, elapsed: time.Since(start)}
		}

		cik, _, _ := m.edgarClient.ResolveCIK(ctx, ticker)

		var output string
		switch m.outputMode {
		case "json":
			output = captureOutput(func() { report.RenderJSON(rep) })
		case "md":
			output = captureOutput(func() { report.RenderMarkdown(rep) })
		default:
			output = report.RenderString(rep)
		}

		return reportDoneMsg{report: rep, output: output, cik: cik, elapsed: time.Since(start)}
	}
}

func (m *Model) loadFilingsCmd(ticker, formFilter string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Always resolve CIK for the requested ticker
		cik, _, err := m.edgarClient.ResolveCIK(ctx, ticker)
		if err != nil {
			return filingsDoneMsg{err: err}
		}

		formTypes := []string{"S-3", "S-3/A", "424B5", "424B2", "424B3", "10-K", "10-Q", "8-K", "SC 13D", "S-1", "F-3"}
		if formFilter != "" {
			formTypes = []string{formFilter}
			if !strings.HasSuffix(formFilter, "/A") {
				formTypes = append(formTypes, formFilter+"/A")
			}
		}

		filings, err := m.edgarClient.ListFilings(ctx, cik, formTypes, 20)
		if err != nil {
			return filingsDoneMsg{err: err, cik: cik}
		}

		output := renderFilingsListBubble(ticker, filings)
		return filingsDoneMsg{filings: filings, output: output, cik: cik}
	}
}

func (m *Model) readFilingCmd(ticker string, idx int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		filing := m.lastFilings[idx]

		doc, err := m.edgarClient.GetFilingDocument(ctx, m.lastCIK, filing)
		if err != nil {
			return readDoneMsg{err: err}
		}
		doc.Ticker = ticker

		text := doc.CleanText
		if len(text) > 5000 {
			text = text[:5000] + "\n\n" + StyleInfo.Render("  [Truncated at 5000 chars. Use CLI --max-chars 0 for full text]")
		}

		output := fmt.Sprintf("\n%s\n  URL: %s\n\n%s",
			StyleSection.Render(fmt.Sprintf("  ─── %s — %s ───", doc.Form, doc.FilingDate)),
			doc.URL,
			text,
		)

		return readDoneMsg{output: output, doc: doc}
	}
}

func (m *Model) analyzeFilingCmd(ticker string, idx int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		filing := m.lastFilings[idx]

		doc, err := m.edgarClient.GetFilingDocument(ctx, m.lastCIK, filing)
		if err != nil {
			return analysisDoneMsg{err: err}
		}
		doc.Ticker = ticker

		result, err := analysis.AnalyzeFiling(ctx, doc)
		if err != nil {
			return analysisDoneMsg{err: err}
		}

		output := renderAnalysisBubble(doc, result)
		return analysisDoneMsg{output: output}
	}
}

func (m *Model) analyzeDocCmd(doc *edgar.FilingDocument) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		result, err := analysis.AnalyzeFiling(ctx, doc)
		if err != nil {
			return analysisDoneMsg{err: err}
		}
		output := renderAnalysisBubble(doc, result)
		return analysisDoneMsg{output: output}
	}
}
