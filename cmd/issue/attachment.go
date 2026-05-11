package issue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

// attachmentCmd is the parent for attachment subcommands.
// Mirrors the Jira REST surface: upload (POST), list (GET issue), delete (DELETE by id).
// The matching MCP tools are jira_add_attachment, jira_list_attachments,
// jira_delete_attachment — same shape, same arguments.
var attachmentCmd = &cobra.Command{
	Use:     "attachment",
	Aliases: []string{"att"},
	Short:   "Manage issue attachments",
}

var attachmentAddCmd = &cobra.Command{
	Use:     "add ISSUE-KEY FILE [FILE...]",
	Short:   "Upload one or more files to an issue",
	Example: "  jira8 issue attachment add ESA-123 diag.png trace.log",
	Args:    cobra.MinimumNArgs(2),
	RunE:    runAttachmentAdd,
}

var attachmentListCmd = &cobra.Command{
	Use:     "list ISSUE-KEY",
	Aliases: []string{"ls"},
	Short:   "List attachments on an issue",
	Example: "  jira8 issue attachment list ESA-123",
	Args:    cobra.ExactArgs(1),
	RunE:    runAttachmentList,
}

var attachmentDeleteCmd = &cobra.Command{
	Use:     "delete ATTACHMENT-ID",
	Aliases: []string{"rm"},
	Short:   "Delete an attachment by ID",
	Example: "  jira8 issue attachment delete 45821",
	Args:    cobra.ExactArgs(1),
	RunE:    runAttachmentDelete,
}

func init() {
	attachmentCmd.AddCommand(attachmentAddCmd)
	attachmentCmd.AddCommand(attachmentListCmd)
	attachmentCmd.AddCommand(attachmentDeleteCmd)
}

func runAttachmentAdd(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]
	files := args[1:]

	attachments, err := a.Client.AddAttachments(context.Background(), key, files)
	if err != nil {
		return err
	}

	if a.Output == "json" {
		data, err := json.MarshalIndent(attachments, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Uploaded %d attachment(s) to %s:\n", len(attachments), key)
	for _, att := range attachments {
		fmt.Printf("  %s  %s  (%s)\n", labelStyle.Render("#"+att.ID), att.Filename, humanSize(att.Size))
	}
	return nil
}

func runAttachmentList(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	attachments, err := a.Client.ListAttachments(context.Background(), key)
	if err != nil {
		return err
	}

	if a.Output == "json" {
		data, err := json.MarshalIndent(attachments, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(attachments) == 0 {
		fmt.Printf("No attachments on %s\n", key)
		return nil
	}

	fmt.Printf("Attachments on %s (%d):\n", key, len(attachments))
	for _, att := range attachments {
		created := att.Created
		if len(created) > 10 {
			created = created[:10]
		}
		fmt.Printf("  %s  %s  %s  %s  %s\n",
			labelStyle.Render("#"+att.ID),
			att.Filename,
			labelStyle.Render(humanSize(att.Size)),
			userName(att.Author),
			labelStyle.Render(created),
		)
	}
	return nil
}

func runAttachmentDelete(cmd *cobra.Command, args []string) error {
	a := app.Get()
	id := args[0]

	if err := a.Client.DeleteAttachment(context.Background(), id); err != nil {
		return err
	}

	fmt.Printf("Deleted attachment %s\n", id)
	return nil
}
