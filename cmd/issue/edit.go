package issue

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/markup"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:     "edit ISSUE-KEY",
	Short:   "Edit an issue",
	Example: "  jira8 issue edit ESA-123 --summary \"New title\" --assignee me",
	Args:    cobra.ExactArgs(1),
	RunE:    runEdit,
}

func init() {
	editCmd.Flags().String("summary", "", "New summary")
	editCmd.Flags().String("description", "", "New description")
	editCmd.Flags().String("description-file", "", "Read new description from file (use - for stdin)")
	editCmd.Flags().String("assignee", "", "New assignee (use 'me' for current user, empty to unassign)")
	editCmd.Flags().String("priority", "", "New priority")
	editCmd.Flags().String("epic-name", "", "New Epic Name (only valid on Epic issues)")
	editCmd.Flags().String("epic-link", "", "Epic key to associate this issue with (empty to detach)")
	editCmd.Flags().Bool("markdown", false, "Treat --description as Markdown and convert to Jira Wiki Markup before sending")
	editCmd.Flags().StringArray("attach", nil, "Attach a file to the issue (repeatable). Can be used alone, without other field edits.")
}

func runEdit(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	fields := make(map[string]any)

	if cmd.Flags().Changed("summary") {
		v, _ := cmd.Flags().GetString("summary")
		fields["summary"] = v
	}

	if v, set, err := app.ReadTextInput(cmd, "description", "description-file"); err != nil {
		return err
	} else if set {
		if md, _ := cmd.Flags().GetBool("markdown"); md {
			v = markup.MarkdownToWiki(v)
		}
		fields["description"] = v
	}

	if cmd.Flags().Changed("assignee") {
		v, _ := cmd.Flags().GetString("assignee")
		if v == "" {
			fields["assignee"] = nil
		} else {
			username, err := a.Client.ResolveAssignee(context.Background(), v)
			if err != nil {
				return err
			}
			fields["assignee"] = models.UserRef{Name: username}
		}
	}

	if cmd.Flags().Changed("priority") {
		v, _ := cmd.Flags().GetString("priority")
		fields["priority"] = models.PriorityRef{Name: v}
	}

	if cmd.Flags().Changed("epic-name") || cmd.Flags().Changed("epic-link") {
		epicNameID, epicLinkID, err := a.EpicFieldIDs(context.Background())
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("epic-name") {
			v, _ := cmd.Flags().GetString("epic-name")
			fields[epicNameID] = v
		}
		if cmd.Flags().Changed("epic-link") {
			v, _ := cmd.Flags().GetString("epic-link")
			if v == "" {
				fields[epicLinkID] = nil
			} else {
				fields[epicLinkID] = v
			}
		}
	}

	attachFiles, _ := cmd.Flags().GetStringArray("attach")

	if len(fields) == 0 && len(attachFiles) == 0 {
		return fmt.Errorf("no fields to update; use --summary, --description, --assignee, --priority, --epic-name, --epic-link, or --attach")
	}

	if len(fields) > 0 {
		req := &models.EditIssueRequest{Fields: fields}
		if err := a.Client.EditIssue(context.Background(), key, req); err != nil {
			return err
		}
		fmt.Printf("Updated %s\n", key)
	}

	if len(attachFiles) > 0 {
		uploaded, err := a.Client.AddAttachments(context.Background(), key, attachFiles)
		if err != nil {
			return fmt.Errorf("attaching files to %s: %w", key, err)
		}
		fmt.Printf("Uploaded %d attachment(s) to %s:\n", len(uploaded), key)
		for _, att := range uploaded {
			fmt.Printf("  %s  %s  (%s)\n", labelStyle.Render("#"+att.ID), att.Filename, humanSize(att.Size))
		}
	}

	return nil
}
