package issue

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var commentEditCmd = &cobra.Command{
	Use:     "comment-edit ISSUE-KEY",
	Aliases: []string{"ce"},
	Short:   "Edit a comment on an issue",
	Example: `  jira8 issue comment-edit ABC-123 --id 84887 --body "Updated text"`,
	Args:    cobra.ExactArgs(1),
	RunE:    runCommentEdit,
}

func init() {
	commentEditCmd.Flags().String("id", "", "Comment ID (required)")
	commentEditCmd.Flags().String("body", "", "Updated comment body (required, or use --body-file)")
	commentEditCmd.Flags().String("body-file", "", "Read updated body from file (use - for stdin)")
	commentEditCmd.Flags().Bool("markdown", false, "Treat --body as Markdown and convert it to Jira Wiki Markup; local images (![alt](path)) are attached to the issue")
	_ = commentEditCmd.MarkFlagRequired("id")
}

func runCommentEdit(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	commentID, _ := cmd.Flags().GetString("id")
	body, err := app.ReadBody(cmd, "body", "body-file")
	if err != nil {
		return err
	}
	if !body.Set || body.Text == "" {
		return fmt.Errorf("comment body is required (use --body or --body-file)")
	}
	uploaded, err := app.UploadImages(context.Background(), key, body.Images)
	if err != nil {
		return err
	}

	comment, err := a.Client.EditComment(context.Background(), key, commentID, body.Text)
	if err != nil {
		return err
	}

	if a.Output == "json" {
		return app.OutputJSON(comment)
	}

	fmt.Printf("Comment %s updated on %s\n", comment.ID, key)
	app.PrintUploadedImages(uploaded)
	return nil
}
