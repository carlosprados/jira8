package issue

import (
	"testing"

	"github.com/amplia/jira8/internal/models"
)

// TestLinkRow checks that a link reads from the side of the issue that holds
// it: Jira sets only the other issue, inwardIssue when this one is outward.
func TestLinkRow(t *testing.T) {
	blocks := models.IssueLinkType{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"}
	other := &models.Issue{Key: "ABC-2", Fields: models.IssueFields{Summary: "s", Status: &models.Status{Name: "Done"}}}

	got := linkRow(models.IssueLink{ID: "1", Type: blocks, InwardIssue: other})
	want := LinkRow{ID: "1", Type: "Blocks", Direction: "outward", Phrase: "blocks", Key: "ABC-2", Summary: "s", Status: "Done"}
	if got != want {
		t.Errorf("outward: got %+v, want %+v", got, want)
	}

	got = linkRow(models.IssueLink{ID: "1", Type: blocks, OutwardIssue: other})
	want.Direction, want.Phrase = "inward", "is blocked by"
	if got != want {
		t.Errorf("inward: got %+v, want %+v", got, want)
	}
}
