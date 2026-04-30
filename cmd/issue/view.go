package issue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:     "view ISSUE-KEY",
	Short:   "View issue details",
	Example: "  jira8 issue view ESA-123",
	Args:    cobra.ExactArgs(1),
	RunE:    runView,
}

func runView(cmd *cobra.Command, args []string) error {
	a := app.Get()
	issue, err := a.Client.GetIssue(context.Background(), args[0])
	if err != nil {
		return err
	}

	if a.Output == "json" {
		data, err := json.MarshalIndent(issue, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Resolve Epic custom field IDs best-effort so view can render Epic Name /
	// Epic Link. A failure here is non-fatal — we still render the rest.
	epicNameID, epicLinkID, _ := a.EpicFieldIDs(context.Background())
	printIssueDetailWithEpic(issue, epicNameID, epicLinkID)
	return nil
}
