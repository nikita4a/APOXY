package main

import (
	"apoxy/config"
	"apoxy/proxy"
	"apoxy/tui"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	configPath := "config.yaml"
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v
", err)
		os.Exit(1)
	}
	p := tea.NewProgram(tui.NewModel(cfg, configPath), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v
", err)
		os.Exit(1)
	}
}
