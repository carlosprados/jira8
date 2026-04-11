package issue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var commentAddCmd = &cobra.Command{
	Use:     "comment-add ISSUE-KEY",
	Aliases: []string{"ca"},
	Short:   "Add a comment to an issue",
	Example: `  jira8 issue comment-add ESA-123 --body "Deployed to staging"`,
	Args:    cobra.ExactArgs(1),
	RunE:    runCommentAdd,
}

func init() {
	commentAddCmd.Flags().String("body", "", "Comment body (required)")
	_ = commentAddCmd.MarkFlagRequired("body")
}

func runCommentAdd(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	body, _ := cmd.Flags().GetString("body")

	comment, err := a.Client.AddComment(context.Background(), key, &models.AddCommentRequest{
		Body: body,
	})
	if err != nil {
		return err
	}

	if a.Output == "json" {
		data, err := json.MarshalIndent(comment, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Comment added to %s by %s\n", key, userName(comment.Author))
	return nil
}
