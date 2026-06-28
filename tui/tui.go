package tui

import (
	"apoxy/config"
	"apoxy/proxy"
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	cfg        *config.Config
	configPath string
	runner     *proxy.Runner
	ctx        context.Context
	cancel     context.CancelFunc
	state      state
	menuCursor int
	results    []*proxy.APIProxyResult
	lastResult *proxy.APIProxyResult
	scanned, alive, totalModels int
	avgLatency time.Duration
	errors     []string
	logLines   []string
	width, height int
}

type state int
const ( stateMenu state = iota; stateScanning; stateDone )

func NewModel(cfg *config.Config, configPath string) *model {
	return &model{cfg: cfg, configPath: configPath, state: stateMenu}
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateMenu: return m.updateMenu(msg)
		case stateScanning: return m.updateScanning(msg)
		case stateDone: return m.updateDone(msg)
		}
	case tea.WindowSizeMsg: m.width, m.height = msg.Width, msg.Height
	case proxy.RunnerEvent: return m.handleEvent(msg)
	}
	return m, nil
}

func (m *model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up","k": if m.menuCursor > 0 { m.menuCursor-- }
	case "down","j": if m.menuCursor < 3 { m.menuCursor++ }
	case "enter":
		switch m.menuCursor {
		case 0: return m.startScan()
		case 1: m.cfg.CheckModels = false; return m.startScan()
		case 2: m.logLines = append(m.logLines, "Edit config.yaml to add sources"); m.menuCursor = 0
		case 3: return m, tea.Quit
		}
	case "q","ctrl+c": return m, tea.Quit
	}
	return m, nil
}

func (m *model) startScan() (tea.Model, tea.Cmd) {
	m.state = stateScanning; m.results = nil; m.scanned, m.alive, m.totalModels = 0, 0, 0
	m.errors, m.logLines = nil, nil
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.runner = proxy.NewRunner(m.cfg)
	m.runner.Start(m.ctx)
	return m, waitForEvent(m.runner.EventChan)
}

func (m *model) updateScanning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case " ": if m.runner.IsPaused() { m.runner.Resume() } else { m.runner.Pause() }
	case "esc","q": m.runner.Stop(); m.state = stateDone
	}
	return m, waitForEvent(m.runner.EventChan)
}

func (m *model) updateDone(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y": proxy.ExportResults(m.results, m.cfg.ExportPath)
	case "h": proxy.ExportHermesFormat(m.results, "exports/hermes_providers.yaml")
	case "enter","r": m.state = stateMenu; m.menuCursor = 0
	case "q","ctrl+c": return m, tea.Quit
	}
	return m, nil
}

func (m *model) handleEvent(event proxy.RunnerEvent) (tea.Model, tea.Cmd) {
	switch event.Type {
	case proxy.EventScanStart: m.logLines = append(m.logLines, fmt.Sprintf("Found %d endpoints", event.Payload.(int)))
	case proxy.EventProxyChecked:
		payload := event.Payload.(proxy.ProxyCheckedPayload)
		m.scanned++; m.lastResult = payload.Result
		if payload.Result.Alive {
			m.alive++; m.totalModels += payload.Result.ModelsCount; m.results = append(m.results, payload.Result)
			unlim := ""
			if payload.Result.Unlimited { unlim = " [UNLIMITED]" }
			m.logLines = append(m.logLines, fmt.Sprintf("OK %s (%d models%s)", payload.Result.URL, payload.Result.ModelsCount, unlim))
		}
	case proxy.EventScanDone:
		if m.alive > 0 {
			var total time.Duration
			for _, r := range m.results { total += r.Latency }
			m.avgLatency = total / time.Duration(m.alive)
		}
		m.state = stateDone; return m, nil
	case proxy.EventError: m.logLines = append(m.logLines, fmt.Sprintf("ERR %v", event.Payload))
	}
	return m, waitForEvent(m.runner.EventChan)
}

func waitForEvent(ch <-chan proxy.RunnerEvent) tea.Cmd {
	return func() tea.Msg { event, ok := <-ch; if !ok { return nil }; return event }
}

func (m *model) View() string {
	switch m.state {
	case stateMenu: return m.viewMenu()
	case stateScanning: return m.viewScanning()
	case stateDone: return m.viewDone()
	}
	return ""
}

func (m *model) viewMenu() string {
	title := titleStyle.Render("APOXY - API Proxy Scanner")
	subtitle := subtitleStyle.Render("Finds OpenAI-compatible API endpoints")
	items := []string{"Full Scan (check models)", "Quick Scan (health only)", "Add Sources (config.yaml)", "Quit"}
	var menu strings.Builder
	for i, item := range items {
		if i == m.menuCursor { menu.WriteString(selectedStyle.Render("> "+item)+"\n") } else { menu.WriteString("  "+item+"\n") }
	}
	info := fmt.Sprintf("Sources: %d | Threads: %d | Timeout: %v", len(m.cfg.Sources), m.cfg.Threads, m.cfg.Timeout)
	return lipgloss.JoinVertical(lipgloss.Center, title, subtitle, "", menu.String(), "", dimStyle.Render(info), "", dimStyle.Render("nav / Enter / Q"))
}

func (m *model) viewScanning() string {
	status := "Scanning..."
	if m.runner != nil && m.runner.IsPaused() { status = "PAUSED" }
	stats := fmt.Sprintf("Scanned: %d | Alive: %d | Models: %d", m.scanned, m.alive, m.totalModels)
	var last string
	if m.lastResult != nil {
		if m.lastResult.Alive { last = fmt.Sprintf("Last: OK %s (%v)", m.lastResult.URL, m.lastResult.Latency.Round(time.Millisecond)) } else { last = fmt.Sprintf("Last: FAIL %s", m.lastResult.URL) }
	}
	var log strings.Builder
	start := 0
	if len(m.logLines) > 15 { start = len(m.logLines) - 15 }
	for _, l := range m.logLines[start:] { log.WriteString(dimStyle.Render(l)+"\n") }
	return lipgloss.JoinVertical(lipgloss.Left, titleStyle.Render(status), subtitleStyle.Render(stats), "", last, "", log.String(), "", dimStyle.Render("Space=Pause Esc=Stop"))
}

func (m *model) viewDone() string {
	status := fmt.Sprintf("DONE - %d alive / %d scanned", m.alive, m.scanned)
	stats := fmt.Sprintf("Models: %d | Avg latency: %v", m.totalModels, m.avgLatency)
	var res strings.Builder
	for i, r := range m.results {
		if i >= 20 { res.WriteString(fmt.Sprintf("  ... +%d more\n", len(m.results)-20)); break }
		unlim := ""
		if r.Unlimited { unlim = " UNLIMITED" }
		res.WriteString(fmt.Sprintf("  %d. %s | %v | %d models%s\n", i+1, r.URL, r.Latency.Round(time.Millisecond), r.ModelsCount, unlim))
	}
	return lipgloss.JoinVertical(lipgloss.Left, titleStyle.Render(status), subtitleStyle.Render(stats), "", res.String(), "", dimStyle.Render("Y=JSON H=Hermes R=Rescan Q=Quit"))
}
