package client

import (
	"testing"
	"time"

	"github.com/amplia/jira8/internal/models"
)

// Columns: 0 To Do (1), 1 In Progress (2), 2 Review (3), 3 Done (4). Status 9
// is not on the board.
var flowCols = map[string]int{"1": 0, "2": 1, "3": 2, "4": 3}

func flowIssue(created string, moves ...[3]string) models.Issue {
	is := models.Issue{Key: "ABC-1", Fields: models.IssueFields{Created: created, Status: &models.Status{ID: "1"}}}
	cl := &models.Changelog{}
	for _, m := range moves {
		cl.Histories = append(cl.Histories, models.History{Created: m[0], Items: []models.HistoryItem{{Field: "status", From: m[1], To: m[2]}}})
	}
	is.Changelog = cl
	return is
}

func at(day int) string {
	return time.Date(2026, 9, day, 10, 0, 0, 0, time.UTC).Format(jiraTime)
}

func TestIssueFlow(t *testing.T) {
	// To Do → In Progress (3) → off board (5) → Review (6) → Done (8) →
	// reopened to In Progress (9) → Done (10).
	is := flowIssue(at(1),
		[3]string{at(3), "1", "2"},
		[3]string{at(5), "2", "9"},
		[3]string{at(6), "9", "3"},
		[3]string{at(8), "3", "4"},
		[3]string{at(9), "4", "2"},
		[3]string{at(10), "2", "4"},
	)
	f, ok := issueFlow(is, flowCols, 1, 3)
	if !ok || f.started == nil {
		t.Fatalf("issueFlow = %+v, %v", f, ok)
	}
	if got := f.started.UTC().Day(); got != 3 {
		t.Errorf("started day %d, want 3", got)
	}
	if got := f.done.UTC().Day(); got != 10 {
		t.Errorf("done day %d, want 10 (last entry into done)", got)
	}
	want := []time.Duration{0, 3 * 24 * time.Hour, 2 * 24 * time.Hour, 24 * time.Hour}
	for i, d := range want {
		if f.inColumn[i] != d {
			t.Errorf("column %d: %v, want %v", i, f.inColumn[i], d)
		}
	}
}

func TestIssueFlowSkippedStart(t *testing.T) {
	f, ok := issueFlow(flowIssue(at(1), [3]string{at(2), "1", "4"}), flowCols, 1, 3)
	if !ok || f.started != nil {
		t.Fatalf("straight to done: started %v, ok %v; want nil, true", f.started, ok)
	}
	if _, ok := issueFlow(flowIssue(at(1), [3]string{at(2), "1", "2"}), flowCols, 1, 3); ok {
		t.Error("never done: ok = true")
	}
}

func TestIssueFlowCreatedInProgress(t *testing.T) {
	is := flowIssue(at(1), [3]string{at(4), "2", "4"})
	f, ok := issueFlow(is, flowCols, 1, 3)
	if !ok || f.started == nil || f.started.UTC().Day() != 1 {
		t.Fatalf("created in progress: started %v, ok %v; want day 1", f.started, ok)
	}
}

func TestDistribution(t *testing.T) {
	d := distribution([]float64{10, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	if d.Count != 10 || d.P50 != 5 || d.P85 != 9 || d.Max != 10 {
		t.Errorf("distribution = %+v", d)
	}
	if d := distribution(nil); d.Count != 0 {
		t.Errorf("empty distribution = %+v", d)
	}
}

func TestMonday(t *testing.T) {
	for _, tc := range []struct {
		day  int
		want string
	}{{14, "2026-09-14"}, {13, "2026-09-07"}, {20, "2026-09-14"}} {
		if got := monday(time.Date(2026, 9, tc.day, 12, 0, 0, 0, time.UTC)); got != tc.want {
			t.Errorf("monday(Sep %d) = %s, want %s", tc.day, got, tc.want)
		}
	}
}
