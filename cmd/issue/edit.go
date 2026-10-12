package issue

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit ISSUE-KEY",
	Short: "Edit an issue",
	Example: `  jira8 issue edit ABC-123 --summary "New title" --assignee me
  jira8 issue edit ABC-123 --due-date 2026-09-24 --original-estimate 5d
  jira8 issue edit ABC-123 --due-date ""              # clear the due date
  jira8 issue edit ABC-123 --add-label urgent --remove-label triage
  jira8 issue edit ABC-123 --field "Story Points=8" --field "Fix Version/s=2.4,2.5"`,
	Args: cobra.ExactArgs(1),
	RunE: runEdit,
}

func init() {
	editCmd.Flags().String("summary", "", "New summary")
	editCmd.Flags().String("description", "", "New description")
	editCmd.Flags().String("description-file", "", "Read new description from file (use - for stdin)")
	editCmd.Flags().String("assignee", "", "New assignee (use 'me' for current user, empty to unassign)")
	editCmd.Flags().String("priority", "", "New priority")
	editCmd.Flags().String("epic-name", "", "New Epic Name (only valid on Epic issues)")
	editCmd.Flags().String("epic-link", "", "Epic key to associate this issue with (empty to detach)")
	editCmd.Flags().String("due-date", "", "Due date, format YYYY-MM-DD (empty to clear)")
	editCmd.Flags().String("original-estimate", "", "Original time estimate, e.g. 3d, 5h, 1h30m")
	editCmd.Flags().Bool("markdown", false, "Treat --description as Markdown and convert it to Jira Wiki Markup; local images (![alt](path)) are attached to the issue")
	editCmd.Flags().StringArray("attach", nil, "Attach a file to the issue (repeatable). Can be used alone, without other field edits.")
	app.AddFieldFlags(editCmd)
	app.AddLabelEditFlags(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	fields := make(map[string]any)

	if cmd.Flags().Changed("summary") {
		v, _ := cmd.Flags().GetString("summary")
		fields["summary"] = v
	}

	body, err := app.ReadBody(cmd, "description", "description-file")
	if err != nil {
		return err
	}
	if body.Set {
		fields["description"] = body.Text
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

	if cmd.Flags().Changed("due-date") {
		v, _ := cmd.Flags().GetString("due-date")
		if v == "" {
			fields["duedate"] = nil
		} else {
			fields["duedate"] = v
		}
	}

	if cmd.Flags().Changed("original-estimate") {
		v, _ := cmd.Flags().GetString("original-estimate")
		if v != "" {
			var err error
			if v, err = app.JiraDuration(v); err != nil {
				return err
			}
		}
		fields["timetracking"] = models.TimeTracking{OriginalEstimate: v}
	}

	update, err := app.EditFields(cmd, key, fields)
	if err != nil {
		return err
	}

	attachFiles, _ := cmd.Flags().GetStringArray("attach")

	if len(fields) == 0 && len(update) == 0 && len(attachFiles) == 0 {
		return fmt.Errorf("no fields to update; use --summary, --description, --assignee, --priority, --epic-name, --epic-link, --due-date, --original-estimate, --labels, --add-label, --remove-label, --field or --attach")
	}

	// Images first, so the new description renders them as soon as it lands.
	uploaded, err := app.UploadImages(context.Background(), key, body.Images)
	if err != nil {
		return err
	}

	if len(fields) > 0 || len(update) > 0 {
		req := &models.EditIssueRequest{Fields: fields, Update: update}
		if err := a.Client.EditIssue(context.Background(), key, req); err != nil {
			return err
		}
	}

	if len(attachFiles) > 0 {
		attached, err := a.Client.AddAttachments(context.Background(), key, attachFiles)
		if err != nil {
			return fmt.Errorf("attaching files to %s: %w", key, err)
		}
		uploaded = append(uploaded, attached...)
	}

	if a.Output == "json" {
		return app.OutputJSON(struct {
			Key         string              `json:"key"`
			Updated     []string            `json:"updated"`
			Attachments []models.Attachment `json:"attachments,omitempty"`
		}{key, app.UpdatedFields(fields, update), uploaded})
	}
	if len(fields) > 0 || len(update) > 0 {
		fmt.Printf("Updated %s\n", key)
	}
	if len(uploaded) > 0 {
		fmt.Printf("Uploaded %d attachment(s) to %s:\n", len(uploaded), key)
		for _, att := range uploaded {
			fmt.Printf("  %s  %s  (%s)\n", labelStyle.Render("#"+att.ID), att.Filename, humanSize(att.Size))
		}
	}
	return nil
}
