package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/amplia/jira8/internal/config"
	"github.com/amplia/jira8/internal/models"
)

const (
	maxRetries  = 3
	pageSize    = 50
	apiBasePath = "/rest/api/2"

	// Jira Server 8 schema identifiers for Greenhopper (Agile) custom fields.
	// These are stable across instances; the numeric customfield_XXXXX IDs are NOT,
	// which is why we resolve them dynamically via GET /field.
	SchemaEpicName = "com.pyxis.greenhopper.jira:gh-epic-label"
	SchemaEpicLink = "com.pyxis.greenhopper.jira:gh-epic-link"
)

// APIError represents a Jira API error with status code and messages.
type APIError struct {
	StatusCode int
	Messages   []string
	Errors     map[string]string
}

func (e *APIError) Error() string {
	var parts []string
	for _, m := range e.Messages {
		parts = append(parts, m)
	}
	for field, msg := range e.Errors {
		parts = append(parts, fmt.Sprintf("%s: %s", field, msg))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Jira API error (HTTP %d)", e.StatusCode)
	}
	return fmt.Sprintf("Jira API error (HTTP %d): %s", e.StatusCode, strings.Join(parts, "; "))
}

// Client is the Jira REST API client.
type Client struct {
	baseURL    string
	authHeader string
	httpClient *http.Client
}

// New creates a new Jira API client from config.
// Supports Bearer token auth and Basic auth (user:password).
func New(cfg *config.Config) *Client {
	auth := "Bearer " + cfg.Token
	if cfg.Token == "" && cfg.User != "" {
		auth = "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.User+":"+cfg.Password))
	}
	return &Client{
		baseURL:    strings.TrimRight(cfg.URL, "/"),
		authHeader: auth,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// do executes an HTTP request against the Jira API with auth, retries, and error handling.
func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	fullURL := c.baseURL + apiBasePath + path

	for attempt := range maxRetries {
		req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Authorization", c.authHeader)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("executing request: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries-1 {
			wait := 5 * time.Second
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil && secs > 0 && secs <= 30 {
					wait = time.Duration(secs) * time.Second
				}
			}
			time.Sleep(wait)
			// Reset body reader for retry
			if body != nil {
				data, _ := json.Marshal(body)
				bodyReader = bytes.NewReader(data)
			}
			continue
		}

		if resp.StatusCode >= 400 {
			apiErr := &APIError{StatusCode: resp.StatusCode}
			var jiraErr models.JiraError
			if json.Unmarshal(respBody, &jiraErr) == nil {
				apiErr.Messages = jiraErr.ErrorMessages
				apiErr.Errors = jiraErr.Errors
			}
			return nil, apiErr
		}

		return respBody, nil
	}

	return nil, fmt.Errorf("max retries exceeded")
}

// defaultSearchFields is the baseline field set fetched by search queries.
var defaultSearchFields = []string{"summary", "status", "assignee", "priority", "issuetype", "project", "created", "updated"}

// SearchIssues searches for issues using JQL. extraFields, if non-empty, is appended
// to the default field set (e.g. resolved Epic Link/Name custom field IDs).
func (c *Client) SearchIssues(ctx context.Context, jql string, startAt, maxResults int, extraFields ...string) (*models.SearchResult, error) {
	fields := defaultSearchFields
	if len(extraFields) > 0 {
		fields = append(append([]string{}, defaultSearchFields...), extraFields...)
	}
	path := fmt.Sprintf("/search?jql=%s&startAt=%d&maxResults=%d&fields=%s",
		url.QueryEscape(jql), startAt, maxResults, strings.Join(fields, ","))

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result models.SearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing search results: %w", err)
	}
	return &result, nil
}

// SearchAllIssues fetches all issues matching JQL with automatic pagination.
// If max <= 0, fetches all results.
func (c *Client) SearchAllIssues(ctx context.Context, jql string, max int, extraFields ...string) ([]models.Issue, error) {
	var all []models.Issue
	startAt := 0

	for {
		perPage := pageSize
		if max > 0 && max-len(all) < perPage {
			perPage = max - len(all)
		}

		result, err := c.SearchIssues(ctx, jql, startAt, perPage, extraFields...)
		if err != nil {
			return nil, err
		}

		all = append(all, result.Issues...)

		if startAt+len(result.Issues) >= result.Total {
			break
		}
		if max > 0 && len(all) >= max {
			break
		}
		startAt += len(result.Issues)
	}

	return all, nil
}

