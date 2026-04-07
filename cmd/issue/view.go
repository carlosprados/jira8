package issue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amplia/jira-cli/cmd/app"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:   "view ISSUE-KEY",
	Short: "View issue details",
	Args:  cobra.ExactArgs(1),
	RunE:  runView,
}

func runView(cmd *cobra.Command, args []string) error {
	issue, err := app.Get().Client.GetIssue(context.Background(), args[0])
	if err != nil {
		return err
	}

	if app.Get().Output == "json" {
		data, err := json.MarshalIndent(issue, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	printIssueDetail(issue)
	return nil
}
