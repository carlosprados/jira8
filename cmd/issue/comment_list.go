package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var commentListCmd = &cobra.Command{
	Use:     "comment-list ISSUE-KEY",
	Aliases: []string{"cl"},
	Short:   "List comments on an issue",
	Example: "  jira8 issue comment-list ESA-123",
	Args:    cobra.ExactArgs(1),
	RunE:    runCommentList,
}

func init() {
	commentListCmd.Flags().Bool("markdown", false, "Convert comment bodies from Jira Wiki Markup to Markdown")
}

func runCommentList(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	comments, err := a.Client.GetComments(context.Background(), key)
	if err != nil {
		return err
	}

	if md, _ := cmd.Flags().GetBool("markdown"); md {
		app.RenderCommentsAsMarkdown(comments)
	}

	if a.Output == "json" {
		return app.OutputJSON(comments)
	}

	if len(comments) == 0 {
		fmt.Printf("No comments on %s\n", key)
		return nil
	}

	fmt.Printf("Comments on %s (%d):\n", key, len(comments))
	for _, c := range comments {
		created := c.Created
		if len(created) > 10 {
			created = created[:10]
		}
		fmt.Printf("\n  %s  %s  %s\n", labelStyle.Render("#"+c.ID), headerStyle.Render(userName(c.Author)), labelStyle.Render(created))
		for _, line := range strings.Split(c.Body, "\n") {
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Println()
	return nil
}
