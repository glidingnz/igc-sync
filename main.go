package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/glidingnz/igc-sync/internal/api"
	"github.com/glidingnz/igc-sync/internal/ui"
)

const baseURL = api.DefaultBaseURL

func main() {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 1. Fetch all events (shows a loading spinner TUI).
	events, err := ui.FetchEventsWithUI(baseURL, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching events: %v\n", err)
		os.Exit(1)
	}
	if events == nil {
		// User quit during loading.
		os.Exit(0)
	}

	now, err := api.Now()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 2. Build a window of last 5 past + next 10 current/upcoming events.
	window, cursorIndex := api.SelectWindow(events, now, 5, 10)
	if len(window) == 0 {
		fmt.Println("No events found.")
		os.Exit(0)
	}

	// 3. Present interactive event selector with cursor on the closest event.
	selected, err := ui.SelectEvent(window, cursorIndex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error selecting event: %v\n", err)
		os.Exit(1)
	}
	if selected == nil {
		fmt.Println("No event selected. Exiting.")
		os.Exit(0)
	}

	// 4. Derive output directory: {cwd}/{event-slug}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}
	outputDir := filepath.Join(cwd, selected.Slug)

	// 5. Start polling loop.
	cfg := ui.PollConfig{
		BaseURL:   baseURL,
		EventID:   selected.ID,
		EventSlug: selected.Slug,
		EventName: selected.Name,
		OutputDir: outputDir,
		Client:    client,
	}
	if err := ui.RunPoller(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
