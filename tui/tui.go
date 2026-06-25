package tui

import (
	"context"
	"fmt"
	"minoxy/config"
	"minoxy/proxy"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Screen int

const (
	ScreenWelcome Screen = iota
	ScreenChecking
	ScreenExport
)

type runnerClosedMsg struct{}
type eventMsg proxy.RunnerEvent

type Model struct {
	screen     Screen
	config     *config.Config
	configPath string
	runner     *proxy.Runner
	ctx        context.Context
	cancelFunc context.CancelFunc

	// Checking State
	scrapedCount  int
	checkedCount  int
	liveCount     int
	httpCount     int
	socks4Count   int
	socks5Count   int
	avgPing       time.Duration
	scrapingDone  bool
	checkingDone  bool
	currentSource string
	recentLive    []*proxy.CheckedProxy
	allLive       []*proxy.CheckedProxy
	progressBar   progress.Model

	// Welcome Screen Menu
	welcomeIdx int // 0: Start, 1: Toggle Settings, 2: Exit

	// Export Screen Form Navigation
	exportIdx    int // 0: HTTP, 1: SOCKS4, 2: SOCKS5, 3: CC, 4: Ping, 5: Format, 6: Path, 7: ExportBtn, 8: BackBtn
	focusedField int // 0: menu navigation, 3: CC editing, 4: Ping editing, 6: Path editing

	// Filters & Form inputs
	filterHTTP   bool
	filterSocks4 bool
	filterSocks5 bool
	filterCC     textinput.Model // Country Code input
	filterPing   textinput.Model // Max Ping input

	// Export Path Input
	exportPath   textinput.Model
	exportFormat int // index of ["raw", "uri", "pretty", "csv", "json"]
	exportMsg    string
	exportErr    bool
	showConfig   bool // Welcome screen toggle

	width  int
	height int
}

func NewModel(cfg *config.Config, configPath string) Model {
	pBar := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
	)

	filterCC := textinput.New()
	filterCC.Placeholder = "All (e.g. US,GB)"
	filterCC.Width = 25

	filterPing := textinput.New()
	filterPing.Placeholder = "No limit (e.g. 500ms)"
	filterPing.Width = 25

	exportPath := textinput.New()
	exportPath.SetValue(cfg.ExportPath)
	exportPath.Width = 30

	return Model{
		screen:       ScreenWelcome,
		config:       cfg,
		configPath:   configPath,
		progressBar:  pBar,
		filterHTTP:   true,
		filterSocks4: true,
		filterSocks5: true,
		filterCC:     filterCC,
		filterPing:   filterPing,
		exportPath:   exportPath,
		exportFormat: 2, // Default to "pretty"
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func listenForEvents(ch chan proxy.RunnerEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return runnerClosedMsg{}
		}
		return eventMsg(event)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progressBar.Width = msg.Width - 10
		if m.progressBar.Width > 60 {
			m.progressBar.Width = 60
		}
		return m, nil

	case runnerClosedMsg:
		m.checkingDone = true
		m.runner = nil
		return m, nil

	case eventMsg:
		switch msg.Type {
		case proxy.EventScrapingSource:
			m.currentSource = msg.Payload.(string)
		case proxy.EventScrapeError:
			m.currentSource = msg.Payload.(string)
		case proxy.EventSourceScraped:
			// stats are calculated dynamically below
		case proxy.EventScrapingDone:
			m.scrapingDone = true
			m.scrapedCount = msg.Payload.(int)
		case proxy.EventProxyChecked:
			payload := msg.Payload.(proxy.EventProxyCheckedPayload)
			scraped, checked, live, http, s4, s5, _, avg := m.runner.GetStats()
			m.scrapedCount = scraped
			m.checkedCount = checked
			m.liveCount = live
			m.httpCount = http
			m.socks4Count = s4
			m.socks5Count = s5
			m.avgPing = avg

			if payload.IsLive && payload.Proxy != nil {
				m.allLive = append(m.allLive, payload.Proxy)
				m.recentLive = append(m.recentLive, payload.Proxy)
				if len(m.recentLive) > 4 { // keep last 4 for small screen
					m.recentLive = m.recentLive[1:]
				}
			}

		case proxy.EventCheckingDone:
			m.checkingDone = true
			if payload, ok := msg.Payload.([]*proxy.CheckedProxy); ok {
				m.allLive = payload
			}
		}

		if m.runner != nil {
			return m, listenForEvents(m.runner.EventChan)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.runner != nil {
				m.runner.Stop()
			}
			if m.cancelFunc != nil {
				m.cancelFunc()
			}
			return m, tea.Quit
		}

		switch m.screen {
		case ScreenWelcome:
			return m.updateWelcome(msg)
		case ScreenChecking:
			return m.updateChecking(msg)
		case ScreenExport:
			return m.updateExport(msg)
		}
	}

	return m, nil
}

func (m *Model) startChecker() tea.Cmd {
	m.screen = ScreenChecking
	m.scrapedCount = 0
	m.checkedCount = 0
	m.liveCount = 0
	m.httpCount = 0
	m.socks4Count = 0
	m.socks5Count = 0
	m.avgPing = 0
	m.scrapingDone = false
	m.checkingDone = false
	m.recentLive = nil
	m.allLive = nil

	m.runner = proxy.NewRunner(m.config)
	m.ctx, m.cancelFunc = context.WithCancel(context.Background())
	m.runner.Start(m.ctx)

	return listenForEvents(m.runner.EventChan)
}

