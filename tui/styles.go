package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Palettes (Professional Dark Slate & Purples)
	BrandColor   = lipgloss.Color("#7C3AED") // Rich Violet
	LightPurple  = lipgloss.Color("#C084FC") // Soft Lavender
	DeepPurple   = lipgloss.Color("#4C1D95") // Royal Purple
	BgDark       = lipgloss.Color("#0F172A") // Deep Slate Black
	TextMuted    = lipgloss.Color("#94A3B8") // Slate Gray
	White        = lipgloss.Color("#F8FAFC") // Off-White
	GreenSuccess = lipgloss.Color("#10B981") // Emerald Green
	RedError     = lipgloss.Color("#EF4444") // Coral Red

	// Typography & Styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(White).
			Background(BrandColor).
			Padding(0, 2).
			MarginTop(1).
			MarginBottom(1)

	BannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(BrandColor).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(TextMuted).
			Italic(true)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(DeepPurple).
			Padding(1, 2).
			Margin(0, 0)

	LivePanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BrandColor).
			Padding(1, 2).
			Margin(0, 0)

	InfoLabelStyle = lipgloss.NewStyle().
			Foreground(LightPurple).
			Bold(true)

	ValueStyle = lipgloss.NewStyle().
			Foreground(White)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(GreenSuccess).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(RedError).
			Bold(true)

	HelpStyle = lipgloss.NewStyle().
			Foreground(TextMuted).
			MarginTop(1)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(LightPurple).
			Bold(true)

	SelectedRowStyle = lipgloss.NewStyle().
				Background(BrandColor).
				Foreground(White).
				Bold(true)

	TableRowStyle = lipgloss.NewStyle().
			Foreground(White)

	// Direct Style Wrappers
	GrayStyle      = lipgloss.NewStyle().Foreground(TextMuted)
	SecondaryStyle = lipgloss.NewStyle().Foreground(LightPurple)
	GreenStyle     = lipgloss.NewStyle().Foreground(GreenSuccess)
	RedStyle       = lipgloss.NewStyle().Foreground(RedError)
)
