package issue

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var commentDeleteCmd = &cobra.Command{
	Use:     "comment-delete ISSUE-KEY",
	Aliases: []string{"cd"},
	Short:   "Delete a comment from an issue",
	Example: `  jira8 issue comment-delete ABC-123 --id 84887
  jira8 issue comment-delete ABC-123 --id 84887 --yes`,
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

	if ok, err := app.Confirm(fmt.Sprintf("Delete comment %s from %s?", commentID, key), yes); !ok {
		return err
	}

	if err := a.Client.DeleteComment(context.Background(), key, commentID); err != nil {
		return err
	}

	return app.Done(map[string]any{"key": key, "id": commentID, "deleted": true},
		"Comment %s deleted from %s", commentID, key)
}
