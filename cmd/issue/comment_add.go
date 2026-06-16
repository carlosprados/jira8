package issue

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/markup"
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
	commentAddCmd.Flags().String("body", "", "Comment body (required, or use --body-file)")
	commentAddCmd.Flags().String("body-file", "", "Read body from file (use - for stdin)")
	commentAddCmd.Flags().Bool("markdown", false, "Treat --body as Markdown and convert to Jira Wiki Markup before sending")
}

func runCommentAdd(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	body, set, err := app.ReadTextInput(cmd, "body", "body-file")
	if err != nil {
		return err
	}
	if !set || body == "" {
		return fmt.Errorf("comment body is required (use --body or --body-file)")
	}
	if md, _ := cmd.Flags().GetBool("markdown"); md {
		body = markup.MarkdownToWiki(body)
	}

	comment, err := a.Client.AddComment(context.Background(), key, &models.AddCommentRequest{
		Body: body,
	})
	if err != nil {
		return err
	}

	if a.Output == "json" {
		return app.OutputJSON(comment)
	}

	fmt.Printf("Comment added to %s by %s\n", key, userName(comment.Author))
	return nil
}
