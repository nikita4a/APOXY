package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	var s string
	switch m.screen {
	case ScreenWelcome:
		s = m.viewWelcome()
	case ScreenChecking:
		s = m.viewChecking()
	case ScreenExport:
		s = m.viewExport()
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(s)
}

func (m Model) viewWelcome() string {
	// Large Premium Block ASCII Art in Violet
	banner := `
 ███╗   ███╗██╗███╗   ██╗ ██████╗ ██╗  ██╗██╗   ██╗
 ████╗ ████║██║████╗  ██║██╔═══██╗╚██╗██╔╝╚██╗ ██╔╝
 ██╔████╔██║██║██╔██╗ ██║██║   ██║ ╚███╔╝  ╚████╔╝ 
 ██║╚██╔╝██║██║██║╚██╗██║██║   ██║ ██╔██╗   ╚██╔╝  
 ██║ ╚═╝ ██║██║██║ ╚████║╚██████╔╝██╔╝ ██╗   ██║   
 ╚═╝     ╚═╝╚═╝╚═╝  ╚═══╝ ╚═════╝ ╚═╝  ╚═╝   ╚═╝   
`
	header := BannerStyle.Render(banner) + "\n" +
		SubtitleStyle.Render("MINOXY — High-performance asynchronous proxy checker in Go") + "\n"

	// Divider
	divider := GrayStyle.Render(strings.Repeat("━", 54)) + "\n"

	// Welcome Menu lines
	var menuLines []string
	options := []string{
		"Start proxy checking",
		"Show / Hide configuration settings",
		"Exit",
	}

	for i, opt := range options {
		if i == m.welcomeIdx {
			menuLines = append(menuLines, SelectedRowStyle.Width(50).Render(fmt.Sprintf("  ➔  %s", opt)))
		} else {
			menuLines = append(menuLines, fmt.Sprintf("     %s", ValueStyle.Render(opt)))
		}
	}

	menuPanel := PanelStyle.Width(54).Render("📋 MAIN MENU\n\n" + strings.Join(menuLines, "\n"))

	var body string
	if m.showConfig {
		var confLines []string
		confLines = append(confLines, fmt.Sprintf("%-18s : %s", InfoLabelStyle.Render("Config file"), ValueStyle.Render(m.configPath)))
		confLines = append(confLines, fmt.Sprintf("%-18s : %s", InfoLabelStyle.Render("Threads"), ValueStyle.Render(fmt.Sprintf("%d workers", m.config.Threads))))
		confLines = append(confLines, fmt.Sprintf("%-18s : %s", InfoLabelStyle.Render("Timeout"), ValueStyle.Render(m.config.Timeout.String())))
		confLines = append(confLines, fmt.Sprintf("%-18s : %s", InfoLabelStyle.Render("Check URL"), ValueStyle.Render(m.config.CheckURL)))
		confLines = append(confLines, fmt.Sprintf("%-18s : %s", InfoLabelStyle.Render("Export path"), ValueStyle.Render(m.config.ExportPath)))

		var srcLines []string
		for i, src := range m.config.Sources {
			if i >= 3 {
				srcLines = append(srcLines, GrayStyle.Render(fmt.Sprintf("  ... and %d more sources", len(m.config.Sources)-3)))
				break
			}
			shortSrc := src
			if len(shortSrc) > 42 {
				shortSrc = shortSrc[:39] + "..."
			}
			srcLines = append(srcLines, fmt.Sprintf("  • %s", ValueStyle.Render(shortSrc)))
		}

		confPanel := PanelStyle.Width(54).Render("⚙️  CURRENT SETTINGS\n\n" + strings.Join(confLines, "\n") + "\n\n📂 CONNECTED SOURCES:\n" + strings.Join(srcLines, "\n"))
		body = lipgloss.JoinVertical(lipgloss.Left, menuPanel, "", confPanel)
	} else {
		body = menuPanel
	}

	footer := HelpStyle.Render("\n[ ↑/↓ ] Navigation  •  [ Enter ] Select Action  •  [ q ] Exit")

	return lipgloss.JoinVertical(lipgloss.Left, header, divider, body, footer)
}

