package ui

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/glidingnz/igc-sync/internal/api"
	igcsync "github.com/glidingnz/igc-sync/internal/sync"
)

const (
	pollInterval         = 60 * time.Second
	minForceSyncInterval = 5 * time.Second
)

// pollPhase represents what the poller is currently doing.
type pollPhase int

const (
	phaseChecking    pollPhase = iota // fetching file list and computing diff
	phaseDownloading                  // actively downloading files
	phaseCountdown                    // idle; counting down to next automatic check
)

// PollConfig holds configuration for the polling loop.
type PollConfig struct {
	BaseURL   string
	EventID   int
	EventSlug string
	EventName string
	OutputDir string
	Client    *http.Client
	Interval  time.Duration // defaults to pollInterval if zero
}

// ---- Tea messages -------------------------------------------------------

type tickMsg time.Time

type diffDoneMsg struct {
	remote     []api.IgcFile
	toDownload []downloadItem
	err        error
}

// downloadItem is a single queued download with whether it's a new vs updated file.
type downloadItem struct {
	file  api.IgcFile
	isNew bool // false = file exists locally but hash changed
}

type fileDownloadedMsg struct {
	isNew     bool
	err       error
	remaining []downloadItem
}

// ---- Model ---------------------------------------------------------------

type pollModel struct {
	cfg PollConfig

	width  int
	height int

	phase       pollPhase
	remoteTotal int
	localCount  int

	// Current download-cycle progress (reset each sync).
	downloadTotal int
	downloadDone  int
	newCount      int
	updatedCount  int
	errCount      int

	// Preserved after each completed sync for the status line.
	lastSyncAt   time.Time
	lastNewCount int
	lastUpdCount int

	nextSyncAt      time.Time
	lastError       string
	lastForceSyncAt time.Time
}

func newPollModel(cfg PollConfig) pollModel {
	return pollModel{cfg: cfg, phase: phaseChecking}
}

func (m pollModel) interval() time.Duration {
	if m.cfg.Interval > 0 {
		return m.cfg.Interval
	}
	return pollInterval
}

func (m pollModel) Init() tea.Cmd {
	return fetchDiffCmd(m.cfg)
}

func (m pollModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.phase == phaseCountdown &&
				time.Since(m.lastForceSyncAt) >= minForceSyncInterval {
				m.lastForceSyncAt = time.Now()
				m.phase = phaseChecking
				return m, fetchDiffCmd(m.cfg)
			}
		}

	case diffDoneMsg:
		if msg.err != nil {
			m.lastError = msg.err.Error()
			m.nextSyncAt = time.Now().Add(m.interval())
			m.phase = phaseCountdown
			return m, tickCmd()
		}
		m.lastError = ""
		m.remoteTotal = len(msg.remote)
		m.localCount = m.remoteTotal - len(msg.toDownload)
		m.downloadTotal = len(msg.toDownload)
		m.downloadDone = 0
		m.newCount = 0
		m.updatedCount = 0
		m.errCount = 0

		if len(msg.toDownload) == 0 {
			m.lastSyncAt = time.Now()
			m.lastNewCount = 0
			m.lastUpdCount = 0
			m.nextSyncAt = time.Now().Add(m.interval())
			m.phase = phaseCountdown
			return m, tickCmd()
		}
		m.phase = phaseDownloading
		return m, downloadNextCmd(msg.toDownload, m.cfg.OutputDir, m.cfg.Client)

	case fileDownloadedMsg:
		m.downloadDone++
		if msg.err != nil {
			m.errCount++
			m.lastError = fmt.Sprintf("download error: %v", msg.err)
		} else {
			m.localCount++
			if msg.isNew {
				m.newCount++
			} else {
				m.updatedCount++
			}
		}
		if len(msg.remaining) > 0 {
			return m, downloadNextCmd(msg.remaining, m.cfg.OutputDir, m.cfg.Client)
		}
		// All downloads done.
		m.lastSyncAt = time.Now()
		m.lastNewCount = m.newCount
		m.lastUpdCount = m.updatedCount
		m.nextSyncAt = time.Now().Add(m.interval())
		m.phase = phaseCountdown
		return m, tickCmd()

	case tickMsg:
		if m.phase == phaseCountdown {
			if time.Now().After(m.nextSyncAt) {
				m.phase = phaseChecking
				return m, fetchDiffCmd(m.cfg)
			}
			return m, tickCmd()
		}
	}

	return m, nil
}

