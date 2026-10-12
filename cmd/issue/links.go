package issue

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var linksCmd = &cobra.Command{
	Use:   "links ISSUE-KEY",
	Short: "List the links of an issue",
	Long: "List the links of an issue, read from its side: \"ABC-1 blocks ABC-2\" shows on " +
		"ABC-1 as \"blocks ABC-2\" and on ABC-2 as \"is blocked by ABC-1\". The id is what " +
		"'issue link-delete' takes.",
	Example: "  jira8 issue links ABC-123\n  jira8 issue links ABC-123 -o json",
	Args:    cobra.ExactArgs(1),
	RunE:    runLinks,
}

var linkDeleteCmd = &cobra.Command{
	Use:     "link-delete LINK-ID",
	Short:   "Delete an issue link",
	Example: "  jira8 issue link-delete 55539\n  jira8 issue link-delete 55539 --yes",
	Args:    cobra.ExactArgs(1),
	RunE:    runLinkDelete,
}

func init() {
	linkDeleteCmd.Flags().Bool("yes", false, "Skip confirmation prompt")
}

// LinkRow is a link seen from one of its issues.
type LinkRow struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Direction string `json:"direction"` // outward: this issue is the subject of Phrase
	Phrase    string `json:"phrase"`
	Key       string `json:"key"`
	Summary   string `json:"summary,omitempty"`
	Status    string `json:"status,omitempty"`
}

func linkRow(l models.IssueLink) LinkRow {
	r := LinkRow{ID: l.ID, Type: l.Type.Name, Direction: "outward", Phrase: l.Type.Outward}
	other := l.InwardIssue
	if other == nil {
		r.Direction, r.Phrase, other = "inward", l.Type.Inward, l.OutwardIssue
	}
	if other != nil {
		r.Key = other.Key
		r.Summary = other.Fields.Summary
		r.Status = statusName(other.Fields.Status)
	}
	return r
}

func runLinks(cmd *cobra.Command, args []string) error {
	a := app.Get()
	links, err := a.Client.GetIssueLinks(context.Background(), args[0])
	if err != nil {
		return err
	}
	rows := make([]LinkRow, len(links))
	for i, l := range links {
		rows[i] = linkRow(l)
	}
	if a.Output == "json" {
		return app.OutputJSON(rows)
	}
	if len(rows) == 0 {
		fmt.Printf("%s has no links.\n", args[0])
		return nil
	}
	fmt.Printf("%s  %s  %s  %s  %s\n",
		headerStyle.Render(pad("ID", 8)),
		headerStyle.Render(pad("LINK", 22)),
		headerStyle.Render(pad("ISSUE", 12)),
		headerStyle.Render(pad("STATUS", 15)),
		headerStyle.Render("SUMMARY"),
	)
	for _, r := range rows {
		fmt.Printf("%s  %s  %s  %s  %s\n", pad(r.ID, 8), pad(r.Phrase, 22),
			keyStyle.Render(pad(r.Key, 12)), colorStatus(pad(r.Status, 15)), truncate(r.Summary, 55))
	}
	return nil
}

func runLinkDelete(cmd *cobra.Command, args []string) error {
	a := app.Get()
	id := args[0]
	yes, _ := cmd.Flags().GetBool("yes")

	// Read it first: the prompt and the answer say which link went away, and a
	// wrong id fails before anything is deleted.
	link, err := a.Client.GetIssueLink(context.Background(), id)
	if err != nil {
		return err
	}
	var outward, inward string
	if link.OutwardIssue != nil {
		outward = link.OutwardIssue.Key
	}
	if link.InwardIssue != nil {
		inward = link.InwardIssue.Key
	}
	desc := fmt.Sprintf("%s %s %s", outward, link.Type.Outward, inward)

	if ok, err := app.Confirm(fmt.Sprintf("Delete link %s (%s)?", id, desc), yes); !ok {
		return err
	}
	if err := a.Client.DeleteIssueLink(context.Background(), id); err != nil {
		return err
	}
	return app.Done(map[string]any{"id": id, "outward": outward, "type": link.Type.Name, "inward": inward, "deleted": true},
		"Link %s deleted (%s)", id, desc)
}
