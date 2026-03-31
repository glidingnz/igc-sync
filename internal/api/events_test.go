package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchEvents(t *testing.T) {
	body := `{
		"success": true,
		"data": [
			{"id": 1, "name": "Test Comp", "slug": "test-comp", "location": "Taupo",
			 "start_date": "2026-04-01T00:00:00.000000Z", "end_date": "2026-04-07T00:00:00.000000Z",
			 "org": {"name": "Taupo Gliding Club"}},
			{"id": 2, "name": "Far Future Comp", "slug": "far-future", "location": "Omarama",
			 "start_date": "2027-01-01T00:00:00.000000Z", "end_date": "2027-01-07T00:00:00.000000Z",
			 "org": {"name": "Omarama"}}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/events/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// No timerange query param expected anymore.
		if r.URL.Query().Get("timerange") != "" {
			t.Errorf("unexpected timerange query param: %s", r.URL.Query().Get("timerange"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	events, err := FetchEvents(srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("FetchEvents error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Name != "Test Comp" {
		t.Errorf("expected 'Test Comp', got %q", events[0].Name)
	}
	if events[0].Org.Name != "Taupo Gliding Club" {
		t.Errorf("expected org name 'Taupo Gliding Club', got %q", events[0].Org.Name)
	}
}

func TestFetchEvents_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := FetchEvents(srv.URL, srv.Client())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestFetchEvents_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := FetchEvents(srv.URL, srv.Client())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFetchEvents_Empty(t *testing.T) {
	body := `{"success": true, "data": []}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	events, err := FetchEvents(srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestEvent_JSONFields(t *testing.T) {
	sample := `{
		"id": 334,
		"name": "Central Plateau Soaring Competition",
		"slug": "central-plateau-soaring-competition-oct-2026",
		"location": "Taupo",
		"start_date": "2026-11-07T00:00:00.000000Z",
		"end_date": "2026-11-14T00:00:00.000000Z",
		"org": {"name": "Taupo Gliding Club"}
	}`
	var e Event
	if err := json.Unmarshal([]byte(sample), &e); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if e.ID != 334 {
		t.Errorf("expected ID 334, got %d", e.ID)
	}
	if e.Slug != "central-plateau-soaring-competition-oct-2026" {
		t.Errorf("unexpected slug: %s", e.Slug)
	}
}

func TestParseEventDate(t *testing.T) {
	cases := []struct {
		input    string
		wantYear int
	}{
		{"2026-04-01T00:00:00.000000Z", 2026},
		{"2026-04-01T00:00:00Z", 2026},
		{"2026-04-01", 2026},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseEventDate(tc.input)
			if err != nil {
				t.Fatalf("parseEventDate(%q) error: %v", tc.input, err)
			}
			if got.Year() != tc.wantYear {
				t.Errorf("expected year %d, got %d", tc.wantYear, got.Year())
			}
		})
	}
}

// now = 2026-04-05 (mid-comp)
var selectWindowNow = time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

// helpers to build test events
func makeEvent(id int, name, start, end string) Event {
	return Event{ID: id, Name: name, StartDate: start, EndDate: end}
}

func TestSelectWindow_NormalCase(t *testing.T) {
	// 3 past, 1 active, 3 future — should return last 2 past + active + next 2 future (window 2,2)
	events := []Event{
		makeEvent(1, "Past1", "2026-01-01", "2026-01-07"),
		makeEvent(2, "Past2", "2026-02-01", "2026-02-07"),
		makeEvent(3, "Past3", "2026-03-01", "2026-03-07"),
		makeEvent(4, "Active", "2026-04-01", "2026-04-10"), // active on Apr 5
		makeEvent(5, "Next1", "2026-05-01", "2026-05-07"),
		makeEvent(6, "Next2", "2026-06-01", "2026-06-07"),
		makeEvent(7, "Next3", "2026-07-01", "2026-07-07"),
	}

	window, cursor := SelectWindow(events, selectWindowNow, 2, 2)

	if len(window) != 4 {
		t.Fatalf("expected 4 events in window, got %d: %v", len(window), window)
	}
	if window[0].ID != 2 {
		t.Errorf("expected Past2 first, got %s (id=%d)", window[0].Name, window[0].ID)
	}
	if window[1].ID != 3 {
		t.Errorf("expected Past3 second, got %s", window[1].Name)
	}
	if window[2].ID != 4 {
		t.Errorf("expected Active third, got %s", window[2].Name)
	}
	if cursor != 2 {
		t.Errorf("expected cursor on Active (index 2), got %d", cursor)
	}
}

