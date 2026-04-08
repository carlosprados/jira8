package issue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var worklogListCmd = &cobra.Command{
	Use:     "worklog-list ISSUE-KEY",
	Aliases: []string{"wll"},
	Short:   "List worklog entries for an issue",
	Example: "  jira8 issue worklog-list ESA-123",
	Args:    cobra.ExactArgs(1),
	RunE:    runWorklogList,
}

func runWorklogList(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	worklogs, err := a.Client.GetWorklogs(context.Background(), key)
	if err != nil {
		return err
	}

	if a.Output == "json" {
		data, err := json.MarshalIndent(worklogs, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(worklogs) == 0 {
		fmt.Printf("No worklogs found for %s\n", key)
		return nil
	}

	fmt.Printf("Worklogs for %s (%d entries):\n\n", key, len(worklogs))
	for _, wl := range worklogs {
		author := "unknown"
		if wl.Author != nil {
			author = wl.Author.DisplayName
		}
		started := wl.Started
		if len(started) > 10 {
			started = started[:10]
		}
		fmt.Printf("  %s | %s | %s", author, started, wl.TimeSpent)
		if wl.Comment != "" {
			fmt.Printf(" | %s", wl.Comment)
		}
		fmt.Println()
	}
	return nil
}
