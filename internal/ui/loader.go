package ui

import (
"fmt"
"net/http"

"github.com/charmbracelet/bubbles/spinner"
tea "github.com/charmbracelet/bubbletea"
"github.com/charmbracelet/lipgloss"

"github.com/glidingnz/igc-sync/internal/api"
)

type eventsFetchedMsg struct {
events []api.Event
err    error
}

type loaderModel struct {
spinner spinner.Model
width   int
height  int
baseURL string
client  *http.Client
// result is populated when the fetch completes.
events []api.Event
err    error
done   bool
}

func newLoaderModel(baseURL string, client *http.Client) loaderModel {
s := spinner.New()
s.Spinner = spinner.Dot
s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
return loaderModel{spinner: s, baseURL: baseURL, client: client}
}

func (m loaderModel) Init() tea.Cmd {
return tea.Batch(
m.spinner.Tick,
fetchEventsCmd(m.baseURL, m.client),
)
}

func (m loaderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
switch msg := msg.(type) {
case tea.KeyMsg:
if msg.String() == "ctrl+c" || msg.String() == "q" {
m.done = true
return m, tea.Quit
}
case tea.WindowSizeMsg:
m.width = msg.Width
m.height = msg.Height
case eventsFetchedMsg:
m.events = msg.events
m.err = msg.err
m.done = true
return m, tea.Quit
}
var cmd tea.Cmd
m.spinner, cmd = m.spinner.Update(msg)
return m, cmd
}

func (m loaderModel) View() string {
if m.done {
return ""
}
content := m.spinner.View() + "  Fetching events from gliding.net.nz..."
if m.width > 0 && m.height > 0 {
return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
return content
}

func fetchEventsCmd(baseURL string, client *http.Client) tea.Cmd {
return func() tea.Msg {
events, err := api.FetchEvents(baseURL, client)
return eventsFetchedMsg{events: events, err: err}
}
}

// FetchEventsWithUI fetches events while showing a centered loading spinner.
// It blocks until the fetch completes or the user quits (q/ctrl+c).
// Returns nil events (no error) if the user quit before the fetch completed.
func FetchEventsWithUI(baseURL string, client *http.Client) ([]api.Event, error) {
m := newLoaderModel(baseURL, client)
p := tea.NewProgram(m, tea.WithAltScreen())
final, err := p.Run()
if err != nil {
return nil, fmt.Errorf("loading screen error: %w", err)
}
result := final.(loaderModel)
if result.err != nil {
return nil, result.err
}
return result.events, nil
}
