package client

import (
	"testing"
	"time"
)

func TestAgingCard(t *testing.T) {
	// Columns as in flowCols, plus status 5 in the In Progress column (1).
	cols := map[string]int{"1": 0, "2": 1, "5": 1, "3": 2, "4": 3}
	now := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)

	// Starts on day 3, moves to Review on 5, back to In Progress on 8, then
	// to status 5 (same column) on 10: in column since 8, age since 3.
	is := flowIssue(at(1),
		[3]string{at(3), "1", "2"},
		[3]string{at(5), "2", "3"},
		[3]string{at(8), "3", "2"},
		[3]string{at(10), "2", "5"},
	)
	c, ok := agingCard(is, cols, 1, 1, now)
	if !ok {
		t.Fatal("agingCard: ok = false")
	}
	if c.InColumnDays != 12 {
		t.Errorf("InColumnDays = %v, want 12 (since day 8)", c.InColumnDays)
	}
	if c.AgeDays == nil || *c.AgeDays != 17 {
		t.Errorf("AgeDays = %v, want 17 (since day 3)", c.AgeDays)
	}

	// Still in the to-do column: no age.
	c, _ = agingCard(flowIssue(at(1)), cols, 0, 1, now)
	if c.AgeDays != nil || c.InColumnDays != 19 {
		t.Errorf("to-do card: AgeDays %v, InColumnDays %v; want nil, 19", c.AgeDays, c.InColumnDays)
	}
}
