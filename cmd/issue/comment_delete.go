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

var commentDeleteCmd = &cobra.Command{
	Use:     "comment-delete ISSUE-KEY",
	Aliases: []string{"cd"},
	Short:   "Delete a comment from an issue",
	Example: `  jira8 issue comment-delete ESA-123 --id 84887
  jira8 issue comment-delete ESA-123 --id 84887 --yes`,
	Args: cobra.ExactArgs(1),
	RunE: runCommentDelete,
}

func init() {
	commentDeleteCmd.Flags().String("id", "", "Comment ID (required)")
	commentDeleteCmd.Flags().Bool("yes", false, "Skip confirmation prompt")
	_ = commentDeleteCmd.MarkFlagRequired("id")
}

func runCommentDelete(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	commentID, _ := cmd.Flags().GetString("id")
	yes, _ := cmd.Flags().GetBool("yes")

	if !yes {
		fmt.Printf("Delete comment %s from %s? [y/N] ", commentID, key)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := a.Client.DeleteComment(context.Background(), key, commentID); err != nil {
		return err
	}

	fmt.Printf("Comment %s deleted from %s\n", commentID, key)
	return nil
}