func (m Model) updateWelcome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.welcomeIdx > 0 {
			m.welcomeIdx--
		}
	case "down":
		if m.welcomeIdx < 2 {
			m.welcomeIdx++
		}
	case "enter":
		switch m.welcomeIdx {
		case 0: // Start
			return m, m.startChecker()
		case 1: // Toggle Settings
			m.showConfig = !m.showConfig
		case 2: // Exit
			return m, tea.Quit
		}
	case "q", "esc":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateChecking(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case " ": // Pause / Resume
		if m.runner != nil {
			if m.runner.IsPaused() {
				m.runner.Resume()
			} else {
				m.runner.Pause()
			}
		}
	case "enter":
		if m.checkingDone {
			m.screen = ScreenExport
			m.exportIdx = 7 // focus export button directly
		}
	case "esc":
		if m.runner != nil {
			m.runner.Stop()
		}
		if m.cancelFunc != nil {
			m.cancelFunc()
		}
		m.checkingDone = true
		m.screen = ScreenExport
		m.exportIdx = 7
	case "q":
		if m.runner != nil {
			m.runner.Stop()
		}
		if m.cancelFunc != nil {
			m.cancelFunc()
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// If typing in a field, send keys to it
	if m.focusedField != 0 {
		switch msg.String() {
		case "enter":
			// Save and return to form navigation
			m.focusedField = 0
			m.filterCC.Blur()
			m.filterPing.Blur()
			m.exportPath.Blur()
			return m, nil
		case "esc":
			// Cancel and return to form navigation
			m.focusedField = 0
			m.filterCC.Blur()
			m.filterPing.Blur()
			m.exportPath.Blur()
			return m, nil
		default:
			switch m.focusedField {
			case 3:
				m.filterCC, cmd = m.filterCC.Update(msg)
				return m, cmd
			case 4:
				m.filterPing, cmd = m.filterPing.Update(msg)
				return m, cmd
			case 6:
				m.exportPath, cmd = m.exportPath.Update(msg)
				return m, cmd
			}
		}
	}

	// Normal form item navigation
	switch msg.String() {
	case "up":
		if m.exportIdx > 0 {
			m.exportIdx--
		}
	case "down":
		if m.exportIdx < 8 {
			m.exportIdx++
		}
	case "left":
		if m.exportIdx == 5 { // Format selector
			if m.exportFormat > 0 {
				m.exportFormat--
			}
		}
	case "right":
		if m.exportIdx == 5 { // Format selector
			if m.exportFormat < 4 {
				m.exportFormat++
			}
		}
	case "enter", " ":
		m.exportMsg = "" // Clear message when navigating
		switch m.exportIdx {
		case 0:
			m.filterHTTP = !m.filterHTTP
		case 1:
			m.filterSocks4 = !m.filterSocks4
		case 2:
			m.filterSocks5 = !m.filterSocks5
		case 3: // Edit CC
			m.focusedField = 3
			m.filterCC.Focus()
		case 4: // Edit Ping
			m.focusedField = 4
			m.filterPing.Focus()
		case 5: // Cycle format on enter as well
			if msg.String() == "enter" {
				m.exportFormat = (m.exportFormat + 1) % 5
			}
		case 6: // Edit Path
			m.focusedField = 6
			m.exportPath.Focus()
		case 7: // Export Button
			m.triggerExport()
		case 8: // Back Button
			m.screen = ScreenWelcome
		}
	case "esc":
		m.screen = ScreenWelcome
	case "q":
		return m, tea.Quit
	}

	return m, nil
}

func (m *Model) triggerExport() {
	m.exportMsg = ""
	m.exportErr = false

	ccFilter := strings.TrimSpace(strings.ToUpper(m.filterCC.Value()))
	var countries []string
	if ccFilter != "" {
		parts := strings.Split(ccFilter, ",")
		for _, p := range parts {
			countries = append(countries, strings.TrimSpace(p))
		}
	}

	pingLimit := time.Duration(0)
	pingVal := strings.TrimSpace(m.filterPing.Value())
	if pingVal != "" {
		if !strings.HasSuffix(pingVal, "ms") && !strings.HasSuffix(pingVal, "s") {
			pingVal += "ms"
		}
		if d, err := time.ParseDuration(pingVal); err == nil {
			pingLimit = d
		}
	}

	filter := proxy.ExportFilter{
		Protocols: nil,
		MaxPing:   pingLimit,
		Countries: countries,
	}
	if m.filterHTTP {
		filter.Protocols = append(filter.Protocols, "http")
	}
	if m.filterSocks4 {
		filter.Protocols = append(filter.Protocols, "socks4")
	}
	if m.filterSocks5 {
		filter.Protocols = append(filter.Protocols, "socks5")
	}

	if len(filter.Protocols) == 0 {
		m.exportMsg = "Error: Select at least one proxy protocol!"
		m.exportErr = true
		return
	}

	formats := []string{"raw", "uri", "pretty", "csv", "json"}
	fmtName := formats[m.exportFormat]

	path := strings.TrimSpace(m.exportPath.Value())
	if path == "" {
		m.exportMsg = "Error: File path cannot be empty!"
		m.exportErr = true
		return
	}

	count, err := proxy.Export(m.allLive, filter, fmtName, path)
	if err != nil {
		m.exportMsg = fmt.Sprintf("Error: %v", err)
		m.exportErr = true
	} else {
		m.exportMsg = fmt.Sprintf("Success! Exported %d proxies to %s", count, path)
		m.exportErr = false
	}
}
