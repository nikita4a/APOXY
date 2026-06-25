package main

import (
	"fmt"
	"minoxy/config"
	"minoxy/tui"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	configPath := "config.yaml"

	// Load configuration from config.yaml
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Start the interactive TUI directly using the configuration
	runTUI(cfg, configPath)
}

func runTUI(cfg *config.Config, configPath string) {
	p := tea.NewProgram(tui.NewModel(cfg, configPath), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
