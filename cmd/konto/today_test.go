package main

import (
	"testing"
	"time"
)

func TestTodayFollowsTheEnvironmentOverrideElseTheClock(t *testing.T) {
	east := time.FixedZone("UTC+2", 2*60*60)
	clock := func() time.Time { return time.Date(2030, 1, 2, 0, 30, 0, 0, east) } // still Jan 1st in UTC
	env := func(value string) func(string) string { return func(string) string { return value } }

	for name, tc := range map[string]struct {
		value string
		want  time.Time
	}{
		"override":   {"2026-09-30", time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)},
		"unset":      {"", time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC)},
		"not a date": {"tomorrow", time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC)},
	} {
		if got := todayFunc(env(tc.value), clock)(); !got.Equal(tc.want) || got.Location() != time.UTC {
			t.Errorf("%s: today = %v, want %v", name, got, tc.want)
		}
	}
}

func TestDownloadDirIsTheXDGOneElseDownloadsInHome(t *testing.T) {
	env := func(value string) func(string) string {
		return func(key string) string {
			if key == "XDG_DOWNLOAD_DIR" {
				return value
			}

			return ""
		}
	}

	if got := downloadDir(env("/data/dl"), "/home/me"); got != "/data/dl" {
		t.Errorf("downloadDir = %q, want the XDG one", got)
	}
	for _, unset := range []string{"", "  "} {
		if got := downloadDir(env(unset), "/home/me"); got != "/home/me/Downloads" {
			t.Errorf("downloadDir(%q) = %q, want ~/Downloads", unset, got)
		}
	}
}
