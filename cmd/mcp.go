package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/client"
	"github.com/amplia/jira8/internal/models"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server commands",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server (stdio transport)",
	RunE:  runMCPServe,
}

func init() {
	mcpCmd.AddCommand(mcpServeCmd)
}

func runMCPServe(cmd *cobra.Command, args []string) error {
	a := app.Get()

	s := server.NewMCPServer(
		"jira",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, true),
		server.WithPromptCapabilities(true),
	)

	jc := a.Client

	// Register Resources and Prompts (see cmd/mcp_resources.go, cmd/mcp_prompts.go).
	// Tools below are still registered for clients that don't support those
	// primitives (e.g. LM Studio supports tools only).
	registerResources(s, jc)
	registerPrompts(s, jc, a.Config.Project)

	s.AddTool(
		mcp.NewTool("jira_list_issues",
			mcp.WithDescription("List Jira issues using filters or JQL"),
			mcp.WithString("project", mcp.Description("Project key (e.g. ESA)")),
			mcp.WithString("status", mcp.Description("Filter by status name")),
			mcp.WithString("assignee", mcp.Description("Filter by assignee username, use 'me' for current user")),
			mcp.WithString("type", mcp.Description("Filter by issue type (e.g. Epic, Story, Bug)")),
			mcp.WithString("epic", mcp.Description("Filter issues linked to this Epic key (e.g. ESA-42)")),
			mcp.WithString("jql", mcp.Description("Raw JQL query (overrides other filters)")),
			mcp.WithNumber("max", mcp.Description("Maximum number of results (default 50)")),
		),
		listIssuesHandler(jc, a.Config.Project),
	)

	s.AddTool(
		mcp.NewTool("jira_get_issue",
			mcp.WithDescription("Get detailed information about a Jira issue"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Issue key (e.g. ESA-123)")),
		),
		getIssueHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_create_issue",
			mcp.WithDescription("Create a new Jira issue"),
			mcp.WithString("project", mcp.Required(), mcp.Description("Project key")),
			mcp.WithString("summary", mcp.Required(), mcp.Description("Issue summary")),
			mcp.WithString("type", mcp.Required(), mcp.Description("Issue type (e.g. Bug, Task, Story, Epic)")),
			mcp.WithString("description", mcp.Description("Issue description")),
			mcp.WithString("assignee", mcp.Description("Assignee username")),
			mcp.WithString("priority", mcp.Description("Priority name")),
			mcp.WithString("parent", mcp.Description("Parent issue key for Sub-task creation (e.g. ESA-65)")),
			mcp.WithString("epic_name", mcp.Description("Epic Name (required when type is Epic)")),
			mcp.WithString("epic_link", mcp.Description("Epic key to associate this issue with (e.g. ESA-42)")),
		),
		createIssueHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_edit_issue",
			mcp.WithDescription("Edit an existing Jira issue"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Issue key (e.g. ESA-123)")),
			mcp.WithString("summary", mcp.Description("New summary")),
			mcp.WithString("description", mcp.Description("New description")),
			mcp.WithString("assignee", mcp.Description("New assignee username (empty to unassign)")),
			mcp.WithString("priority", mcp.Description("New priority name")),
			mcp.WithString("epic_name", mcp.Description("New Epic Name (only for Epics)")),
			mcp.WithString("epic_link", mcp.Description("New Epic key to link to (empty string to detach)")),
		),
		editIssueHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_transition_issue",
			mcp.WithDescription("Transition a Jira issue to a new status"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Issue key (e.g. ESA-123)")),
			mcp.WithString("to", mcp.Required(), mcp.Description("Name of the transition to perform")),
		),
		transitionIssueHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_list_transitions",
			mcp.WithDescription("List available transitions for a Jira issue"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Issue key (e.g. ESA-123)")),
		),
		listTransitionsHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_add_worklog",
			mcp.WithDescription("Add a worklog entry to a Jira issue"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Issue key (e.g. ESA-123)")),
			mcp.WithString("time", mcp.Required(), mcp.Description("Time spent (e.g., 2h, 30m, 1d)")),
			mcp.WithString("date", mcp.Description("Start date/time in ISO 8601 (optional, defaults to now)")),
			mcp.WithString("comment", mcp.Description("Worklog comment")),
		),
		addWorklogHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_list_worklogs",
			mcp.WithDescription("List worklog entries for a Jira issue"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Issue key (e.g. ESA-123)")),
		),
		listWorklogsHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_add_comment",
			mcp.WithDescription("Add a comment to a Jira issue"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Issue key (e.g. ESA-123)")),
			mcp.WithString("body", mcp.Required(), mcp.Description("Comment body text")),
		),
		addCommentHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_list_comments",
			mcp.WithDescription("List comments on a Jira issue"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Issue key (e.g. ESA-123)")),
		),
		listCommentsHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_edit_comment",
			mcp.WithDescription("Edit an existing comment on a Jira issue"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Issue key (e.g. ESA-123)")),
			mcp.WithString("comment_id", mcp.Required(), mcp.Description("Comment ID")),
			mcp.WithString("body", mcp.Required(), mcp.Description("Updated comment body (wiki markup)")),
		),
		editCommentHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_list_issue_types",
			mcp.WithDescription("List issue types available for creation in a project (e.g. Bug, Task, Story, Sub-task)"),
			mcp.WithString("project", mcp.Required(), mcp.Description("Project key (e.g. ESA)")),
		),
		listIssueTypesHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_list_statuses",
			mcp.WithDescription("List all statuses grouped by issue type for a project"),
			mcp.WithString("project", mcp.Required(), mcp.Description("Project key (e.g. ESA)")),
		),
		listStatusesHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_list_priorities",
			mcp.WithDescription("List all available issue priorities"),
		),
		listPrioritiesHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_list_epics",
			mcp.WithDescription("List Epics in a project (shortcut for jira_list_issues with issuetype=Epic)"),
			mcp.WithString("project", mcp.Description("Project key (e.g. ESA)")),
			mcp.WithString("status", mcp.Description("Filter by status name")),
			mcp.WithNumber("max", mcp.Description("Maximum number of results (default 50)")),
		),
		listEpicsHandler(jc, a.Config.Project),
	)

	s.AddTool(
		mcp.NewTool("jira_list_epic_children",
			mcp.WithDescription("List issues linked to an Epic (Epic Link = KEY)"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Epic issue key (e.g. ESA-42)")),
			mcp.WithNumber("max", mcp.Description("Maximum number of results (default 100)")),
		),
		listEpicChildrenHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_create_epic",
			mcp.WithDescription("Create an Epic (ergonomic shortcut over jira_create_issue with type=Epic)"),
			mcp.WithString("project", mcp.Required(), mcp.Description("Project key")),
			mcp.WithString("name", mcp.Required(), mcp.Description("Epic Name (shown on the Agile board)")),
			mcp.WithString("summary", mcp.Required(), mcp.Description("Epic summary")),
			mcp.WithString("description", mcp.Description("Epic description")),
			mcp.WithString("assignee", mcp.Description("Assignee username (use 'me' for current user)")),
			mcp.WithString("priority", mcp.Description("Priority name")),
		),
		createEpicHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_edit_epic",
			mcp.WithDescription("Edit an Epic (ergonomic shortcut over jira_edit_issue exposing 'name' for Epic Name)"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Epic issue key (e.g. ESA-42)")),
			mcp.WithString("name", mcp.Description("New Epic Name")),
			mcp.WithString("summary", mcp.Description("New summary")),
			mcp.WithString("description", mcp.Description("New description")),
			mcp.WithString("assignee", mcp.Description("New assignee username (empty to unassign)")),
			mcp.WithString("priority", mcp.Description("New priority name")),
		),
		editEpicHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_view_epic",
			mcp.WithDescription("Get an Epic and (optionally) its linked children in a single call"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Epic issue key (e.g. ESA-42)")),
			mcp.WithBoolean("include_children", mcp.Description("Include linked children in the response (default true)")),
			mcp.WithNumber("max_children", mcp.Description("Maximum children to return (default 100)")),
		),
		viewEpicHandler(jc),
	)

	return server.ServeStdio(s)
}

func toJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "marshal error: %s"}`, err)
	}
	return string(data)
}

func listIssuesHandler(jc *client.Client, defaultProject string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jql := req.GetString("jql", "")
		if jql == "" {
			jql = client.BuildJQLWith(client.JQLFilters{
				Project:  req.GetString("project", defaultProject),
				Status:   req.GetString("status", ""),
				Assignee: req.GetString("assignee", ""),
				Type:     req.GetString("type", ""),
				Epic:     req.GetString("epic", ""),
			})
		}

		max := req.GetInt("max", 50)
		issues, err := jc.SearchAllIssues(ctx, jql, max)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(toJSON(issues)), nil
	}
}

func getIssueHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		issue, err := jc.GetIssue(ctx, key)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(toJSON(issue)), nil
	}
}

func createIssueHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := req.RequireString("project")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		summary, err := req.RequireString("summary")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		issueType, err := req.RequireString("type")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		epicName := req.GetString("epic_name", "")
		epicLink := req.GetString("epic_link", "")
		isEpic := strings.EqualFold(issueType, "Epic")
		if isEpic && epicName == "" {
			return mcp.NewToolResultError("epic_name is required when type is Epic"), nil
		}
		if epicName != "" && !isEpic {
			return mcp.NewToolResultError("epic_name only applies when type is Epic"), nil
		}

		createReq := &models.CreateIssueRequest{
			Fields: models.CreateIssueFields{
				Project:     models.ProjectRef{Key: project},
				Summary:     summary,
				IssueType:   models.TypeRef{Name: issueType},
				Description: req.GetString("description", ""),
			},
		}

		if assignee := req.GetString("assignee", ""); assignee != "" {
			username := assignee
			if strings.EqualFold(assignee, "me") {
				user, err := jc.GetMyself(ctx)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("resolving current user: %s", err)), nil
				}
				username = user.Name
			}
			createReq.Fields.Assignee = &models.UserRef{Name: username}
		}

		if priority := req.GetString("priority", ""); priority != "" {
			createReq.Fields.Priority = &models.PriorityRef{Name: priority}
		}

		if parent := req.GetString("parent", ""); parent != "" {
			createReq.Fields.Parent = &models.IssueKeyRef{Key: parent}
		}

		if epicName != "" || epicLink != "" {
			epicNameID, epicLinkID, err := app.Get().EpicFieldIDs(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			createReq.Fields.Extra = map[string]any{}
			if epicName != "" {
				createReq.Fields.Extra[epicNameID] = epicName
			}
			if epicLink != "" {
				createReq.Fields.Extra[epicLinkID] = epicLink
			}
		}

		resp, err := jc.CreateIssue(ctx, createReq)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(toJSON(resp)), nil
	}
}

func editIssueHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		args := req.GetArguments()
		fields := make(map[string]any)

		if v, ok := args["summary"]; ok {
			fields["summary"] = v
		}
		if v, ok := args["description"]; ok {
			fields["description"] = v
		}
		if v, ok := args["assignee"]; ok {
			assignee, _ := v.(string)
			if assignee == "" {
				fields["assignee"] = nil
			} else {
				username := assignee
				if strings.EqualFold(assignee, "me") {
					user, err := jc.GetMyself(ctx)
					if err != nil {
						return mcp.NewToolResultError(fmt.Sprintf("resolving current user: %s", err)), nil
					}
					username = user.Name
				}
				fields["assignee"] = models.UserRef{Name: username}
			}
		}
		if v, ok := args["priority"]; ok {
			priority, _ := v.(string)
			fields["priority"] = models.PriorityRef{Name: priority}
		}

		_, hasEpicName := args["epic_name"]
		_, hasEpicLink := args["epic_link"]
		if hasEpicName || hasEpicLink {
			epicNameID, epicLinkID, err := app.Get().EpicFieldIDs(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if v, ok := args["epic_name"]; ok {
				s, _ := v.(string)
				fields[epicNameID] = s
			}
			if v, ok := args["epic_link"]; ok {
				s, _ := v.(string)
				if s == "" {
					fields[epicLinkID] = nil
				} else {
					fields[epicLinkID] = s
				}
			}
		}

		if len(fields) == 0 {
			return mcp.NewToolResultError("no fields to update"), nil
		}

		editReq := &models.EditIssueRequest{Fields: fields}
		if err := jc.EditIssue(ctx, key, editReq); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf(`{"updated": "%s"}`, key)), nil
	}
}

// listEpicsHandler lists Epics in a project — ergonomic shortcut over the generic
// list tool with issuetype=Epic preset.
func listEpicsHandler(jc *client.Client, defaultProject string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jql := client.BuildJQLWith(client.JQLFilters{
			Project: req.GetString("project", defaultProject),
			Status:  req.GetString("status", ""),
			Type:    "Epic",
		})

		// Include Epic Name in the response so AI agents can display it without
		// an extra round-trip. A failure to resolve is non-fatal.
		var extra []string
		if epicNameID, _, err := app.Get().EpicFieldIDs(ctx); err == nil && epicNameID != "" {
			extra = append(extra, epicNameID)
		}

		max := req.GetInt("max", 50)
		issues, err := jc.SearchAllIssues(ctx, jql, max, extra...)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(toJSON(issues)), nil
	}
}

// listEpicChildrenHandler lists issues linked to an Epic via the Epic Link field.
func listEpicChildrenHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jql := client.BuildJQLWith(client.JQLFilters{Epic: key})
		max := req.GetInt("max", 100)
		issues, err := jc.SearchAllIssues(ctx, jql, max)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(toJSON(issues)), nil
	}
}

func transitionIssueHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		transitionName, err := req.RequireString("to")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		transitions, err := jc.GetTransitions(ctx, key)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var match *models.Transition
		for i, t := range transitions {
			if strings.EqualFold(t.Name, transitionName) {
				match = &transitions[i]
				break
			}
		}

		if match == nil {
			names := make([]string, len(transitions))
			for i, t := range transitions {
				names[i] = t.Name
			}
			return mcp.NewToolResultError(fmt.Sprintf("transition %q not found; available: %s", transitionName, strings.Join(names, ", "))), nil
		}

		transReq := &models.TransitionRequest{
			Transition: models.TransitionRef{ID: match.ID},
		}
		if err := jc.DoTransition(ctx, key, transReq); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		target := transitionName
		if match.To != nil {
			target = match.To.Name
		}
		return mcp.NewToolResultText(fmt.Sprintf(`{"transitioned": "%s", "to": "%s"}`, key, target)), nil
	}
}

func addWorklogHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		timeSpent, err := req.RequireString("time")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		wlReq := &models.AddWorklogRequest{
			TimeSpent: timeSpent,
			Started:   req.GetString("date", ""),
			Comment:   req.GetString("comment", ""),
		}

		wl, err := jc.AddWorklog(ctx, key, wlReq)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(toJSON(wl)), nil
	}
}

func listWorklogsHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		worklogs, err := jc.GetWorklogs(ctx, key)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(toJSON(worklogs)), nil
	}
}

func addCommentHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, err := req.RequireString("body")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		comment, err := jc.AddComment(ctx, key, &models.AddCommentRequest{Body: body})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(toJSON(comment)), nil
	}
}

func listCommentsHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		comments, err := jc.GetComments(ctx, key)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(toJSON(comments)), nil
	}
}

func editCommentHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		commentID, err := req.RequireString("comment_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, err := req.RequireString("body")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		comment, err := jc.EditComment(ctx, key, commentID, body)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(toJSON(comment)), nil
	}
}

func listIssueTypesHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := req.RequireString("project")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		meta, err := jc.GetCreateMeta(ctx, project)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(toJSON(meta.IssueTypes)), nil
	}
}

func listStatusesHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := req.RequireString("project")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		statuses, err := jc.GetProjectStatuses(ctx, project)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(toJSON(statuses)), nil
	}
}

func listPrioritiesHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		priorities, err := jc.GetPriorities(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(toJSON(priorities)), nil
	}
}

func listTransitionsHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		transitions, err := jc.GetTransitions(ctx, key)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(toJSON(transitions)), nil
	}
}

// createEpicHandler creates an Epic with the required Epic Name custom field
// resolved dynamically. Mirrors `jira8 epic create`.
func createEpicHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := req.RequireString("project")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		name, err := req.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		summary, err := req.RequireString("summary")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		epicNameID, _, err := app.Get().EpicFieldIDs(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		createReq := &models.CreateIssueRequest{
			Fields: models.CreateIssueFields{
				Project:     models.ProjectRef{Key: project},
				Summary:     summary,
				IssueType:   models.TypeRef{Name: "Epic"},
				Description: req.GetString("description", ""),
				Extra:       map[string]any{epicNameID: name},
			},
		}

		if assignee := req.GetString("assignee", ""); assignee != "" {
			username := assignee
			if strings.EqualFold(assignee, "me") {
				user, err := jc.GetMyself(ctx)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("resolving current user: %s", err)), nil
				}
				username = user.Name
			}
			createReq.Fields.Assignee = &models.UserRef{Name: username}
		}
		if priority := req.GetString("priority", ""); priority != "" {
			createReq.Fields.Priority = &models.PriorityRef{Name: priority}
		}

		resp, err := jc.CreateIssue(ctx, createReq)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(toJSON(resp)), nil
	}
}

// editEpicHandler edits an Epic, exposing 'name' as a friendly alias for the
// Epic Name custom field. Mirrors `jira8 epic edit`.
func editEpicHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		args := req.GetArguments()
		fields := make(map[string]any)

		if v, ok := args["name"]; ok {
			epicNameID, _, err := app.Get().EpicFieldIDs(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			s, _ := v.(string)
			fields[epicNameID] = s
		}
		if v, ok := args["summary"]; ok {
			fields["summary"] = v
		}
		if v, ok := args["description"]; ok {
			fields["description"] = v
		}
		if v, ok := args["assignee"]; ok {
			assignee, _ := v.(string)
			if assignee == "" {
				fields["assignee"] = nil
			} else {
				username := assignee
				if strings.EqualFold(assignee, "me") {
					user, err := jc.GetMyself(ctx)
					if err != nil {
						return mcp.NewToolResultError(fmt.Sprintf("resolving current user: %s", err)), nil
					}
					username = user.Name
				}
				fields["assignee"] = models.UserRef{Name: username}
			}
		}
		if v, ok := args["priority"]; ok {
			priority, _ := v.(string)
			fields["priority"] = models.PriorityRef{Name: priority}
		}

		if len(fields) == 0 {
			return mcp.NewToolResultError("no fields to update"), nil
		}
		if err := jc.EditIssue(ctx, key, &models.EditIssueRequest{Fields: fields}); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf(`{"updated": "%s"}`, key)), nil
	}
}

// viewEpicHandler returns an Epic together with its linked children in a single
// call. Mirrors `jira8 epic view` (which fetches both eagerly).
func viewEpicHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		issue, err := jc.GetIssue(ctx, key)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		includeChildren := req.GetBool("include_children", true)
		var children []models.Issue
		if includeChildren {
			max := req.GetInt("max_children", 100)
			jql := client.BuildJQLWith(client.JQLFilters{Epic: key})
			children, err = jc.SearchAllIssues(ctx, jql, max)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("fetching children: %s", err)), nil
			}
		}

		out := struct {
			Epic     *models.Issue  `json:"epic"`
			Children []models.Issue `json:"children,omitempty"`
		}{Epic: issue, Children: children}
		return mcp.NewToolResultText(toJSON(out)), nil
	}
}
