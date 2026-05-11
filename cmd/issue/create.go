package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/markup"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:     "create",
	Short:   "Create an issue",
	Example: "  jira8 issue create --summary \"Fix login\" --type Bug --priority High",
	RunE:    runCreate,
}

func init() {
	createCmd.Flags().String("summary", "", "Issue summary (required)")
	createCmd.Flags().String("type", "Task", "Issue type")
	createCmd.Flags().String("project", "", "Project key (default from config)")
	createCmd.Flags().String("description", "", "Issue description")
	createCmd.Flags().String("description-file", "", "Read description from file (use - for stdin)")
	createCmd.Flags().String("assignee", "", "Assignee username (use 'me' for current user)")
	createCmd.Flags().String("priority", "", "Issue priority")
	createCmd.Flags().String("parent", "", "Parent issue key for Sub-task creation (e.g. ESA-65)")
	createCmd.Flags().String("epic-name", "", "Epic Name (required when --type Epic)")
	createCmd.Flags().String("epic-link", "", "Epic key to associate this issue with (e.g. ESA-42)")
	createCmd.Flags().Bool("markdown", false, "Treat --description as Markdown and convert to Jira Wiki Markup before sending")
	createCmd.Flags().StringArray("attach", nil, "Attach a file (repeatable, e.g. --attach a.png --attach b.log)")

	_ = createCmd.MarkFlagRequired("summary")
}

func runCreate(cmd *cobra.Command, args []string) error {
	a := app.Get()

	summary, _ := cmd.Flags().GetString("summary")
	issueType, _ := cmd.Flags().GetString("type")
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = a.Config.Project
	}
	description, _, err := app.ReadTextInput(cmd, "description", "description-file")
	if err != nil {
		return err
	}
	if md, _ := cmd.Flags().GetBool("markdown"); md {
		description = markup.MarkdownToWiki(description)
	}
	assignee, _ := cmd.Flags().GetString("assignee")
	priority, _ := cmd.Flags().GetString("priority")
	parent, _ := cmd.Flags().GetString("parent")
	epicName, _ := cmd.Flags().GetString("epic-name")
	epicLink, _ := cmd.Flags().GetString("epic-link")

	isEpic := strings.EqualFold(issueType, "Epic")
	if isEpic && epicName == "" {
		return fmt.Errorf("--epic-name is required when --type Epic")
	}
	if epicName != "" && !isEpic {
		return fmt.Errorf("--epic-name only applies when --type Epic")
	}

	req := &models.CreateIssueRequest{
		Fields: models.CreateIssueFields{
			Project:     models.ProjectRef{Key: project},
			Summary:     summary,
			IssueType:   models.TypeRef{Name: issueType},
			Description: description,
		},
	}

	if assignee != "" {
		username := assignee
		if strings.EqualFold(assignee, "me") {
			user, err := a.Client.GetMyself(context.Background())
			if err != nil {
				return fmt.Errorf("resolving current user: %w", err)
			}
			username = user.Name
		}
		req.Fields.Assignee = &models.UserRef{Name: username}
	}

	if priority != "" {
		req.Fields.Priority = &models.PriorityRef{Name: priority}
	}

	if parent != "" {
		req.Fields.Parent = &models.IssueKeyRef{Key: parent}
	}

	if epicName != "" || epicLink != "" {
		epicNameID, epicLinkID, err := a.EpicFieldIDs(context.Background())
		if err != nil {
			return err
		}
		req.Fields.Extra = map[string]any{}
		if epicName != "" {
			req.Fields.Extra[epicNameID] = epicName
		}
		if epicLink != "" {
			req.Fields.Extra[epicLinkID] = epicLink
		}
	}

	resp, err := a.Client.CreateIssue(context.Background(), req)
	if err != nil {
		return err
	}

	// Attachments are uploaded after creation because Jira's /issue endpoint
	// does not accept multipart bodies. The two calls are not transactional:
	// if the upload fails the issue already exists, so we report both the
	// created key and the upload error.
	attachFiles, _ := cmd.Flags().GetStringArray("attach")
	var uploaded []models.Attachment
	if len(attachFiles) > 0 {
		uploaded, err = a.Client.AddAttachments(context.Background(), resp.Key, attachFiles)
		if err != nil {
			return fmt.Errorf("created %s but failed to upload attachments: %w", resp.Key, err)
		}
	}

	if a.Output == "json" {
		payload := struct {
			*models.CreateIssueResponse
			Attachments []models.Attachment `json:"attachments,omitempty"`
		}{CreateIssueResponse: resp, Attachments: uploaded}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Created %s\n", resp.Key)
	if len(uploaded) > 0 {
		fmt.Printf("Uploaded %d attachment(s):\n", len(uploaded))
		for _, att := range uploaded {
			fmt.Printf("  %s  %s  (%s)\n", labelStyle.Render("#"+att.ID), att.Filename, humanSize(att.Size))
		}
	}
	return nil
}
