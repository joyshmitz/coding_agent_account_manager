// Package tui provides the terminal user interface for caam.
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// viewState represents the current view/mode of the TUI.
type viewState int

const (
	stateList viewState = iota
	stateDetail
	stateConfirm
	stateHelp
)

// Profile represents a saved auth profile for display.
type Profile struct {
	Name     string
	Provider string
	IsActive bool
}

// Model is the main Bubble Tea model for the caam TUI.
type Model struct {
	// Provider state
	providers      []string // codex, claude, gemini
	activeProvider int      // Currently selected provider index

	// Profile state
	profiles map[string][]Profile // Profiles by provider
	selected int                  // Currently selected profile index

	// View state
	width  int
	height int
	state  viewState
	err    error

	// UI components
	keys           keyMap
	styles         Styles
	providerPanel  *ProviderPanel
	profilesPanel  *ProfilesPanel

	// Status message
	statusMsg string
}

// DefaultProviders returns the default list of provider names.
func DefaultProviders() []string {
	return []string{"claude", "codex", "gemini"}
}

// New creates a new TUI model with default settings.
func New() Model {
	return NewWithProviders(DefaultProviders())
}

// NewWithProviders creates a new TUI model with the specified providers.
func NewWithProviders(providers []string) Model {
	profilesPanel := NewProfilesPanel()
	if len(providers) > 0 {
		profilesPanel.SetProvider(providers[0])
	}
	return Model{
		providers:      providers,
		activeProvider: 0,
		profiles:       make(map[string][]Profile),
		selected:       0,
		state:          stateList,
		keys:           defaultKeyMap(),
		styles:         DefaultStyles(),
		providerPanel:  NewProviderPanel(providers),
		profilesPanel:  profilesPanel,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.loadProfiles,
	)
}

// loadProfiles loads profiles for all providers.
func (m Model) loadProfiles() tea.Msg {
	profiles := make(map[string][]Profile)

	for _, name := range m.providers {
		// TODO: Load actual profiles from vault
		// For now, return empty list
		profiles[name] = []Profile{}
	}

	return profilesLoadedMsg{profiles: profiles}
}

// profilesLoadedMsg is sent when profiles are loaded.
type profilesLoadedMsg struct {
	profiles map[string][]Profile
}

// errMsg is sent when an error occurs.
type errMsg struct {
	err error
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case profilesLoadedMsg:
		m.profiles = msg.profiles
		// Update provider panel counts
		if m.providerPanel != nil {
			counts := make(map[string]int)
			for provider, profiles := range m.profiles {
				counts[provider] = len(profiles)
			}
			m.providerPanel.SetProfileCounts(counts)
		}
		// Update profiles panel with current provider's profiles
		m.syncProfilesPanel()
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil
	}

	return m, nil
}

