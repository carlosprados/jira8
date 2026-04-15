package cmd

// MCP Resources for jira8.
//
// Resources expose data the LLM can read by URI. They complement (not replace)
// the metadata tools — clients that don't support resources (e.g. LM Studio)
// keep using the tools.
//
// URI scheme:
//   jira://priorities                       — global priority list (static)
//   jira://projects/{key}/types             — issue types valid for a project
//   jira://projects/{key}/statuses          — statuses grouped by issue type
//   jira://issues/{key}                     — full issue payload (Jira raw JSON)
//
// Handlers return application/json so AI agents can parse without re-tokenising.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amplia/jira8/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerResources installs the four jira:// resources/templates on s.
func registerResources(s *server.MCPServer, jc *client.Client) {
	s.AddResource(
		mcp.NewResource(
			"jira://priorities",
			"Jira priorities",
			mcp.WithResourceDescription("Global list of issue priorities available on the Jira instance"),
			mcp.WithMIMEType("application/json"),
		),
		prioritiesResourceHandler(jc),
	)

	s.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"jira://projects/{key}/types",
			"Project issue types",
			mcp.WithTemplateDescription("Issue types available for creation in a given project (e.g. Bug, Story, Epic)"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		projectTypesResourceHandler(jc),
	)

	s.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"jira://projects/{key}/statuses",
			"Project statuses",
			mcp.WithTemplateDescription("Workflow statuses grouped by issue type for a given project"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		projectStatusesResourceHandler(jc),
	)

	s.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"jira://issues/{key}",
			"Jira issue",
			mcp.WithTemplateDescription("Full payload of a single Jira issue (raw fields, including custom fields like Epic Link)"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		issueResourceHandler(jc),
	)
}

func prioritiesResourceHandler(jc *client.Client) server.ResourceHandlerFunc {
	return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		priorities, err := jc.GetPriorities(ctx)
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, priorities)
	}
}

func projectTypesResourceHandler(jc *client.Client) server.ResourceTemplateHandlerFunc {
	return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		key, err := templateString(req, "key")
		if err != nil {
			return nil, err
		}
		meta, err := jc.GetCreateMeta(ctx, key)
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, meta.IssueTypes)
	}
}

func projectStatusesResourceHandler(jc *client.Client) server.ResourceTemplateHandlerFunc {
	return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		key, err := templateString(req, "key")
		if err != nil {
			return nil, err
		}
		statuses, err := jc.GetProjectStatuses(ctx, key)
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, statuses)
	}
}

func issueResourceHandler(jc *client.Client) server.ResourceTemplateHandlerFunc {
	return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		key, err := templateString(req, "key")
		if err != nil {
			return nil, err
		}
		issue, err := jc.GetIssue(ctx, key)
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, issue)
	}
}

// templateString fetches a string variable parsed from a resource template URI.
// mcp-go stores the captures as []string (the V field of yosida95/uritemplate
// Value) rather than a single string, so we unwrap the first element.
func templateString(req mcp.ReadResourceRequest, name string) (string, error) {
	v, ok := req.Params.Arguments[name]
	if !ok {
		return "", fmt.Errorf("missing template variable %q in URI %q", name, req.Params.URI)
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return "", fmt.Errorf("template variable %q in URI %q is empty", name, req.Params.URI)
		}
		return val, nil
	case []string:
		if len(val) == 0 || val[0] == "" {
			return "", fmt.Errorf("template variable %q in URI %q is empty", name, req.Params.URI)
		}
		return val[0], nil
	default:
		return "", fmt.Errorf("unexpected type %T for template variable %q", v, name)
	}
}

// jsonResource serialises v as application/json under the given URI.
func jsonResource(uri string, v any) ([]mcp.ResourceContents, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling resource %s: %w", uri, err)
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}