func TestSelectWindow_CursorOnEarliestActiveWhenMultiple(t *testing.T) {
	// Two active events: cursor should be on the one that started earlier.
	events := []Event{
		makeEvent(1, "Active Early", "2026-04-01", "2026-04-10"),
		makeEvent(2, "Active Late", "2026-04-03", "2026-04-12"),
		makeEvent(3, "Future", "2026-05-01", "2026-05-07"),
	}

	window, cursor := SelectWindow(events, selectWindowNow, 5, 10)

	if len(window) != 3 {
		t.Fatalf("expected 3 events, got %d", len(window))
	}
	if cursor != 0 {
		t.Errorf("expected cursor on earliest active (index 0), got %d (%s)", cursor, window[cursor].Name)
	}
	if window[0].ID != 1 {
		t.Errorf("expected 'Active Early' at index 0, got %s", window[0].Name)
	}
}

func TestSelectWindow_CursorOnNearestFutureWhenNoActive(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	events := []Event{
		makeEvent(1, "Past1", "2026-02-01", "2026-02-07"),
		makeEvent(2, "Past2", "2026-03-01", "2026-03-07"),
		makeEvent(3, "Upcoming", "2026-04-10", "2026-04-17"),
		makeEvent(4, "FarFuture", "2026-06-01", "2026-06-07"),
	}

	window, cursor := SelectWindow(events, now, 5, 10)

	if len(window) != 4 {
		t.Fatalf("expected 4 events, got %d", len(window))
	}
	if window[cursor].ID != 3 {
		t.Errorf("expected cursor on 'Upcoming' (id=3), got %s (id=%d)", window[cursor].Name, window[cursor].ID)
	}
}

func TestSelectWindow_CursorOnLastPastWhenNoUpcoming(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	events := []Event{
		makeEvent(1, "Past1", "2026-01-01", "2026-01-07"),
		makeEvent(2, "Past2", "2026-02-01", "2026-02-07"),
	}

	window, cursor := SelectWindow(events, now, 5, 10)

	if len(window) != 2 {
		t.Fatalf("expected 2 events, got %d", len(window))
	}
	if cursor != 1 {
		t.Errorf("expected cursor on last past event (index 1), got %d", cursor)
	}
}

func TestSelectWindow_SortedByStartDate(t *testing.T) {
	// Deliberately out of order.
	events := []Event{
		makeEvent(3, "C", "2026-06-01", "2026-06-07"),
		makeEvent(1, "A", "2026-02-01", "2026-02-07"),
		makeEvent(2, "B", "2026-04-01", "2026-04-07"),
	}

	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	window, _ := SelectWindow(events, now, 5, 10)

	if len(window) != 3 {
		t.Fatalf("expected 3 events, got %d", len(window))
	}
	if window[0].ID != 1 || window[1].ID != 2 || window[2].ID != 3 {
		t.Errorf("events not sorted by start date: got IDs %d %d %d", window[0].ID, window[1].ID, window[2].ID)
	}
}

func TestSelectWindow_PastCountLimitApplied(t *testing.T) {
	events := []Event{
		makeEvent(1, "OldPast", "2026-01-01", "2026-01-07"),
		makeEvent(2, "RecentPast", "2026-03-01", "2026-03-07"),
		makeEvent(3, "Future", "2026-05-01", "2026-05-07"),
	}

	now := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	window, _ := SelectWindow(events, now, 1, 10) // only 1 past

	if len(window) != 2 {
		t.Fatalf("expected 2 events (1 past + 1 future), got %d", len(window))
	}
	if window[0].ID != 2 {
		t.Errorf("expected only 'RecentPast' (id=2) in past, got id=%d", window[0].ID)
	}
}

func TestSelectWindow_FutureCountLimitApplied(t *testing.T) {
	events := []Event{
		makeEvent(1, "Next1", "2026-05-01", "2026-05-07"),
		makeEvent(2, "Next2", "2026-06-01", "2026-06-07"),
		makeEvent(3, "Next3", "2026-07-01", "2026-07-07"),
	}

	now := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	window, _ := SelectWindow(events, now, 5, 2) // only 2 future

	if len(window) != 2 {
		t.Fatalf("expected 2 events, got %d", len(window))
	}
	if window[0].ID != 1 || window[1].ID != 2 {
		t.Errorf("expected first 2 future events, got IDs %d %d", window[0].ID, window[1].ID)
	}
}

func TestSelectWindow_Empty(t *testing.T) {
	window, cursor := SelectWindow(nil, time.Now(), 5, 10)
	if len(window) != 0 {
		t.Errorf("expected empty window, got %v", window)
	}
	if cursor != 0 {
		t.Errorf("expected cursor 0 for empty window, got %d", cursor)
	}
}

func TestSelectWindow_InvalidDatesSkipped(t *testing.T) {
	events := []Event{
		makeEvent(1, "Bad", "not-a-date", "also-bad"),
		makeEvent(2, "Good", "2026-05-01", "2026-05-07"),
	}
	now := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	window, _ := SelectWindow(events, now, 5, 10)
	if len(window) != 1 || window[0].ID != 2 {
		t.Errorf("expected only valid event in window, got %v", window)
	}
}

