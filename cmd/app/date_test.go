package app

import (
	"testing"
	"time"
)

func TestWorklogStarted(t *testing.T) {
	saved := time.Local
	time.Local = time.FixedZone("CEST", 2*3600)
	defer func() { time.Local = saved }()

	cases := map[string]string{
		"":                             "",
		"2026-10-01":                   "2026-10-01T09:00:00.000+0200",
		"2026-10-01T16:30":             "2026-10-01T16:30:00.000+0200",
		"2026-10-01 16:30":             "2026-10-01T16:30:00.000+0200",
		"2026-10-01T16:30:15":          "2026-10-01T16:30:15.000+0200",
		"2026-10-01T14:30:00Z":         "2026-10-01T14:30:00.000+0000",
		"2026-10-01T16:30:00+02:00":    "2026-10-01T16:30:00.000+0200",
		"2026-04-07T09:00:00.000+0200": "2026-04-07T09:00:00.000+0200",
		"2026-04-07T09:00:00.123-0500": "2026-04-07T09:00:00.123-0500",
	}
	for in, want := range cases {
		got, err := WorklogStarted(in)
		if err != nil || got != want {
			t.Errorf("WorklogStarted(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"yesterday", "01/10/2026", "2026-13-01"} {
		if _, err := WorklogStarted(bad); err == nil {
			t.Errorf("WorklogStarted(%q): expected error", bad)
		}
	}
}

func TestJiraDuration(t *testing.T) {
	for in, want := range map[string]string{
		"2h": "2h", "1h30m": "1h 30m", "1h 30m": "1h 30m", "1d4h": "1d 4h", "1.5h": "1.5h", "1w2d3h4m": "1w 2d 3h 4m",
	} {
		if got, err := JiraDuration(in); err != nil || got != want {
			t.Errorf("JiraDuration(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "90", "1 hour", "1x"} {
		if _, err := JiraDuration(bad); err == nil {
			t.Errorf("JiraDuration(%q): expected error", bad)
		}
	}
}
