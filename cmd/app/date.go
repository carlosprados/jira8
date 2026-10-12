package app

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// jiraTimeLayout is the only format the worklog "started" field accepts.
const jiraTimeLayout = "2006-01-02T15:04:05.000-0700"

// WorklogStarted normalises a --date value to Jira's worklog format. A bare
// date means 09:00 local time; values without an offset are local time.
func WorklogStarted(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	for _, layout := range []string{jiraTimeLayout, time.RFC3339Nano} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format(jiraTimeLayout), nil
		}
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			if layout == "2006-01-02" {
				t = t.Add(9 * time.Hour)
			}
			return t.Format(jiraTimeLayout), nil
		}
	}
	return "", fmt.Errorf("invalid --date %q: use YYYY-MM-DD, YYYY-MM-DDTHH:MM or RFC 3339", s)
}

var durationRe = regexp.MustCompile(`^\s*(\d+(?:\.\d+)?[wdhm]\s*)+$`)
var durationPartRe = regexp.MustCompile(`\d+(?:\.\d+)?[wdhm]`)

// JiraDuration normalises a duration such as "1h30m" to Jira's "1h 30m": Jira
// rejects the units written together.
func JiraDuration(s string) (string, error) {
	if !durationRe.MatchString(s) {
		return "", fmt.Errorf("invalid duration %q: use units w, d, h, m, e.g. 2h, 1h30m, 3d", s)
	}
	return strings.Join(durationPartRe.FindAllString(s, -1), " "), nil
}