// GetIssue fetches a single issue by key.
func (c *Client) GetIssue(ctx context.Context, key string) (*models.Issue, error) {
	data, err := c.do(ctx, http.MethodGet, "/issue/"+url.PathEscape(key), nil)
	if err != nil {
		return nil, err
	}

	var issue models.Issue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, fmt.Errorf("parsing issue: %w", err)
	}
	return &issue, nil
}

// CreateIssue creates a new issue.
func (c *Client) CreateIssue(ctx context.Context, req *models.CreateIssueRequest) (*models.CreateIssueResponse, error) {
	data, err := c.do(ctx, http.MethodPost, "/issue", req)
	if err != nil {
		return nil, err
	}

	var resp models.CreateIssueResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing create response: %w", err)
	}
	return &resp, nil
}

// EditIssue updates an existing issue.
func (c *Client) EditIssue(ctx context.Context, key string, req *models.EditIssueRequest) error {
	_, err := c.do(ctx, http.MethodPut, "/issue/"+url.PathEscape(key), req)
	return err
}

// GetTransitions returns available transitions for an issue.
func (c *Client) GetTransitions(ctx context.Context, key string) ([]models.Transition, error) {
	data, err := c.do(ctx, http.MethodGet, "/issue/"+url.PathEscape(key)+"/transitions", nil)
	if err != nil {
		return nil, err
	}

	var resp models.TransitionsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing transitions: %w", err)
	}
	return resp.Transitions, nil
}

// DoTransition performs a workflow transition on an issue.
func (c *Client) DoTransition(ctx context.Context, key string, req *models.TransitionRequest) error {
	_, err := c.do(ctx, http.MethodPost, "/issue/"+url.PathEscape(key)+"/transitions", req)
	return err
}

// AddWorklog adds a worklog entry to an issue.
func (c *Client) AddWorklog(ctx context.Context, key string, req *models.AddWorklogRequest) (*models.Worklog, error) {
	data, err := c.do(ctx, http.MethodPost, "/issue/"+url.PathEscape(key)+"/worklog", req)
	if err != nil {
		return nil, err
	}

	var wl models.Worklog
	if err := json.Unmarshal(data, &wl); err != nil {
		return nil, fmt.Errorf("parsing worklog response: %w", err)
	}
	return &wl, nil
}

// GetWorklogs returns all worklog entries for an issue.
func (c *Client) GetWorklogs(ctx context.Context, key string) ([]models.Worklog, error) {
	data, err := c.do(ctx, http.MethodGet, "/issue/"+url.PathEscape(key)+"/worklog", nil)
	if err != nil {
		return nil, err
	}

	var resp models.WorklogsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing worklogs: %w", err)
	}
	return resp.Worklogs, nil
}

// DeleteWorklog removes a worklog entry from an issue.
func (c *Client) DeleteWorklog(ctx context.Context, key, worklogID string) error {
	path := "/issue/" + url.PathEscape(key) + "/worklog/" + url.PathEscape(worklogID)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// AddComment adds a comment to an issue.
func (c *Client) AddComment(ctx context.Context, key string, req *models.AddCommentRequest) (*models.Comment, error) {
	data, err := c.do(ctx, http.MethodPost, "/issue/"+url.PathEscape(key)+"/comment", req)
	if err != nil {
		return nil, err
	}

	var comment models.Comment
	if err := json.Unmarshal(data, &comment); err != nil {
		return nil, fmt.Errorf("parsing comment response: %w", err)
	}
	return &comment, nil
}

// GetComments returns all comments for an issue.
func (c *Client) GetComments(ctx context.Context, key string) ([]models.Comment, error) {
	data, err := c.do(ctx, http.MethodGet, "/issue/"+url.PathEscape(key)+"/comment", nil)
	if err != nil {
		return nil, err
	}

	var resp models.CommentsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing comments: %w", err)
	}
	return resp.Comments, nil
}

// GetMyself returns the currently authenticated user.
func (c *Client) GetMyself(ctx context.Context) (*models.User, error) {
	data, err := c.do(ctx, http.MethodGet, "/myself", nil)
	if err != nil {
		return nil, err
	}

	var user models.User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("parsing user: %w", err)
	}
	return &user, nil
}

