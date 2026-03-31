package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glidingnz/igc-sync/internal/api"
)

// makeIgcContent returns fake IGC bytes and their SHA-256 hash.
func makeIgcContent(s string) ([]byte, string) {
	content := []byte(s)
	h := sha256.Sum256(content)
	return content, hex.EncodeToString(h[:])
}

// ---- runCycle integration tests (real HTTP) --------------------------------

func TestRunCycle_DownloadsNewFiles(t *testing.T) {
	content, hash := makeIgcContent("B 120000 5215000N01630000E A 00100 00200\n")

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/events/42/igc-files":
			w.Write([]byte(fmt.Sprintf(`{
				"success": true, "data": [{
					"id": 1, "filename": "641GBE1.igc",
					"flight_date": "2026-04-01", "rego": "GBE",
					"flight_number": 1, "size_bytes": %d,
					"file_hash": "%s",
					"url": "%s/igc/641GBE1.igc"
				}],
				"next_page_url": null, "total": 1
			}`, len(content), hash, srvURL)))
		case "/igc/641GBE1.igc":
			w.Write(content)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	dir := t.TempDir()
	cfg := PollConfig{
		BaseURL:   srv.URL,
		EventID:   42,
		EventSlug: "test-comp",
		OutputDir: dir,
		Client:    srv.Client(),
	}

	if err := runCycle(cfg); err != nil {
		t.Fatalf("runCycle error: %v", err)
	}

	destPath := filepath.Join(dir, "641GBE1.igc")
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("file not downloaded: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("downloaded content mismatch")
	}
}

func TestRunCycle_NoChanges(t *testing.T) {
	content, hash := makeIgcContent("IGC file data")

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/events/1/igc-files" {
			w.Write([]byte(fmt.Sprintf(`{
				"success": true, "data": [{
					"id": 1, "filename": "641GBE1.igc", "flight_date": "2026-04-01",
					"rego": "GBE", "flight_number": 1, "size_bytes": %d,
					"file_hash": "%s", "url": "%s/igc/641GBE1.igc"
				}], "next_page_url": null, "total": 1
			}`, len(content), hash, srvURL)))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "641GBE1.igc"), content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := PollConfig{
		BaseURL:   srv.URL,
		EventID:   1,
		EventSlug: "test-comp",
		OutputDir: dir,
		Client:    srv.Client(),
	}

	if err := runCycle(cfg); err != nil {
		t.Fatalf("runCycle error: %v", err)
	}
}

func TestRunCycle_UpdatesChangedFile(t *testing.T) {
	oldContent := []byte("old content")
	newContent, newHash := makeIgcContent("new content")

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/events/7/igc-files":
			w.Write([]byte(fmt.Sprintf(`{
				"success": true, "data": [{
					"id": 1, "filename": "641GBE1.igc", "flight_date": "2026-04-01",
					"rego": "GBE", "flight_number": 1, "size_bytes": %d,
					"file_hash": "%s", "url": "%s/igc/641GBE1.igc"
				}], "next_page_url": null, "total": 1
			}`, len(newContent), newHash, srvURL)))
		case "/igc/641GBE1.igc":
			w.Write(newContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "641GBE1.igc"), oldContent, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := PollConfig{
		BaseURL:   srv.URL,
		EventID:   7,
		EventSlug: "test-comp",
		OutputDir: dir,
		Client:    srv.Client(),
	}

	if err := runCycle(cfg); err != nil {
		t.Fatalf("runCycle error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "641GBE1.igc"))
	if err != nil {
		t.Fatalf("reading updated file: %v", err)
	}
	if string(got) != string(newContent) {
		t.Errorf("file was not updated: got %q, want %q", got, newContent)
	}
}

// ---- output directory creation tests ----------------------------------------

func TestRunPoller_CreatesOutputDirOnStart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-event-dir")
	// dir does not exist yet
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("expected dir to not exist before test")
	}

	cfg := PollConfig{
		EventSlug: "test-event",
		OutputDir: dir,
		Interval:  10 * time.Second,
	}

	if err := prepareOutputDir(cfg.OutputDir); err != nil {
		t.Fatalf("prepareOutputDir error: %v", err)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected output directory to be created")
	}
}

func TestRunPoller_ExistingDirIsNotAnError(t *testing.T) {
	dir := t.TempDir() // already exists

	cfg := PollConfig{
		EventSlug: "test-event",
		OutputDir: dir,
		Interval:  10 * time.Second,
	}

	if err := prepareOutputDir(cfg.OutputDir); err != nil {
		t.Fatalf("prepareOutputDir should not error on existing dir: %v", err)
	}
}

// ---- pollModel unit tests (no HTTP, no real terminal) ----------------------

