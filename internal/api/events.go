package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

const DefaultBaseURL = "https://gliding.net.nz"

// Event represents a gliding event from the /api/events API.
type Event struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Location  string `json:"location"`
	StartDate string `json:"start_date"` // "YYYY-MM-DDTHH:MM:SS.000000Z"
	EndDate   string `json:"end_date"`
	Org       struct {
		Name string `json:"name"`
	} `json:"org"`
}

type eventsResponse struct {
	Success bool    `json:"success"`
	Data    []Event `json:"data"`
}

// FetchEvents retrieves all events from the API.
func FetchEvents(baseURL string, client *http.Client) ([]Event, error) {
	url := baseURL + "/api/events/"
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("events API returned status %d", resp.StatusCode)
	}

	var result eventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding events response: %w", err)
	}

	return result.Data, nil
}

// SelectWindow sorts all events by start date, then returns a window of up to
// pastCount fully-past events followed by up to futureCount current/upcoming
// events, along with the index within that window of the best default selection:
//   - the earliest-starting active event (if any are active now), or
//   - the nearest upcoming event (if no active events), or
//   - the last event in the past portion (if no upcoming events).
func SelectWindow(events []Event, now time.Time, pastCount, futureCount int) ([]Event, int) {
	today := now.Truncate(24 * time.Hour)

	sorted := make([]Event, 0, len(events))
	for _, e := range events {
		if _, err := parseEventDate(e.StartDate); err != nil {
			continue
		}
		if _, err := parseEventDate(e.EndDate); err != nil {
			continue
		}
		sorted = append(sorted, e)
	}
	sort.Slice(sorted, func(i, j int) bool {
		ti, _ := parseEventDate(sorted[i].StartDate)
		tj, _ := parseEventDate(sorted[j].StartDate)
		return ti.Before(tj)
	})

	var past, upcoming []Event
	for _, e := range sorted {
		end, _ := parseEventDate(e.EndDate)
		if end.Truncate(24 * time.Hour).Before(today) {
			past = append(past, e)
		} else {
			upcoming = append(upcoming, e)
		}
	}

	// Take the tail of past and the head of upcoming.
	if len(past) > pastCount {
		past = past[len(past)-pastCount:]
	}
	if len(upcoming) > futureCount {
		upcoming = upcoming[:futureCount]
	}

	window := append(past, upcoming...)
	if len(window) == 0 {
		return window, 0
	}

	pastLen := len(past)

	// Priority 1: earliest-starting active event.
	for i := range window {
		start, _ := parseEventDate(window[i].StartDate)
		end, _ := parseEventDate(window[i].EndDate)
		startDay := start.Truncate(24 * time.Hour)
		endDay := end.Truncate(24 * time.Hour)
		if !startDay.After(today) && !endDay.Before(today) {
			return window, i
		}
	}

	// Priority 2: nearest upcoming event (first in upcoming slice).
	if len(upcoming) > 0 {
		return window, pastLen
	}

	// Priority 3: most recent past event.
	return window, pastLen - 1
}

// parseEventDate handles both "YYYY-MM-DDTHH:MM:SS.000000Z" and "YYYY-MM-DD" formats.
func parseEventDate(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02T15:04:05.000000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q", s)
}

