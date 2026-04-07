package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/amplia/jira-cli/internal/config"
	"github.com/amplia/jira-cli/internal/models"
)

const (
	maxRetries  = 3
	pageSize    = 50
	apiBasePath = "/rest/api/2"
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
	token      string
	httpClient *http.Client
}

// New creates a new Jira API client from config.
func New(cfg *config.Config) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.URL, "/"),
		token:   cfg.Token,
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
		req.Header.Set("Authorization", "Bearer "+c.token)
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

// SearchIssues searches for issues using JQL.
func (c *Client) SearchIssues(ctx context.Context, jql string, startAt, maxResults int) (*models.SearchResult, error) {
	path := fmt.Sprintf("/search?jql=%s&startAt=%d&maxResults=%d&fields=summary,status,assignee,priority,issuetype,project,created,updated",
		url.QueryEscape(jql), startAt, maxResults)

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
func (c *Client) SearchAllIssues(ctx context.Context, jql string, max int) ([]models.Issue, error) {
	var all []models.Issue
	startAt := 0

	for {
		perPage := pageSize
		if max > 0 && max-len(all) < perPage {
			perPage = max - len(all)
		}

		result, err := c.SearchIssues(ctx, jql, startAt, perPage)
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

// BuildJQL constructs a JQL query from individual filter parameters.
func BuildJQL(project, status, assignee string) string {
	var clauses []string

	if project != "" {
		clauses = append(clauses, fmt.Sprintf("project = %s", project))
	}
	if status != "" {
		clauses = append(clauses, fmt.Sprintf("status = \"%s\"", status))
	}
	if assignee != "" {
		if strings.EqualFold(assignee, "me") {
			clauses = append(clauses, "assignee = currentUser()")
		} else {
			clauses = append(clauses, fmt.Sprintf("assignee = \"%s\"", assignee))
		}
	}

	if len(clauses) == 0 {
		return "ORDER BY updated DESC"
	}
	return strings.Join(clauses, " AND ") + " ORDER BY updated DESC"
}