// ---- Styles -------------------------------------------------------------

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	fileStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	statusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	controlStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func (m pollModel) View() string {
	var b strings.Builder

	// Header
	name := m.cfg.EventName
	if name == "" {
		name = m.cfg.EventSlug
	}
	fmt.Fprintln(&b, titleStyle.Render("igc-sync · "+name))
	fmt.Fprintln(&b, dimStyle.Render("Syncing files to: "+m.cfg.OutputDir))
	fmt.Fprintln(&b)

	// File count — always visible
	fmt.Fprintln(&b, "  "+fileStyle.Render(fmt.Sprintf("Files: %d / %d", m.localCount, m.remoteTotal)))
	fmt.Fprintln(&b)

	// Status line
	switch m.phase {
	case phaseChecking:
		fmt.Fprintln(&b, "  "+statusStyle.Render("Checking for updates..."))
	case phaseDownloading:
		fmt.Fprintln(&b, "  "+statusStyle.Render(
			fmt.Sprintf("↓ Downloading file %d of %d...", m.downloadDone+1, m.downloadTotal)))
	case phaseCountdown:
		secsLeft := int(time.Until(m.nextSyncAt).Seconds())
		if secsLeft < 0 {
			secsLeft = 0
		}
		line := fmt.Sprintf("Next check in %ds", secsLeft)
		if time.Since(m.lastForceSyncAt) >= minForceSyncInterval {
			line += "  ·  press Enter to check now"
		}
		fmt.Fprintln(&b, "  "+statusStyle.Render(line))
	}

	// Last sync info
	if !m.lastSyncAt.IsZero() && m.phase == phaseCountdown {
		info := "Last sync: " + m.lastSyncAt.Format("15:04:05")
		if m.lastNewCount > 0 || m.lastUpdCount > 0 {
			info += fmt.Sprintf("  ·  %d new", m.lastNewCount)
			if m.lastUpdCount > 0 {
				info += fmt.Sprintf(", %d updated", m.lastUpdCount)
			}
		} else {
			info += "  ·  no changes"
		}
		fmt.Fprintln(&b, "  "+dimStyle.Render(info))
	}

	// Error
	if m.lastError != "" {
		fmt.Fprintln(&b, "  "+errorStyle.Render("⚠  "+m.lastError))
	}

	fmt.Fprintln(&b)
	fmt.Fprint(&b, controlStyle.Render("  [q] quit"))

	content := b.String()
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}
	return content
}

// ---- Tea commands --------------------------------------------------------

func fetchDiffCmd(cfg PollConfig) tea.Cmd {
	return func() tea.Msg {
		remote, items, err := fetchAndDiff(cfg)
		return diffDoneMsg{remote: remote, toDownload: items, err: err}
	}
}

func downloadNextCmd(items []downloadItem, outputDir string, client *http.Client) tea.Cmd {
	return func() tea.Msg {
		item := items[0]
		err := igcsync.Download(item.file, outputDir, client)
		return fileDownloadedMsg{isNew: item.isNew, err: err, remaining: items[1:]}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ---- Core sync helpers (also used by runCycle for tests) ----------------

// fetchAndDiff fetches the remote file list and computes what needs downloading.
func fetchAndDiff(cfg PollConfig) (remote []api.IgcFile, items []downloadItem, err error) {
	remote, err = api.FetchIgcFiles(cfg.BaseURL, cfg.EventID, cfg.Client)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching file list: %w", err)
	}

	local, err := igcsync.ScanLocal(cfg.OutputDir)
	if err != nil {
		return nil, nil, fmt.Errorf("scanning local files: %w", err)
	}

	diff := igcsync.Diff(remote, local)

	updatedIDs := make(map[int]bool, len(diff.Updated))
	for _, f := range diff.Updated {
		updatedIDs[f.ID] = true
	}

	for _, f := range diff.New {
		items = append(items, downloadItem{file: f, isNew: true})
	}
	for _, f := range diff.Updated {
		items = append(items, downloadItem{file: f, isNew: false})
	}
	return remote, items, nil
}

// prepareOutputDir creates the output directory if it does not already exist.
func prepareOutputDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// RunPoller starts the interactive status display and polling loop.
// It blocks until the user quits.
func RunPoller(cfg PollConfig) error {
	if err := prepareOutputDir(cfg.OutputDir); err != nil {
		return fmt.Errorf("creating output directory %q: %w", cfg.OutputDir, err)
	}
	m := newPollModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// runCycle performs a synchronous sync cycle. Used by tests.
func runCycle(cfg PollConfig) error {
	_, items, err := fetchAndDiff(cfg)
	if err != nil {
		return err
	}
	for _, item := range items {
		if dlErr := igcsync.Download(item.file, cfg.OutputDir, cfg.Client); dlErr != nil {
			return fmt.Errorf("downloading %s: %w", item.file.Filename, dlErr)
		}
	}
	return nil
}

func containsFile(files []api.IgcFile, target api.IgcFile) bool {
	for _, f := range files {
		if f.ID == target.ID {
			return true
		}
	}
	return false
}


