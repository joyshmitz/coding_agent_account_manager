package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// DetailInfo represents the detailed information for a profile.
type DetailInfo struct {
	Name        string
	Provider    string
	AuthMode    string
	LoggedIn    bool
	Locked      bool
	Path        string
	CreatedAt   time.Time
	LastUsedAt  time.Time
	Account     string
	BrowserCmd  string
	BrowserProf string
}

// DetailPanel renders the right panel showing profile details and available actions.
type DetailPanel struct {
	profile *DetailInfo
	width   int
	height  int
	styles  DetailPanelStyles
}

// DetailPanelStyles holds the styles for the detail panel.
type DetailPanelStyles struct {
	Border       lipgloss.Style
	Title        lipgloss.Style
	Label        lipgloss.Style
	Value        lipgloss.Style
	StatusOK     lipgloss.Style
	StatusBad    lipgloss.Style
	LockIcon     lipgloss.Style
	Divider      lipgloss.Style
	ActionHeader lipgloss.Style
	ActionKey    lipgloss.Style
	ActionDesc   lipgloss.Style
	Empty        lipgloss.Style
}

// DefaultDetailPanelStyles returns the default styles for the detail panel.
func DefaultDetailPanelStyles() DetailPanelStyles {
	return DetailPanelStyles{
		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDarkGray).
			Padding(0, 1),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPurple).
			MarginBottom(1),

		Label: lipgloss.NewStyle().
			Foreground(colorGray).
			Width(12),

		Value: lipgloss.NewStyle().
			Foreground(colorWhite),

		StatusOK: lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true),

		StatusBad: lipgloss.NewStyle().
			Foreground(colorRed),

		LockIcon: lipgloss.NewStyle().
			Foreground(colorYellow),

		Divider: lipgloss.NewStyle().
			Foreground(colorDarkGray),

		ActionHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan).
			MarginTop(1).
			MarginBottom(1),

		ActionKey: lipgloss.NewStyle().
			Foreground(colorPurple).
			Bold(true).
			Width(8),

		ActionDesc: lipgloss.NewStyle().
			Foreground(colorGray),

		Empty: lipgloss.NewStyle().
			Foreground(colorGray).
			Italic(true).
			Padding(2, 2),
	}
}

// NewDetailPanel creates a new detail panel.
func NewDetailPanel() *DetailPanel {
	return &DetailPanel{
		styles: DefaultDetailPanelStyles(),
	}
}

// SetProfile sets the profile to display.
func (p *DetailPanel) SetProfile(profile *DetailInfo) {
	p.profile = profile
}

// SetSize sets the panel dimensions.
func (p *DetailPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// View renders the detail panel.
func (p *DetailPanel) View() string {
	if p.profile == nil {
		empty := p.styles.Empty.Render("Select a profile to view details")
		if p.width > 0 {
			return p.styles.Border.Width(p.width - 2).Render(empty)
		}
		return p.styles.Border.Render(empty)
	}

	prof := p.profile

	// Title
	title := p.styles.Title.Render(fmt.Sprintf("Profile: %s", prof.Name))

	// Detail rows
	var rows []string

	// Provider
	rows = append(rows, p.renderRow("Provider", capitalizeFirst(prof.Provider)))

	// Auth mode
	rows = append(rows, p.renderRow("Auth", prof.AuthMode))

	// Status
	var statusStr string
	if prof.LoggedIn {
		statusStr = p.styles.StatusOK.Render("Logged in ✓")
	} else {
		statusStr = p.styles.StatusBad.Render("Not logged in")
	}
	if prof.Locked {
		statusStr += " " + p.styles.LockIcon.Render("🔒 Locked")
	}
	rows = append(rows, p.renderRow("Status", statusStr))

	// Path (truncate if too long)
	pathDisplay := prof.Path
	maxPathLen := p.width - 16
	if maxPathLen > 0 && len(pathDisplay) > maxPathLen {
		pathDisplay = "~" + pathDisplay[len(pathDisplay)-maxPathLen+1:]
	}
	rows = append(rows, p.renderRow("Path", pathDisplay))

	// Created
	if !prof.CreatedAt.IsZero() {
		rows = append(rows, p.renderRow("Created", prof.CreatedAt.Format("2006-01-02")))
	}

	// Last used
	if !prof.LastUsedAt.IsZero() {
		rows = append(rows, p.renderRow("Last used", formatRelativeTime(prof.LastUsedAt)))
	} else {
		rows = append(rows, p.renderRow("Last used", "never"))
	}

	// Account
	if prof.Account != "" {
		rows = append(rows, p.renderRow("Account", prof.Account))
	}

	// Browser config
	if prof.BrowserCmd != "" || prof.BrowserProf != "" {
		browserStr := prof.BrowserCmd
		if prof.BrowserProf != "" {
			if browserStr != "" {
				browserStr += " (" + prof.BrowserProf + ")"
			} else {
				browserStr = prof.BrowserProf
			}
		}
		rows = append(rows, p.renderRow("Browser", browserStr))
	}

	// Divider
	dividerWidth := p.width - 6
	if dividerWidth < 20 {
		dividerWidth = 20
	}
	divider := p.styles.Divider.Render(strings.Repeat("─", dividerWidth))

	// Actions header
	actionsHeader := p.styles.ActionHeader.Render("Actions")

	// Action rows
	actions := []struct {
		key  string
		desc string
	}{
		{"Enter", fmt.Sprintf("Run %s", prof.Provider)},
		{"l", "Login/refresh"},
		{"o", "Open in browser"},
		{"e", "Edit profile"},
		{"d", "Delete profile"},
		{"b", "Backup current auth"},
	}

	var actionRows []string
	for _, action := range actions {
		key := p.styles.ActionKey.Render(action.key)
		desc := p.styles.ActionDesc.Render(action.desc)
		actionRows = append(actionRows, fmt.Sprintf("%s %s", key, desc))
	}

	// Combine all sections
	detailContent := lipgloss.JoinVertical(lipgloss.Left, rows...)
	actionsContent := lipgloss.JoinVertical(lipgloss.Left, actionRows...)

	inner := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		detailContent,
		"",
		divider,
		actionsHeader,
		actionsContent,
	)

	// Apply border
	if p.width > 0 {
		return p.styles.Border.Width(p.width - 2).Render(inner)
	}
	return p.styles.Border.Render(inner)
}

// renderRow renders a label-value row.
func (p *DetailPanel) renderRow(label, value string) string {
	labelStr := p.styles.Label.Render(label + ":")
	valueStr := p.styles.Value.Render(value)
	return labelStr + " " + valueStr
}
