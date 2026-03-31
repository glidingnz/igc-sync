package api

import (
	"fmt"
	"os"
	"time"
)

// NowEnvVar is the environment variable that overrides the current time for
// manual testing. Set it to a date in "YYYY-MM-DD" format, e.g.:
//
//	IGC_SYNC_NOW=2026-11-08 igc-sync
func NowEnvVar() string { return "IGC_SYNC_NOW" }

// Now returns the current time, or the time specified by the IGC_SYNC_NOW
// environment variable if it is set. Useful for manual testing against future
// or past events without changing the system clock.
func Now() (time.Time, error) {
	if raw := os.Getenv(NowEnvVar()); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return time.Time{}, fmt.Errorf(
				"invalid %s value %q: expected YYYY-MM-DD format", NowEnvVar(), raw,
			)
		}
		return t, nil
	}
	return time.Now(), nil
}
