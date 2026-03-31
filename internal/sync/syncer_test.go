package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/glidingnz/igc-sync/internal/api"
)

func TestLocalPath(t *testing.T) {
	f := api.IgcFile{
		Filename:   "2026-04-01_ZKJ_001.igc",
		FlightDate: "2026-04-01",
	}
	got := LocalPath(f)
	want := filepath.Join("2026-04-01", "2026-04-01_ZKJ_001.igc")
	if got != want {
		t.Errorf("LocalPath = %q, want %q", got, want)
	}
}

func TestDiff_NewFiles(t *testing.T) {
	remote := []api.IgcFile{
		{Filename: "a.igc", FlightDate: "2026-04-01", FileHash: "aaa"},
		{Filename: "b.igc", FlightDate: "2026-04-01", FileHash: "bbb"},
	}
	local := LocalState{}

	result := Diff(remote, local)

	if len(result.New) != 2 {
		t.Errorf("expected 2 new files, got %d", len(result.New))
	}
	if len(result.Updated) != 0 {
		t.Errorf("expected 0 updated files, got %d", len(result.Updated))
	}
}

func TestDiff_UpdatedFile(t *testing.T) {
	remote := []api.IgcFile{
		{Filename: "a.igc", FlightDate: "2026-04-01", FileHash: "newhash"},
	}
	local := LocalState{
		filepath.Join("2026-04-01", "a.igc"): "oldhash",
	}

	result := Diff(remote, local)

	if len(result.New) != 0 {
		t.Errorf("expected 0 new files, got %d", len(result.New))
	}
	if len(result.Updated) != 1 {
		t.Errorf("expected 1 updated file, got %d", len(result.Updated))
	}
	if result.Updated[0].Filename != "a.igc" {
		t.Errorf("unexpected updated filename: %s", result.Updated[0].Filename)
	}
}

func TestDiff_NoChanges(t *testing.T) {
	hash := "abc123"
	remote := []api.IgcFile{
		{Filename: "a.igc", FlightDate: "2026-04-01", FileHash: hash},
	}
	local := LocalState{
		filepath.Join("2026-04-01", "a.igc"): hash,
	}

	result := Diff(remote, local)

	if len(result.New) != 0 || len(result.Updated) != 0 {
		t.Errorf("expected no changes, got new=%d updated=%d", len(result.New), len(result.Updated))
	}
}

func TestDiff_EmptyHashSkipsUpdate(t *testing.T) {
	// If API returns empty file_hash, we should not trigger update (hash unknown).
	remote := []api.IgcFile{
		{Filename: "a.igc", FlightDate: "2026-04-01", FileHash: ""},
	}
	local := LocalState{
		filepath.Join("2026-04-01", "a.igc"): "existinghash",
	}

	result := Diff(remote, local)

	if len(result.Updated) != 0 {
		t.Errorf("empty remote hash should not trigger update, got %d updated", len(result.Updated))
	}
}

func TestDiff_MixedResults(t *testing.T) {
	remote := []api.IgcFile{
		{Filename: "new.igc", FlightDate: "2026-04-01", FileHash: "aaa"},
		{Filename: "same.igc", FlightDate: "2026-04-01", FileHash: "bbb"},
		{Filename: "changed.igc", FlightDate: "2026-04-01", FileHash: "ccc"},
	}
	local := LocalState{
		filepath.Join("2026-04-01", "same.igc"):    "bbb",
		filepath.Join("2026-04-01", "changed.igc"): "old",
	}

	result := Diff(remote, local)

	if len(result.New) != 1 || result.New[0].Filename != "new.igc" {
		t.Errorf("expected 1 new file (new.igc), got %v", result.New)
	}
	if len(result.Updated) != 1 || result.Updated[0].Filename != "changed.igc" {
		t.Errorf("expected 1 updated file (changed.igc), got %v", result.Updated)
	}
}

func TestScanLocal_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	state, err := ScanLocal(dir)
	if err != nil {
		t.Fatalf("ScanLocal error: %v", err)
	}
	if len(state) != 0 {
		t.Errorf("expected empty state, got %v", state)
	}
}

func TestScanLocal_NonExistentDir(t *testing.T) {
	state, err := ScanLocal("/tmp/does-not-exist-xyz-12345")
	if err != nil {
		t.Fatalf("ScanLocal should not error on non-existent dir: %v", err)
	}
	if len(state) != 0 {
		t.Errorf("expected empty state, got %v", state)
	}
}

func TestScanLocal_WithFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a date subdirectory with a file.
	subdir := filepath.Join(dir, "2026-04-01")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	content := []byte("IGC file content here")
	filePath := filepath.Join(subdir, "test.igc")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	state, err := ScanLocal(dir)
	if err != nil {
		t.Fatalf("ScanLocal error: %v", err)
	}

	relPath := filepath.Join("2026-04-01", "test.igc")
	hash, ok := state[relPath]
	if !ok {
		t.Fatalf("expected %q in state, got %v", relPath, state)
	}

	// Verify hash matches manual calculation.
	h := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(h[:])
	if hash != expectedHash {
		t.Errorf("hash mismatch: expected %s, got %s", expectedHash, hash)
	}
}

func TestDownload_Success(t *testing.T) {
	content := []byte("B 123456 5215000N01630000E A 00100 00150\n")
	h := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(h[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := api.IgcFile{
		Filename:   "2026-04-01_ZKJ_001.igc",
		FlightDate: "2026-04-01",
		FileHash:   expectedHash,
		URL:        srv.URL + "/igc/file.igc",
	}

	err := Download(f, dir, srv.Client())
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}

	destPath := filepath.Join(dir, "2026-04-01", "2026-04-01_ZKJ_001.igc")
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("file content mismatch")
	}
}

func TestDownload_HashMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("corrupted content"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := api.IgcFile{
		Filename:   "2026-04-01_ZKJ_001.igc",
		FlightDate: "2026-04-01",
		FileHash:   "0000000000000000000000000000000000000000000000000000000000000000",
		URL:        srv.URL + "/igc/file.igc",
	}

	err := Download(f, dir, srv.Client())
	if err == nil {
		t.Fatal("expected hash mismatch error")
	}

	// Temp file should be cleaned up.
	tmpPath := filepath.Join(dir, "2026-04-01", "2026-04-01_ZKJ_001.igc.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should be removed after hash mismatch")
	}
}

func TestDownload_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := api.IgcFile{
		Filename:   "missing.igc",
		FlightDate: "2026-04-01",
		URL:        srv.URL + "/missing.igc",
	}

	err := Download(f, dir, srv.Client())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestDownload_CreatesSubdirectory(t *testing.T) {
	content := []byte("IGC data")
	h := sha256.Sum256(content)
	hash := hex.EncodeToString(h[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := api.IgcFile{
		Filename:   "file.igc",
		FlightDate: "2026-11-07",
		FileHash:   hash,
		URL:        srv.URL + "/file.igc",
	}

	if err := Download(f, dir, srv.Client()); err != nil {
		t.Fatalf("Download error: %v", err)
	}

	// Verify subdirectory was created.
	subdir := filepath.Join(dir, "2026-11-07")
	if _, err := os.Stat(subdir); os.IsNotExist(err) {
		t.Error("expected subdirectory 2026-11-07 to be created")
	}
}
