//go:build live

// Package live runs igc-sync's real request path against production
// gliding.net.nz. Anonymous GETs only.
package live

import (
	"testing"

	"github.com/glidingnz/igc-sync/internal/api"
	"github.com/glidingnz/igc-sync/internal/sync"
)

// Central Plateau Soaring Competition, Taupo, 7–14 Nov 2026: a finished event
// whose IGC files igc-sync synced in production.
const centralPlateau2026 = 334

func TestLive_FetchEvents(t *testing.T) {
	events, err := api.FetchEvents(api.DefaultBaseURL, api.NewClient("live-test"))
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("FetchEvents returned no events")
	}
}

func TestLive_FetchAndDownloadIgcFile(t *testing.T) {
	client := api.NewClient("live-test")

	files, err := api.FetchIgcFiles(api.DefaultBaseURL, centralPlateau2026, client)
	if err != nil {
		t.Fatalf("FetchIgcFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("event %d has no IGC files", centralPlateau2026)
	}

	// Download verifies the SHA-256 against file_hash and errors on mismatch.
	if err := sync.Download(files[0], t.TempDir(), client); err != nil {
		t.Fatalf("Download %s: %v", files[0].Filename, err)
	}
}
