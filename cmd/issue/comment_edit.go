package issue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/markup"
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
	commentEditCmd.Flags().String("body", "", "Updated comment body (required, or use --body-file)")
	commentEditCmd.Flags().String("body-file", "", "Read updated body from file (use - for stdin)")
	commentEditCmd.Flags().Bool("markdown", false, "Treat --body as Markdown and convert to Jira Wiki Markup before sending")
	_ = commentEditCmd.MarkFlagRequired("id")
}

func runCommentEdit(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	commentID, _ := cmd.Flags().GetString("id")
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
