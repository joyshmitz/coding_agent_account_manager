// Package tui provides the terminal user interface for caam.
// This package uses Bubble Tea and Lipgloss from Charm.
package tui

import (
	// Import charm packages to keep them in go.mod
	_ "github.com/charmbracelet/bubbles/list"
	_ "github.com/charmbracelet/bubbletea"
	_ "github.com/charmbracelet/lipgloss"
	_ "github.com/charmbracelet/log"
)
