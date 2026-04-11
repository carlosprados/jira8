package issue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var commentEditCmd = &cobra.Command{
	Use:     "comment-edit ISSUE-KEY",
	Aliases: []string{"ce"},
	Short:   "Edit a comment on an issue",
	Example: `  jira8 issue comment-edit ESA-123 --id 84887 --body "Updated text"`,
	Args:    cobra.ExactArgs(1),
	RunE:    runCommentEdit,
}

func init() {
	commentEditCmd.Flags().String("id", "", "Comment ID (required)")
	commentEditCmd.Flags().String("body", "", "Updated comment body (required)")
	_ = commentEditCmd.MarkFlagRequired("id")
	_ = commentEditCmd.MarkFlagRequired("body")
}

func runCommentEdit(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	commentID, _ := cmd.Flags().GetString("id")
	body, _ := cmd.Flags().GetString("body")

	comment, err := a.Client.EditComment(context.Background(), key, commentID, body)
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

	fmt.Printf("Comment %s updated on %s\n", comment.ID, key)
	return nil
}
