package issue

import "github.com/spf13/cobra"

// IssueCmd is the parent command for all issue operations.
var IssueCmd = &cobra.Command{
	Use:     "issue",
	Aliases: []string{"i"},
	Short:   "Work with issues",
}

func init() {
	IssueCmd.AddCommand(listCmd)
	IssueCmd.AddCommand(viewCmd)
	IssueCmd.AddCommand(createCmd)
	IssueCmd.AddCommand(editCmd)
	IssueCmd.AddCommand(transitionCmd)
	IssueCmd.AddCommand(transitionsCmd)
	IssueCmd.AddCommand(worklogAddCmd)
	IssueCmd.AddCommand(worklogListCmd)
	IssueCmd.AddCommand(commentAddCmd)
	IssueCmd.AddCommand(commentListCmd)
	IssueCmd.AddCommand(commentEditCmd)
	IssueCmd.AddCommand(commentDeleteCmd)
}