// GetProjectStatuses returns issue types and their statuses for a project.
func (c *Client) GetProjectStatuses(ctx context.Context, projectKey string) ([]models.IssueTypeWithStatuses, error) {
	data, err := c.do(ctx, http.MethodGet, "/project/"+url.PathEscape(projectKey)+"/statuses", nil)
	if err != nil {
		return nil, err
	}

	var result []models.IssueTypeWithStatuses
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing project statuses: %w", err)
	}
	return result, nil
}

// GetCreateMeta returns issue types available for creation in a project.
func (c *Client) GetCreateMeta(ctx context.Context, projectKey string) (*models.CreateMetaProject, error) {
	path := "/issue/createmeta?projectKeys=" + url.QueryEscape(projectKey)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var meta models.CreateMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing create meta: %w", err)
	}
	if len(meta.Projects) == 0 {
		return nil, fmt.Errorf("project %s not found or no issue types available", projectKey)
	}
	return &meta.Projects[0], nil
}

// GetPriorities returns all available priorities.
func (c *Client) GetPriorities(ctx context.Context) ([]models.Priority, error) {
	data, err := c.do(ctx, http.MethodGet, "/priority", nil)
	if err != nil {
		return nil, err
	}

	var result []models.Priority
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing priorities: %w", err)
	}
	return result, nil
}

// JQLFilters groups the individual filter inputs accepted by BuildJQL.
// Fields are optional — empty values are skipped.
type JQLFilters struct {
	Project  string
	Status   string
	Assignee string
	Epic     string // issue key whose Epic Link equals this value
	Type     string // issue type name (e.g. "Epic", "Story")
}

// BuildJQL constructs a JQL query from the provided filters.
// Use BuildJQLWith for the full filter set; this variant is kept for compatibility
// and covers project/status/assignee only.
func BuildJQL(project, status, assignee string) string {
	return BuildJQLWith(JQLFilters{Project: project, Status: status, Assignee: assignee})
}

// BuildJQLWith constructs a JQL query from a JQLFilters struct.
func BuildJQLWith(f JQLFilters) string {
	var clauses []string

	if f.Project != "" {
		clauses = append(clauses, fmt.Sprintf("project = %s", f.Project))
	}
	if f.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = \"%s\"", f.Status))
	}
	if f.Assignee != "" {
		if strings.EqualFold(f.Assignee, "me") {
			clauses = append(clauses, "assignee = currentUser()")
		} else {
			clauses = append(clauses, fmt.Sprintf("assignee = \"%s\"", f.Assignee))
		}
	}
	if f.Type != "" {
		clauses = append(clauses, fmt.Sprintf("issuetype = \"%s\"", f.Type))
	}
	if f.Epic != "" {
		clauses = append(clauses, fmt.Sprintf("\"Epic Link\" = %s", f.Epic))
	}

	if len(clauses) == 0 {
		return "ORDER BY updated DESC"
	}
	return strings.Join(clauses, " AND ") + " ORDER BY updated DESC"
}

// GetFields returns all field descriptors from the Jira instance (system + custom).
// Used to discover custom field IDs dynamically (e.g. Epic Name, Epic Link) since
// they vary between instances.
func (c *Client) GetFields(ctx context.Context) ([]models.Field, error) {
	data, err := c.do(ctx, http.MethodGet, "/field", nil)
	if err != nil {
		return nil, err
	}

	var fields []models.Field
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("parsing fields: %w", err)
	}
	return fields, nil
}

// ResolveEpicFields returns the customfield_XXXXX IDs for Epic Name and Epic Link
// on this Jira instance, by querying /field and matching on schema.custom.
// Returns an error if either is not found (the Agile/Greenhopper plugin is likely
// not installed).
func (c *Client) ResolveEpicFields(ctx context.Context) (epicNameID, epicLinkID string, err error) {
	fields, err := c.GetFields(ctx)
	if err != nil {
		return "", "", err
	}

	for _, f := range fields {
		if f.Schema == nil {
			continue
		}
		switch f.Schema.Custom {
		case SchemaEpicName:
			epicNameID = f.ID
		case SchemaEpicLink:
			epicLinkID = f.ID
		}
	}

	if epicNameID == "" || epicLinkID == "" {
		return "", "", fmt.Errorf("epic fields not found on this Jira instance (Epic Name: %q, Epic Link: %q); is the Agile/Greenhopper plugin installed?", epicNameID, epicLinkID)
	}
	return epicNameID, epicLinkID, nil
}