func testCfg() PollConfig {
	return PollConfig{
		EventSlug: "test-event",
		OutputDir: "/tmp/test",
		Interval:  10 * time.Second,
	}
}

func sendKey(m tea.Model, key string) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

func sendSpecialKey(m tea.Model, keyType tea.KeyType) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: keyType})
}

func TestPollModel_QuitOnQ(t *testing.T) {
	m := newPollModel(testCfg())
	_, cmd := sendKey(m, "q")
	if cmd == nil {
		t.Fatal("expected a command from 'q' keypress")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestPollModel_QuitOnCtrlC(t *testing.T) {
	m := newPollModel(testCfg())
	_, cmd := sendSpecialKey(m, tea.KeyCtrlC)
	if cmd == nil {
		t.Fatal("expected a command from ctrl+c")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestPollModel_DiffDone_NoFiles_TransitionsToCountdown(t *testing.T) {
	m := newPollModel(testCfg())
	remote := []api.IgcFile{
		{ID: 1, Filename: "a.igc", FileHash: "abc"},
	}
	updated, cmd := m.Update(diffDoneMsg{remote: remote, toDownload: nil})
	m = updated.(pollModel)

	if m.phase != phaseCountdown {
		t.Errorf("expected phaseCountdown, got %v", m.phase)
	}
	if m.remoteTotal != 1 {
		t.Errorf("expected remoteTotal=1, got %d", m.remoteTotal)
	}
	if m.localCount != 1 {
		t.Errorf("expected localCount=1 (no downloads needed), got %d", m.localCount)
	}
	if cmd == nil {
		t.Error("expected tick command to be returned")
	}
}

func TestPollModel_DiffDone_WithFiles_TransitionsToDownloading(t *testing.T) {
	m := newPollModel(testCfg())
	remote := []api.IgcFile{
		{ID: 1, Filename: "a.igc"},
		{ID: 2, Filename: "b.igc"},
	}
	items := []downloadItem{
		{file: remote[0], isNew: true},
		{file: remote[1], isNew: true},
	}
	updated, cmd := m.Update(diffDoneMsg{remote: remote, toDownload: items})
	m = updated.(pollModel)

	if m.phase != phaseDownloading {
		t.Errorf("expected phaseDownloading, got %v", m.phase)
	}
	if m.localCount != 0 {
		t.Errorf("expected localCount=0 (nothing downloaded yet), got %d", m.localCount)
	}
	if m.downloadTotal != 2 {
		t.Errorf("expected downloadTotal=2, got %d", m.downloadTotal)
	}
	if cmd == nil {
		t.Error("expected download command to be returned")
	}
}

func TestPollModel_FileDownloaded_NewFile_IncrementsLocalCount(t *testing.T) {
	m := newPollModel(testCfg())
	m.phase = phaseDownloading
	m.remoteTotal = 3
	m.localCount = 1
	m.downloadTotal = 2

	updated, _ := m.Update(fileDownloadedMsg{isNew: true, err: nil, remaining: nil})
	m = updated.(pollModel)

	if m.localCount != 2 {
		t.Errorf("expected localCount=2 after downloading new file, got %d", m.localCount)
	}
	if m.newCount != 1 {
		t.Errorf("expected newCount=1, got %d", m.newCount)
	}
}

func TestPollModel_FileDownloaded_UpdatedFile_IncrementsLocalCount(t *testing.T) {
	m := newPollModel(testCfg())
	m.phase = phaseDownloading
	m.remoteTotal = 2
	m.localCount = 1 // updated file is already counted in alreadyLocal
	m.downloadTotal = 1

	updated, _ := m.Update(fileDownloadedMsg{isNew: false, err: nil, remaining: nil})
	m = updated.(pollModel)

	if m.localCount != 2 {
		t.Errorf("expected localCount=2 after re-downloading updated file, got %d", m.localCount)
	}
	if m.updatedCount != 1 {
		t.Errorf("expected updatedCount=1, got %d", m.updatedCount)
	}
}

func TestPollModel_AllDownloadsDone_TransitionsToCountdown(t *testing.T) {
	m := newPollModel(testCfg())
	m.phase = phaseDownloading
	m.remoteTotal = 2
	m.localCount = 1
	m.downloadTotal = 1

	updated, cmd := m.Update(fileDownloadedMsg{isNew: true, err: nil, remaining: nil})
	m = updated.(pollModel)

	if m.phase != phaseCountdown {
		t.Errorf("expected phaseCountdown after last download, got %v", m.phase)
	}
	if m.localCount != 2 {
		t.Errorf("expected localCount=2, got %d", m.localCount)
	}
	if m.lastSyncAt.IsZero() {
		t.Error("expected lastSyncAt to be set")
	}
	if cmd == nil {
		t.Error("expected tick command after downloads complete")
	}
}

func TestPollModel_EnterInCountdown_TriggersFetch(t *testing.T) {
	m := newPollModel(testCfg())
	m.phase = phaseCountdown
	m.nextSyncAt = time.Now().Add(30 * time.Second)

	updated, cmd := sendKey(m, "enter")
	m = updated.(pollModel)

	if m.phase != phaseChecking {
		t.Errorf("expected phaseChecking after Enter, got %v", m.phase)
	}
	if cmd == nil {
		t.Error("expected fetchDiff command after Enter")
	}
}

func TestPollModel_EnterInChecking_Ignored(t *testing.T) {
	m := newPollModel(testCfg())
	m.phase = phaseChecking // already checking — Enter should be a no-op

	updated, _ := sendKey(m, "enter")
	m = updated.(pollModel)

	if m.phase != phaseChecking {
		t.Errorf("expected phase to remain phaseChecking, got %v", m.phase)
	}
}

func TestPollModel_EnterRateLimit(t *testing.T) {
	m := newPollModel(testCfg())
	m.phase = phaseCountdown
	m.nextSyncAt = time.Now().Add(30 * time.Second)

	// First Enter: triggers sync.
	updated, cmd1 := sendKey(m, "enter")
	m = updated.(pollModel)
	if cmd1 == nil {
		t.Fatal("first Enter should trigger a command")
	}
	if m.phase != phaseChecking {
		t.Errorf("phase should be checking after first Enter, got %v", m.phase)
	}

	// Simulate the diff coming back so we return to countdown.
	m.phase = phaseCountdown
	m.nextSyncAt = time.Now().Add(30 * time.Second)
	m.lastForceSyncAt = time.Now() // just pressed

	// Second Enter immediately after: rate-limited, no command.
	updated2, cmd2 := sendKey(m, "enter")
	m = updated2.(pollModel)
	if m.phase != phaseCountdown {
		t.Errorf("phase should stay countdown when rate-limited, got %v", m.phase)
	}
	_ = cmd2 // may be nil or a no-op
}

func TestPollModel_DiffError_TransitionsToCountdown(t *testing.T) {
	m := newPollModel(testCfg())
	updated, cmd := m.Update(diffDoneMsg{err: fmt.Errorf("connection refused")})
	m = updated.(pollModel)

	if m.phase != phaseCountdown {
		t.Errorf("expected phaseCountdown on error, got %v", m.phase)
	}
	if m.lastError == "" {
		t.Error("expected lastError to be set")
	}
	if cmd == nil {
		t.Error("expected tick command after error")
	}
}

func TestPollModel_View_ShowsFileCount(t *testing.T) {
	m := newPollModel(testCfg())
	m.localCount = 7
	m.remoteTotal = 10
	m.phase = phaseCountdown
	m.nextSyncAt = time.Now().Add(30 * time.Second)

	view := m.View()

	if !strings.Contains(view, "7") || !strings.Contains(view, "10") {
		t.Errorf("view should contain file counts 7 and 10, got:\n%s", view)
	}
}

func TestPollModel_View_ShowsQuitHint(t *testing.T) {
	m := newPollModel(testCfg())
	view := m.View()
	if !strings.Contains(view, "q") {
		t.Errorf("view should mention 'q' to quit, got:\n%s", view)
	}
}

func TestPollModel_View_ShowsCheckingStatus(t *testing.T) {
	m := newPollModel(testCfg())
	m.phase = phaseChecking
	view := m.View()
	if !strings.Contains(view, "Checking") {
		t.Errorf("view should show Checking status, got:\n%s", view)
	}
}

func TestPollModel_View_ShowsDownloadingStatus(t *testing.T) {
	m := newPollModel(testCfg())
	m.phase = phaseDownloading
	m.downloadDone = 1
	m.downloadTotal = 5
	view := m.View()
	if !strings.Contains(view, "Downloading") {
		t.Errorf("view should show Downloading status, got:\n%s", view)
	}
	if !strings.Contains(view, "2 of 5") {
		t.Errorf("view should show download progress '2 of 5', got:\n%s", view)
	}
}

func TestPollModel_View_ShowsCountdownStatus(t *testing.T) {
	m := newPollModel(testCfg())
	m.phase = phaseCountdown
	m.nextSyncAt = time.Now().Add(45 * time.Second)
	view := m.View()
	if !strings.Contains(view, "Next check") {
		t.Errorf("view should show countdown, got:\n%s", view)
	}
	if !strings.Contains(view, "Enter") {
		t.Errorf("view should mention Enter to force check, got:\n%s", view)
	}
}

