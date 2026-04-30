package cmd

// MCP Prompts for jira8.
//
// Prompts are reusable conversational templates a client surfaces (Claude Code:
// /mcp__jira__name; Gemini CLI: /name) so users can invoke a structured request
// against the LLM with pre-fetched Jira context. They are NOT actions — they
// produce a multi-message blob; the LLM decides whether to follow up with tool
// calls. There is no CLI counterpart by design (a slash command that emits a
// conversational blob has no meaning in a non-LLM terminal).

import (
	"context"
	"fmt"
	"strings"

	"github.com/amplia/jira8/internal/client"
	"github.com/amplia/jira8/internal/models"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerPrompts installs the prompts on s.
func registerPrompts(s *server.MCPServer, jc *client.Client, defaultProject string) {
	s.AddPrompt(
		mcp.NewPrompt("triage_issue",
			mcp.WithPromptDescription("Load a Jira issue and ask the LLM to triage it: assess priority, spot missing info, suggest labels and assignee"),
			mcp.WithArgument("key", mcp.RequiredArgument(), mcp.ArgumentDescription("Issue key to triage (e.g. ESA-123)")),
		),
		triageIssuePromptHandler(jc),
	)

	s.AddPrompt(
		mcp.NewPrompt("create_bug_report",
			mcp.WithPromptDescription("Build a well-structured bug report ready to file via jira_create_issue (type=Bug)"),
			mcp.WithArgument("summary", mcp.RequiredArgument(), mcp.ArgumentDescription("Short summary of the bug")),
			mcp.WithArgument("steps_to_reproduce", mcp.RequiredArgument(), mcp.ArgumentDescription("Numbered steps to reproduce")),
			mcp.WithArgument("expected_behavior", mcp.RequiredArgument(), mcp.ArgumentDescription("What should happen")),
			mcp.WithArgument("actual_behavior", mcp.RequiredArgument(), mcp.ArgumentDescription("What actually happens")),
			mcp.WithArgument("environment", mcp.ArgumentDescription("Optional environment details (OS, browser, version, etc.)")),
			mcp.WithArgument("project", mcp.ArgumentDescription("Project key (defaults to the server's configured project)")),
		),
		createBugReportPromptHandler(defaultProject),
	)

	s.AddPrompt(
		mcp.NewPrompt("epic_breakdown",
			mcp.WithPromptDescription("Load an Epic with its current children and ask the LLM to propose missing stories or sub-tasks"),
			mcp.WithArgument("epic_key", mcp.RequiredArgument(), mcp.ArgumentDescription("Epic key to break down (e.g. ESA-42)")),
		),
		epicBreakdownPromptHandler(jc),
	)
}

func triageIssuePromptHandler(jc *client.Client) server.PromptHandlerFunc {
	return func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		key := req.Params.Arguments["key"]
		if key == "" {
			return nil, fmt.Errorf("argument 'key' is required")
		}

		issue, err := jc.GetIssue(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", key, err)
		}

		ctxBlob := summariseIssueForPrompt(issue)
		instructions := strings.Join([]string{
			"Triage the Jira issue above. Produce a concise report with:",
			"  1. Priority assessment (current vs suggested, with a one-line rationale).",
			"  2. Missing information that would block resolution (acceptance criteria, repro steps, scope, etc.).",
			"  3. Suggested labels (3 max, lowercase, hyphenated).",
			"  4. Suggested assignee — only if you can infer one from the description, otherwise say so.",
			"  5. Any duplicate / related issue you suspect (only if confident).",
			"Be terse. Do not invent facts. If a section has nothing useful, write \"n/a\".",
		}, "\n")

		messages := []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(ctxBlob+"\n\n"+instructions)),
		}
		return mcp.NewGetPromptResult(fmt.Sprintf("Triage of %s", key), messages), nil
	}
}

