package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF6B6B")).MarginBottom(1)
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A0A0A0")).MarginBottom(1)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4ECDC4")).PaddingLeft(2)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
)