func (m Model) viewChecking() string {
	titleText := "⚡ SCRAPING & CHECKING PROXIES..."
	if m.checkingDone {
		titleText = "✅ CHECKING COMPLETE!"
	} else if m.runner != nil && m.runner.IsPaused() {
		titleText = "⏸️  CHECKING PAUSED"
	}

	title := TitleStyle.Render(titleText)
	divider := GrayStyle.Render(strings.Repeat("━", 54)) + "\n"

	// Stats content
	var statsLines []string
	statsLines = append(statsLines, fmt.Sprintf("%-18s : %s", InfoLabelStyle.Render("Workers (Threads)"), ValueStyle.Render(strconv.Itoa(m.config.Threads))))
	statsLines = append(statsLines, fmt.Sprintf("%-18s : %s", InfoLabelStyle.Render("Total found"), ValueStyle.Render(strconv.Itoa(m.scrapedCount))))
	statsLines = append(statsLines, fmt.Sprintf("%-18s : %s", InfoLabelStyle.Render("Proxies checked"), ValueStyle.Render(strconv.Itoa(m.checkedCount))))

	deadCount := m.checkedCount - m.liveCount
	statsLines = append(statsLines, fmt.Sprintf("%-18s : %s", InfoLabelStyle.Render("Unavailable (Dead)"), RedStyle.Render(strconv.Itoa(deadCount))))

	statsLines = append(statsLines, "")
	statsLines = append(statsLines, SuccessStyle.Render(fmt.Sprintf("LIVE: %d", m.liveCount)))
	statsLines = append(statsLines, fmt.Sprintf("  HTTP  : %-4s | SOCKS4: %-4s | SOCKS5: %s",
		ValueStyle.Render(strconv.Itoa(m.httpCount)),
		ValueStyle.Render(strconv.Itoa(m.socks4Count)),
		ValueStyle.Render(strconv.Itoa(m.socks5Count))))

	avgPingStr := "0ms"
	if m.avgPing > 0 {
		avgPingStr = fmt.Sprintf("%dms", m.avgPing.Milliseconds())
	}
	statsLines = append(statsLines, fmt.Sprintf("  Avg ping: %s", SecondaryStyle.Render(avgPingStr)))

	statsPanel := PanelStyle.Width(54).Render("📊 RUNTIME STATISTICS\n\n" + strings.Join(statsLines, "\n"))

	// Display only one line for the last live proxy to keep layout small
	var liveLogLine string
	if len(m.recentLive) > 0 {
		p := m.recentLive[len(m.recentLive)-1]
		pingMs := fmt.Sprintf("%dms", p.Ping.Milliseconds())
		flag := p.Flag
		if flag == "" {
			flag = "🏳️"
		}
		liveLogLine = fmt.Sprintf("🟢 Last live: %s %s:%d (%s) - %s",
			flag, p.IP, p.Port, SecondaryStyle.Render(p.Protocol), GreenStyle.Render(pingMs))
	} else {
		liveLogLine = GrayStyle.Render("🟢 Last live: waiting for live proxies...")
	}

	// Progress bar
	pct := 0.0
	if m.scrapedCount > 0 {
		pct = float64(m.checkedCount) / float64(m.scrapedCount)
	}
	if pct > 1.0 {
		pct = 1.0
	}
	progBar := m.progressBar.ViewAs(pct)
	progText := fmt.Sprintf("  %d%% (%d / %d checked)", int(pct*100), m.checkedCount, m.scrapedCount)

	var statusLine string
	if !m.scrapingDone {
		shortSrc := m.currentSource
		if len(shortSrc) > 42 {
			shortSrc = "..." + shortSrc[len(shortSrc)-39:]
		}
		statusLine = fmt.Sprintf("📂 %s: %s", InfoLabelStyle.Render("Searching"), GrayStyle.Render(shortSrc))
	} else {
		statusLine = fmt.Sprintf("⚡ %s", SuccessStyle.Render("Scraping complete. Checking in progress..."))
	}

	var footer string
	if m.checkingDone {
		footer = HelpStyle.Render("\n[ Enter ] Export Settings  •  [ q ] Exit")
	} else {
		footer = HelpStyle.Render("\n[ Space ] Pause/Resume  •  [ Esc ] Stop & Export  •  [ q ] Exit")
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, divider, statsPanel, "\n"+progBar+progText, "\n"+statusLine, "\n"+liveLogLine, footer)
}

