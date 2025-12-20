package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/health"
	"github.com/charmbracelet/lipgloss"
)

// DetailInfo represents the detailed information for a profile.
type DetailInfo struct {
	Name         string
	Provider     string
	AuthMode     string
	LoggedIn     bool
	Locked       bool
	Path         string
	CreatedAt    time.Time
	LastUsedAt   time.Time
	Account      string
	Description  string // Free-form notes about this profile's purpose
	BrowserCmd   string
	BrowserProf  string
	HealthStatus health.HealthStatus
	TokenExpiry  time.Time
	ErrorCount   int
	Penalty      float64
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

	// Status with icon and text
	statusText := prof.HealthStatus.Icon() + " " + prof.HealthStatus.String()
	// Apply color based on status
	var statusStyle lipgloss.Style
	switch prof.HealthStatus {
	case health.StatusHealthy:
		statusStyle = p.styles.StatusOK
	case health.StatusWarning:
		statusStyle = lipgloss.NewStyle().Foreground(colorYellow)
	case health.StatusCritical:
		statusStyle = p.styles.StatusBad
	default:
		statusStyle = lipgloss.NewStyle().Foreground(colorGray)
	}
	rows = append(rows, p.renderRow("Status", statusStyle.Render(statusText)))

	// Token Expiry
	if !prof.TokenExpiry.IsZero() {
		ttl := time.Until(prof.TokenExpiry)
		expiryStr := ""
		if ttl < 0 {
			expiryStr = p.styles.StatusBad.Render("Expired")
		} else {
			expiryStr = fmt.Sprintf("Expires in %s", formatDurationFull(ttl))
		}
		rows = append(rows, p.renderRow("Token", expiryStr))
	}

	// Errors (if any)
	if prof.ErrorCount > 0 {
		errorStr := fmt.Sprintf("%d in last hour", prof.ErrorCount)
		if prof.ErrorCount >= 3 {
			errorStr = p.styles.StatusBad.Render(errorStr)
		} else {
			errorStr = lipgloss.NewStyle().Foreground(colorYellow).Render(errorStr)
		}
		rows = append(rows, p.renderRow("Errors", errorStr))
	} else {
		rows = append(rows, p.renderRow("Errors", p.styles.StatusOK.Render("None")))
	}

	// Penalty (if any)
	if prof.Penalty > 0 {
		penaltyStr := fmt.Sprintf("%.2f", prof.Penalty)
		rows = append(rows, p.renderRow("Penalty", penaltyStr))
	}

	// Lock status
	if prof.Locked {
		rows = append(rows, p.renderRow("Lock", p.styles.LockIcon.Render("🔒 Locked")))
	}

	// Path (truncate if too long)
	pathDisplay := prof.Path
	maxPathLen := p.width - 16
	if maxPathLen > 0 && len(pathDisplay) > maxPathLen {
		pathDisplay = "~" + pathDisplay[len(pathDisplay)-maxPathLen+1:]
	}
	if pathDisplay != "" {
		rows = append(rows, p.renderRow("Path", pathDisplay))
	}

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

	// Description
	if prof.Description != "" {
		rows = append(rows, p.renderRow("Notes", prof.Description))
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
		{"e", "Edit profile"},
		{"o", "Open in browser"},
		{"d", "Delete profile"},
		{"/", "Search profiles"},
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

// formatDurationFull formats duration for details view.
func formatDurationFull(d time.Duration) string {
	if d < time.Minute {
		return "less than a minute"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%d hours %d minutes", hours, minutes)
}

// renderRow renders a label-value row.
func (p *DetailPanel) renderRow(label, value string) string {
	labelStr := p.styles.Label.Render(label + ":")
	valueStr := p.styles.Value.Render(value)
	return labelStr + " " + valueStr
}
