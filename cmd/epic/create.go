package epic

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an Epic",
	Example: "  jira8 epic create --name \"Q2 Refactor\" --summary \"Refactor billing pipeline\"\n" +
		"  jira8 epic create --name \"Billing v2\" --summary \"Billing v2\" --labels billing --field \"Fix Version/s=2.4\"",
	RunE: runCreate,
}

func init() {
	createCmd.Flags().String("name", "", "Epic Name (required, shown on the Agile board)")
	createCmd.Flags().String("summary", "", "Epic summary (required)")
	createCmd.Flags().String("project", "", "Project key (default from config)")
	createCmd.Flags().String("description", "", "Epic description")
	createCmd.Flags().String("description-file", "", "Read description from file (use - for stdin)")
	createCmd.Flags().String("assignee", "", "Assignee username (use 'me' for current user)")
	createCmd.Flags().String("priority", "", "Priority name")
	createCmd.Flags().Bool("markdown", false, "Treat --description as Markdown and convert it to Jira Wiki Markup; local images (![alt](path)) are attached to the issue")

	app.AddFieldFlags(createCmd)

	_ = createCmd.MarkFlagRequired("name")
	_ = createCmd.MarkFlagRequired("summary")
}

func runCreate(cmd *cobra.Command, args []string) error {
	a := app.Get()

	name, _ := cmd.Flags().GetString("name")
	summary, _ := cmd.Flags().GetString("summary")
	project, err := app.ProjectKey(cmd)
	if err != nil {
		return err
	}
	body, err := app.ReadBody(cmd, "description", "description-file")
	if err != nil {
		return err
	}
	description := body.Text
	assignee, _ := cmd.Flags().GetString("assignee")
	priority, _ := cmd.Flags().GetString("priority")

	epicNameID, _, err := a.EpicFieldIDs(context.Background())
	if err != nil {
		return err
	}

	extra, err := app.CreateFields(cmd, project, "Epic")
	if err != nil {
		return err
	}
	extra[epicNameID] = name

	req := &models.CreateIssueRequest{
		Fields: models.CreateIssueFields{
			Project:     models.ProjectRef{Key: project},
			Summary:     summary,
			IssueType:   models.TypeRef{Name: "Epic"},
			Description: description,
			Extra:       extra,
		},
	}

	if assignee != "" {
		username, err := a.Client.ResolveAssignee(context.Background(), assignee)
		if err != nil {
			return err
		}
		req.Fields.Assignee = &models.UserRef{Name: username}
	}

	if priority != "" {
		req.Fields.Priority = &models.PriorityRef{Name: priority}
	}

	resp, err := a.Client.CreateIssue(context.Background(), req)
	if err != nil {
		return err
	}

	uploaded, err := app.UploadImages(context.Background(), resp.Key, body.Images)
	if err != nil {
		return fmt.Errorf("created %s but failed to upload its images: %w", resp.Key, err)
	}

	if a.Output == "json" {
		return app.OutputJSON(struct {
			*models.CreateIssueResponse
			Attachments []models.Attachment `json:"attachments,omitempty"`
		}{resp, uploaded})
	}

	fmt.Printf("Created Epic %s  %s\n", resp.Key, resp.URL)
	app.PrintUploadedImages(uploaded)
	return nil
}
