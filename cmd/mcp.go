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
	)

	jc := a.Client

	s.AddTool(
		mcp.NewTool("jira_list_issues",
			mcp.WithDescription("List Jira issues using filters or JQL"),
			mcp.WithString("project", mcp.Description("Project key (e.g. ESA)")),
			mcp.WithString("status", mcp.Description("Filter by status name")),
			mcp.WithString("assignee", mcp.Description("Filter by assignee username, use 'me' for current user")),
			mcp.WithString("jql", mcp.Description("Raw JQL query (overrides other filters)")),
			mcp.WithNumber("max_results", mcp.Description("Maximum number of results (default 50)")),
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
			mcp.WithString("issue_type", mcp.Required(), mcp.Description("Issue type (e.g. Bug, Task, Story)")),
			mcp.WithString("description", mcp.Description("Issue description")),
			mcp.WithString("assignee", mcp.Description("Assignee username")),
			mcp.WithString("priority", mcp.Description("Priority name")),
			mcp.WithString("parent", mcp.Description("Parent issue key for Sub-task creation (e.g. ESA-65)")),
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
		),
		editIssueHandler(jc),
	)

	s.AddTool(
		mcp.NewTool("jira_transition_issue",
			mcp.WithDescription("Transition a Jira issue to a new status"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Issue key (e.g. ESA-123)")),
			mcp.WithString("transition_name", mcp.Required(), mcp.Description("Name of the transition to perform")),
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
			mcp.WithString("time_spent", mcp.Required(), mcp.Description("Time spent (e.g., 2h, 30m, 1d)")),
			mcp.WithString("started", mcp.Description("Start date/time in ISO 8601 (optional, defaults to now)")),
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
			project := req.GetString("project", defaultProject)
			status := req.GetString("status", "")
			assignee := req.GetString("assignee", "")
			jql = client.BuildJQL(project, status, assignee)
		}

		max := req.GetInt("max_results", 50)
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
		issueType, err := req.RequireString("issue_type")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
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

func transitionIssueHandler(jc *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		transitionName, err := req.RequireString("transition_name")
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
		timeSpent, err := req.RequireString("time_spent")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		wlReq := &models.AddWorklogRequest{
			TimeSpent: timeSpent,
			Started:   req.GetString("started", ""),
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
