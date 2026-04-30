package issue

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var worklogDeleteCmd = &cobra.Command{
	Use:     "worklog-delete ISSUE-KEY",
	Aliases: []string{"wld"},
	Short:   "Delete a worklog entry from an issue",
	Example: `  jira8 issue worklog-delete ESA-123 --id 27705
  jira8 issue worklog-delete ESA-123 --id 27705 --yes`,
	Args: cobra.ExactArgs(1),
	RunE: runWorklogDelete,
}

func init() {
	worklogDeleteCmd.Flags().String("id", "", "Worklog ID (required)")
	worklogDeleteCmd.Flags().Bool("yes", false, "Skip confirmation prompt")
	_ = worklogDeleteCmd.MarkFlagRequired("id")
}

func runWorklogDelete(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	worklogID, _ := cmd.Flags().GetString("id")
	yes, _ := cmd.Flags().GetBool("yes")

	if !yes {
		fmt.Printf("Delete worklog %s from %s? [y/N] ", worklogID, key)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := a.Client.DeleteWorklog(context.Background(), key, worklogID); err != nil {
		return err
	}

	fmt.Printf("Worklog %s deleted from %s\n", worklogID, key)
	return nil
}