func createBugReportPromptHandler(defaultProject string) server.PromptHandlerFunc {
	return func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		args := req.Params.Arguments
		for _, k := range []string{"summary", "steps_to_reproduce", "expected_behavior", "actual_behavior"} {
			if args[k] == "" {
				return nil, fmt.Errorf("argument %q is required", k)
			}
		}

		project := args["project"]
		if project == "" {
			project = defaultProject
		}

		var body strings.Builder
		body.WriteString("h2. Steps to reproduce\n")
		body.WriteString(args["steps_to_reproduce"])
		body.WriteString("\n\nh2. Expected behavior\n")
		body.WriteString(args["expected_behavior"])
		body.WriteString("\n\nh2. Actual behavior\n")
		body.WriteString(args["actual_behavior"])
		if env := args["environment"]; env != "" {
			body.WriteString("\n\nh2. Environment\n")
			body.WriteString(env)
		}

		instructions := strings.Join([]string{
			fmt.Sprintf("Below is a fully formed Bug report for project %q. Review it for clarity, completeness and any obvious gaps.", project),
			"",
			"  - Summary: " + args["summary"],
			"",
			"--- Description (Jira wiki markup) ---",
			body.String(),
			"--- end description ---",
			"",
			"Then propose:",
			"  1. Whether the report is good to file as-is, or what to clarify first.",
			"  2. A suggested priority (Major/Critical/Minor/etc.).",
			"  3. The exact `jira_create_issue` invocation you would run (project, type=Bug, summary, description, priority).",
			"Do not invoke the tool unless the user confirms.",
		}, "\n")

		messages := []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(instructions)),
		}
		return mcp.NewGetPromptResult("Draft bug report for review", messages), nil
	}
}

func epicBreakdownPromptHandler(jc *client.Client) server.PromptHandlerFunc {
	return func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		key := req.Params.Arguments["epic_key"]
		if key == "" {
			return nil, fmt.Errorf("argument 'epic_key' is required")
		}

		epic, err := jc.GetIssue(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("loading epic %s: %w", key, err)
		}
		jql := client.BuildJQLWith(client.JQLFilters{Epic: key})
		children, err := jc.SearchAllIssues(ctx, jql, 100)
		if err != nil {
			return nil, fmt.Errorf("loading children of %s: %w", key, err)
		}

		var b strings.Builder
		b.WriteString(summariseIssueForPrompt(epic))
		b.WriteString(fmt.Sprintf("\n\nCurrent children (%d):\n", len(children)))
		if len(children) == 0 {
			b.WriteString("  (none)\n")
		}
		for _, c := range children {
			status := "-"
			if c.Fields.Status != nil {
				status = c.Fields.Status.Name
			}
			itype := "-"
			if c.Fields.IssueType != nil {
				itype = c.Fields.IssueType.Name
			}
			b.WriteString(fmt.Sprintf("  - %s [%s, %s] %s\n", c.Key, itype, status, c.Fields.Summary))
		}

		instructions := strings.Join([]string{
			"Analyse this Epic and its current children. Then propose:",
			"  1. Stories or sub-tasks that are likely missing to deliver the Epic, each with a one-line rationale.",
			"  2. Children that look out-of-scope or duplicate (if any).",
			"  3. A risk or open question that should be answered before further breakdown.",
			"Be specific. Do not invent technical detail not present in the Epic.",
			"For each suggested story, give a candidate `jira_create_issue` invocation (type=Story, epic_link=" + key + ") but do not invoke it.",
		}, "\n")

		messages := []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(b.String()+"\n"+instructions)),
		}
		return mcp.NewGetPromptResult(fmt.Sprintf("Breakdown of Epic %s", key), messages), nil
	}
}

// summariseIssueForPrompt produces a compact, deterministic textual summary of
// an issue suitable for embedding in a prompt. We avoid pasting the full raw
// payload because it includes ~50 custom fields most of which are noise.
func summariseIssueForPrompt(i *models.Issue) string {
	f := i.Fields
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Issue %s — %s\n", i.Key, f.Summary))
	if f.IssueType != nil {
		b.WriteString(fmt.Sprintf("  Type:     %s\n", f.IssueType.Name))
	}
	if f.Status != nil {
		b.WriteString(fmt.Sprintf("  Status:   %s\n", f.Status.Name))
	}
	if f.Priority != nil {
		b.WriteString(fmt.Sprintf("  Priority: %s\n", f.Priority.Name))
	}
	if f.Assignee != nil {
		name := f.Assignee.DisplayName
		if name == "" {
			name = f.Assignee.Name
		}
		b.WriteString(fmt.Sprintf("  Assignee: %s\n", name))
	}
	if f.Reporter != nil {
		name := f.Reporter.DisplayName
		if name == "" {
			name = f.Reporter.Name
		}
		b.WriteString(fmt.Sprintf("  Reporter: %s\n", name))
	}
	if len(f.Labels) > 0 {
		b.WriteString(fmt.Sprintf("  Labels:   %s\n", strings.Join(f.Labels, ", ")))
	}
	if f.Description != "" {
		b.WriteString("Description:\n")
		b.WriteString(f.Description)
		b.WriteString("\n")
	}
	return b.String()
}
