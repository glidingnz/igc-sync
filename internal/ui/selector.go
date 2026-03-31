package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"

	"github.com/glidingnz/igc-sync/internal/api"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

// eventItem wraps an api.Event for the bubbles list component.
type eventItem struct {
	event api.Event
}

func (i eventItem) Title() string {
	return i.event.Name
}

func (i eventItem) Description() string {
	start := formatDate(i.event.StartDate)
	end := formatDate(i.event.EndDate)
	parts := []string{}
	if i.event.Org.Name != "" {
		parts = append(parts, i.event.Org.Name)
	}
	if i.event.Location != "" {
		parts = append(parts, i.event.Location)
	}
	parts = append(parts, fmt.Sprintf("%s → %s", start, end))
	return strings.Join(parts, " · ")
}

func (i eventItem) FilterValue() string {
	return i.event.Name
}

// selectorModel is the bubbletea model for event selection.
type selectorModel struct {
	list     list.Model
	selected *api.Event
	quitting bool
}

func (m selectorModel) Init() tea.Cmd {
	return nil
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(eventItem); ok {
				event := item.event
				m.selected = &event
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m selectorModel) View() string {
	if m.quitting {
		return ""
	}
	return docStyle.Render(m.list.View())
}

// SelectEvent presents an interactive arrow-key list of events and returns
// the one the user selects. cursorIndex sets the initial selection.
// Returns nil if the user quits without selecting.
func SelectEvent(events []api.Event, cursorIndex int) (*api.Event, error) {
	items := make([]list.Item, len(events))
	for i, e := range events {
		items[i] = eventItem{event: e}
	}

	l := list.New(items, list.NewDefaultDelegate(), 80, 20)
	l.Title = "Select an event to sync IGC files"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	if cursorIndex > 0 && cursorIndex < len(items) {
		l.Select(cursorIndex)
	}

	m := selectorModel{list: l}
	p := tea.NewProgram(m, tea.WithAltScreen())

	final, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("running event selector: %w", err)
	}

	result := final.(selectorModel)
	if result.quitting || result.selected == nil {
		return nil, nil
	}
	return result.selected, nil
}

// formatDate trims the timestamp portion from "YYYY-MM-DDTHH:MM:SS.000000Z" → "YYYY-MM-DD".
func formatDate(s string) string {
	if idx := strings.IndexByte(s, 'T'); idx > 0 {
		return s[:idx]
	}
	return s
}