// handleKeyPress processes keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		if m.state == stateHelp {
			m.state = stateList
		} else {
			m.state = stateHelp
		}
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.selected > 0 {
			m.selected--
			if m.profilesPanel != nil {
				m.profilesPanel.SetSelected(m.selected)
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		profiles := m.currentProfiles()
		if m.selected < len(profiles)-1 {
			m.selected++
			if m.profilesPanel != nil {
				m.profilesPanel.SetSelected(m.selected)
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.Left):
		if m.activeProvider > 0 {
			m.activeProvider--
			m.selected = 0
			m.syncProfilesPanel()
		}
		return m, nil

	case key.Matches(msg, m.keys.Right):
		if m.activeProvider < len(m.providers)-1 {
			m.activeProvider++
			m.selected = 0
			m.syncProfilesPanel()
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		// TODO: Activate selected profile
		m.statusMsg = "Profile activation not yet implemented"
		return m, nil

	case key.Matches(msg, m.keys.Tab):
		// Cycle through providers
		m.activeProvider = (m.activeProvider + 1) % len(m.providers)
		m.selected = 0
		m.syncProfilesPanel()
		return m, nil
	}

	return m, nil
}

// currentProfiles returns the profiles for the currently selected provider.
func (m Model) currentProfiles() []Profile {
	if m.activeProvider >= 0 && m.activeProvider < len(m.providers) {
		return m.profiles[m.providers[m.activeProvider]]
	}
	return nil
}

// currentProvider returns the name of the currently selected provider.
func (m Model) currentProvider() string {
	if m.activeProvider >= 0 && m.activeProvider < len(m.providers) {
		return m.providers[m.activeProvider]
	}
	return ""
}

// updateProviderCounts updates the provider panel with current profile counts.
func (m *Model) updateProviderCounts() {
	counts := make(map[string]int)
	for provider, profiles := range m.profiles {
		counts[provider] = len(profiles)
	}
	m.providerPanel.SetProfileCounts(counts)
}

// syncProviderPanel syncs the provider panel state with the model.
func (m *Model) syncProviderPanel() {
	m.providerPanel.SetActiveProvider(m.activeProvider)
}

// syncProfilesPanel syncs the profiles panel with the current provider's profiles.
func (m Model) syncProfilesPanel() {
	if m.profilesPanel == nil {
		return
	}
	provider := m.currentProvider()
	m.profilesPanel.SetProvider(provider)

	// Convert Profile to ProfileInfo
	profiles := m.profiles[provider]
	infos := make([]ProfileInfo, len(profiles))
	for i, p := range profiles {
		infos[i] = ProfileInfo{
			Name:     p.Name,
			AuthMode: "oauth", // Default, TODO: get from actual profile
			LoggedIn: true,    // TODO: get actual status
			Locked:   false,   // TODO: get actual lock status
			IsActive: p.IsActive,
		}
	}
	m.profilesPanel.SetProfiles(infos)
	m.profilesPanel.SetSelected(m.selected)
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	switch m.state {
	case stateHelp:
		return m.helpView()
	default:
		return m.mainView()
	}
}

// mainView renders the main list view.
func (m Model) mainView() string {
	// Header
	header := m.styles.Header.Render("caam - Coding Agent Account Manager")

	// Calculate panel dimensions
	providerPanelWidth := 20
	profilesPanelWidth := m.width - providerPanelWidth - 6 // Account for borders and spacing
	if profilesPanelWidth < 40 {
		profilesPanelWidth = 40
	}
	contentHeight := m.height - 5 // Header + status bar

	// Sync and render provider panel
	m.providerPanel.SetActiveProvider(m.activeProvider)
	m.providerPanel.SetSize(providerPanelWidth, contentHeight)
	providerPanelView := m.providerPanel.View()

	// Sync and render profiles panel (center panel)
	var profilesPanelView string
	if m.profilesPanel != nil {
		m.profilesPanel.SetSize(profilesPanelWidth, contentHeight)
		profilesPanelView = m.profilesPanel.View()
	} else {
		profilesPanelView = m.renderProfileList()
	}

	// Create panels side by side
	panels := lipgloss.JoinHorizontal(
		lipgloss.Top,
		providerPanelView,
		"  ", // Spacing
		profilesPanelView,
	)

	// Status bar
	status := m.renderStatusBar()

	// Combine header, panels, and status
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		panels,
	)

	// Add status bar at bottom
	availableHeight := m.height - lipgloss.Height(content) - 2
	if availableHeight > 0 {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			lipgloss.NewStyle().Height(availableHeight).Render(""),
			status,
		)
	}

	return content
}

// renderProviderTabs renders the provider selection tabs.
func (m Model) renderProviderTabs() string {
	var tabs []string
	for i, p := range m.providers {
		style := m.styles.Tab
		if i == m.activeProvider {
			style = m.styles.ActiveTab
		}
		tabs = append(tabs, style.Render(p))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// renderProfileList renders the list of profiles for the current provider.
func (m Model) renderProfileList() string {
	profiles := m.currentProfiles()
	if len(profiles) == 0 {
		return m.styles.Empty.Render(fmt.Sprintf("No profiles saved for %s\n\nUse 'caam backup %s <email>' to save a profile",
			m.currentProvider(), m.currentProvider()))
	}

	var items []string
	for i, p := range profiles {
		style := m.styles.Item
		if i == m.selected {
			style = m.styles.SelectedItem
		}

		indicator := "  "
		if p.IsActive {
			indicator = m.styles.Active.Render("● ")
		}

		items = append(items, style.Render(indicator+p.Name))
	}

	return lipgloss.JoinVertical(lipgloss.Left, items...)
}

// renderStatusBar renders the bottom status bar.
func (m Model) renderStatusBar() string {
	left := m.styles.StatusKey.Render("q") + m.styles.StatusText.Render(" quit  ")
	left += m.styles.StatusKey.Render("?") + m.styles.StatusText.Render(" help  ")
	left += m.styles.StatusKey.Render("tab") + m.styles.StatusText.Render(" switch provider  ")
	left += m.styles.StatusKey.Render("enter") + m.styles.StatusText.Render(" activate")

	if m.statusMsg != "" {
		left = m.styles.StatusText.Render(m.statusMsg)
	}

	return m.styles.StatusBar.Width(m.width).Render(left)
}

// helpView renders the help screen.
func (m Model) helpView() string {
	help := `
Keyboard Shortcuts
==================

Navigation
  ↑/k     Move up
  ↓/j     Move down
  ←/h     Previous provider
  →/l     Next provider
  tab     Cycle providers

Actions
  enter   Activate selected profile
  b       Backup current auth
  d       Delete profile

General
  ?       Toggle help
  q/esc   Quit

Press any key to return...
`
	return m.styles.Help.Render(help)
}

// Run starts the TUI application.
func Run() error {
	p := tea.NewProgram(New(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
