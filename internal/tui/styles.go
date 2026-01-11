package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette - modern dark theme.
var (
	colorPurple     = lipgloss.Color("#4f8cff")
	colorPink       = lipgloss.Color("#f472b6")
	colorGreen      = lipgloss.Color("#2fd576")
	colorYellow     = lipgloss.Color("#f2c94c")
	colorCyan       = lipgloss.Color("#5ad1e9")
	colorOrange     = lipgloss.Color("#f5a15f")
	colorRed        = lipgloss.Color("#ff6b6b")
	colorWhite      = lipgloss.Color("#e6edf3")
	colorGray       = lipgloss.Color("#9aa4b2")
	colorDarkGray   = lipgloss.Color("#1f2937")
	colorBackground = lipgloss.Color("#0b1220")
)

// Styles holds all the lipgloss styles for the TUI.
type Styles struct {
	// Header styles
	Header lipgloss.Style

	// Tab styles
	Tab       lipgloss.Style
	ActiveTab lipgloss.Style

	// List item styles
	Item         lipgloss.Style
	SelectedItem lipgloss.Style
	Active       lipgloss.Style

	// Status bar styles
	StatusBar  lipgloss.Style
	StatusKey  lipgloss.Style
	StatusText lipgloss.Style

	// Empty state
	Empty lipgloss.Style

	// Help screen
	Help lipgloss.Style

	// Dialog styles
	Dialog             lipgloss.Style
	DialogTitle        lipgloss.Style
	DialogButton       lipgloss.Style
	DialogButtonActive lipgloss.Style
}

// DefaultStyles returns the default style configuration.
func DefaultStyles() Styles {
	return Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPurple).
			MarginBottom(1),

		Tab: lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(colorGray).
			Border(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderTop(false).
			BorderLeft(false).
			BorderRight(false).
			BorderForeground(colorDarkGray),

		ActiveTab: lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(colorWhite).
			Bold(true).
			Border(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderTop(false).
			BorderLeft(false).
			BorderRight(false).
			BorderForeground(colorPurple),

		Item: lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorWhite),

		SelectedItem: lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorWhite).
			Bold(true).
			Background(colorDarkGray),

		Active: lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true),

		StatusBar: lipgloss.NewStyle().
			Padding(0, 1).
			Background(colorDarkGray).
			Foreground(colorWhite),

		StatusKey: lipgloss.NewStyle().
			Foreground(colorPurple).
			Bold(true),

		StatusText: lipgloss.NewStyle().
			Foreground(colorGray),

		Empty: lipgloss.NewStyle().
			Foreground(colorGray).
			Italic(true).
			Padding(2, 4),

		Help: lipgloss.NewStyle().
			Padding(2, 4).
			Foreground(colorWhite),

		Dialog: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPurple).
			Padding(1, 2),

		DialogTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPurple).
			MarginBottom(1),

		DialogButton: lipgloss.NewStyle().
			Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGray),

		DialogButtonActive: lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(colorWhite).
			Background(colorPurple).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPurple),
	}
}