func (m Model) viewExport() string {
	title := TitleStyle.Render("💾 EXPORT SETTINGS & FILTERS")
	divider := GrayStyle.Render(strings.Repeat("━", 54)) + "\n"

	summaryText := fmt.Sprintf("Total live proxies available: %s\n(HTTP: %d, SOCKS4: %d, SOCKS5: %d)",
		SuccessStyle.Render(strconv.Itoa(m.liveCount)), m.httpCount, m.socks4Count, m.socks5Count)

	summaryPanel := PanelStyle.Width(54).Render("📊 CHECKING RESULTS\n\n" + summaryText)

	// Build form lines with arrow navigation marker
	var formLines []string

	formItems := []struct {
		idx   int
		label string
		value string
	}{
		{0, "Filter HTTP", formatCheck(m.filterHTTP)},
		{1, "Filter SOCKS4", formatCheck(m.filterSocks4)},
		{2, "Filter SOCKS5", formatCheck(m.filterSocks5)},
		{3, "Countries (CC)", formatVal(m.filterCC.Value(), "All countries (ALL)")},
		{4, "Max ping", formatVal(m.filterPing.Value(), "No limit (ALL)")},
		{5, "File format", formatSelect(m.exportFormat)},
		{6, "Save path", formatVal(m.exportPath.Value(), "")},
	}

	for _, item := range formItems {
		label := item.label
		val := item.value

		if m.exportIdx == item.idx {
			if m.focusedField == item.idx {
				if item.idx == 3 {
					val = m.filterCC.View()
				} else if item.idx == 4 {
					val = m.filterPing.View()
				} else if item.idx == 6 {
					val = m.exportPath.View()
				}
			}
			rowContent := fmt.Sprintf("  ➔  %-18s : %s", label, val)
			formLines = append(formLines, SelectedRowStyle.Width(50).Render(rowContent))
		} else {
			rowContent := fmt.Sprintf("     %-18s : %s", label, val)
			formLines = append(formLines, TableRowStyle.Render(rowContent))
		}
	}

	// Add Buttons with highlighted row containers
	if m.exportIdx == 7 {
		formLines = append(formLines, "", SelectedRowStyle.Width(50).Render("  ➔  [ EXPORT PROXIES ]"))
	} else {
		formLines = append(formLines, "", "     ▶️ [ EXPORT PROXIES ]")
	}

	if m.exportIdx == 8 {
		formLines = append(formLines, SelectedRowStyle.Width(50).Render("  ➔  [ Return to main menu ]"))
	} else {
		formLines = append(formLines, "     ↩️ [ Return to main menu ]")
	}

	formPanel := PanelStyle.Width(54).Render("⚙️  EXPORT SETTINGS FORM\n\n" + strings.Join(formLines, "\n"))

	// Status Message
	var msgLine string
	if m.exportMsg != "" {
		if m.exportErr {
			msgLine = "\n" + ErrorStyle.Render(m.exportMsg)
		} else {
			msgLine = "\n" + SuccessStyle.Render(m.exportMsg)
		}
	}

	// Helper help lines
	var helpText string
	if m.focusedField != 0 {
		helpText = "Enter value and press [ Enter ] to save or [ Esc ] to cancel"
	} else {
		helpText = "[ ↑/↓ ] Field navigation  •  [ Space/Enter ] Select/Edit"
		if m.exportIdx == 5 {
			helpText = "[ ↑/↓ ] Navigation  •  [ ←/→ ] Change format"
		}
	}
	footer := HelpStyle.Render("\n" + helpText)

	return lipgloss.JoinVertical(lipgloss.Left, title, divider, summaryPanel, "", formPanel, msgLine, footer)
}

func formatCheck(val bool) string {
	if val {
		return "[✓] Enabled"
	}
	return "[ ] Disabled"
}

func formatVal(val, defaultVal string) string {
	if val == "" {
		if defaultVal != "" {
			return defaultVal
		}
		return "[ empty ]"
	}
	return fmt.Sprintf("[ %s ]", val)
}

func formatSelect(formatIdx int) string {
	formats := []string{"RAW (IP:Port)", "URI Links", "Pretty Text", "CSV Table", "JSON"}
	return fmt.Sprintf("< %s >", formats[formatIdx])
}
