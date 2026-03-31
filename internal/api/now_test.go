package api

import (
	"os"
	"testing"
	"time"
)

func TestNow_RealTime(t *testing.T) {
	os.Unsetenv(NowEnvVar())
	before := time.Now()
	got, err := Now()
	after := time.Now()
	if err != nil {
		t.Fatalf("Now() error: %v", err)
	}
	if got.Before(before) || got.After(after) {
		t.Errorf("Now() = %v, expected between %v and %v", got, before, after)
	}
}

func TestNow_EnvOverride(t *testing.T) {
	t.Setenv(NowEnvVar(), "2026-11-08")
	got, err := Now()
	if err != nil {
		t.Fatalf("Now() error: %v", err)
	}
	if got.Year() != 2026 || got.Month() != 11 || got.Day() != 8 {
		t.Errorf("Now() = %v, expected 2026-11-08", got)
	}
}

func TestNow_InvalidEnv(t *testing.T) {
	t.Setenv(NowEnvVar(), "not-a-date")
	_, err := Now()
	if err == nil {
		t.Fatal("expected error for invalid IGC_SYNC_NOW value")
	}
}

func TestNow_EnvUsedForFiltering(t *testing.T) {
	// Integration: set IGC_SYNC_NOW so that an event starting in "the future"
	// is treated as upcoming from the perspective of the mocked date.
	t.Setenv(NowEnvVar(), "2026-04-01")

	events := []Event{
		{ID: 1, Name: "April Comp", StartDate: "2026-04-05T00:00:00.000000Z", EndDate: "2026-04-10T00:00:00.000000Z"},
		{ID: 2, Name: "Far Away", StartDate: "2027-01-01T00:00:00.000000Z", EndDate: "2027-01-07T00:00:00.000000Z"},
	}

	now, err := Now()
	if err != nil {
		t.Fatalf("Now() error: %v", err)
	}
	window, _ := SelectWindow(events, now, 0, 1) // 0 past, 1 future

	if len(window) != 1 || window[0].ID != 1 {
		t.Errorf("expected only 'April Comp', got %v", window)
	}
}
