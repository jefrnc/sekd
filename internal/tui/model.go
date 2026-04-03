package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jefrnc/sekd/internal/analysis"
	"github.com/jefrnc/sekd/internal/cache"
	"github.com/jefrnc/sekd/internal/config"
	"github.com/jefrnc/sekd/internal/edgar"
	"github.com/jefrnc/sekd/internal/report"
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
	report  *analysis.Report
	output  string
	cik     string
	err     error
}

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
	filCursor   int    // cursor position in filings list
	filScroll   int    // scroll offset for filings list
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

	return Model{
		textInput:   ti,
		spinner:     s,
		state:       stateInput,
		version:     version,
		banner:      renderBanner(version),
		cache:       c,
		edgarClient: edgar.NewClient(c),
		outputMode:  "terminal",
	}, nil
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

		// Result view — esc goes back to filings if available, otherwise input
		if m.state == stateResult {
			switch msg.String() {
			case "esc", "q":
				m.output = ""
				m.goBack()
				return m, nil
			}
		}

		switch msg.String() {
		case "enter":
			return m.handleEnter()
		case "esc":
			if m.state == stateInput {
				m.state = stateConfirm
				m.confirmSel = 0
				m.confirmMsg = "Exit sekd? (y/n)"
				m.confirmYes = tea.Quit
				return m, nil
			}
			m.state = stateInput
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case reportDoneMsg:
		m.state = stateResult
		if msg.err != nil {
			m.output = StyleError.Render("  ✗ " + msg.err.Error())
		} else {
			m.output = msg.output
			m.lastReport = msg.report
			m.lastCIK = msg.cik
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

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if m.state == stateInput {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	if m.state == stateInput && m.output == "" {
		b.WriteString(m.banner)
	}

	if m.output != "" {
		b.WriteString(m.output)
		b.WriteString("\n")
	}

	switch m.state {
	case stateLoading:
		b.WriteString(fmt.Sprintf("\n  %s %s\n", m.spinner.View(), m.statusMsg))

	case stateConfirm:
		b.WriteString("\n")
		b.WriteString(renderConfirmDialog(m.confirmMsg, m.confirmSel))
		b.WriteString("\n")

	case stateFilings:
		b.WriteString(renderFilingsNav(m.lastTicker, m.lastFilings, m.filCursor))

	case stateInput:
		b.WriteString("\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n")

	case stateResult:
		b.WriteString("\n")
		b.WriteString(StyleInfo.Render("  Press esc to go back"))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) handleEnter() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textInput.Value())
	m.textInput.SetValue("")

	if input == "" {
		return m, nil
	}

	if strings.HasPrefix(input, "/") {
		return m.handleCommand(input)
	}

	// Treat as ticker
	ticker := strings.ToUpper(input)
	m.lastTicker = ticker
	m.state = stateLoading
	m.statusMsg = "Fetching data for " + ticker + "..."
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

func (m *Model) goBack() {
	if m.lastFilings != nil {
		m.state = stateFilings
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
	return func() tea.Msg {
		ctx := context.Background()
		builder := report.NewBuilder(m.cache)
		rep, err := builder.Build(ctx, ticker)
		if err != nil {
			return reportDoneMsg{err: err}
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

		return reportDoneMsg{report: rep, output: output, cik: cik}
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
