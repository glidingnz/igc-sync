package ui

import (
	"testing"

	"github.com/glidingnz/igc-sync/internal/api"
)

func TestEventItem_Title(t *testing.T) {
	item := eventItem{event: api.Event{Name: "Grand Prix Matamata"}}
	if item.Title() != "Grand Prix Matamata" {
		t.Errorf("unexpected title: %s", item.Title())
	}
}

func TestEventItem_Description(t *testing.T) {
	item := eventItem{event: api.Event{
		Name:      "Test Comp",
		StartDate: "2026-04-01T00:00:00.000000Z",
		EndDate:   "2026-04-07T00:00:00.000000Z",
		Location:  "Taupo",
		Org:       struct{ Name string `json:"name"` }{Name: "Taupo Gliding Club"},
	}}

	desc := item.Description()
	if desc == "" {
		t.Error("description should not be empty")
	}
	// Should contain org, location, and dates.
	for _, want := range []string{"Taupo Gliding Club", "Taupo", "2026-04-01", "2026-04-07"} {
		found := false
		if len(desc) > 0 {
			for _, part := range []string{desc} {
				if len(part) >= len(want) {
					found = containsStr(part, want)
					if found {
						break
					}
				}
			}
		}
		if !found {
			t.Errorf("description %q should contain %q", desc, want)
		}
	}
}

func TestEventItem_FilterValue(t *testing.T) {
	item := eventItem{event: api.Event{Name: "Canterbury Regional"}}
	if item.FilterValue() != "Canterbury Regional" {
		t.Errorf("unexpected filter value: %s", item.FilterValue())
	}
}

func TestFormatDate(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"2026-04-01T00:00:00.000000Z", "2026-04-01"},
		{"2026-04-01T00:00:00Z", "2026-04-01"},
		{"2026-04-01", "2026-04-01"},
		{"", ""},
	}
	for _, tc := range cases {
		got := formatDate(tc.input)
		if got != tc.want {
			t.Errorf("formatDate(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		len(s) > 0 && func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
