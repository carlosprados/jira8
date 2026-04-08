package models

// SearchResult represents the response from GET /rest/api/2/search.
type SearchResult struct {
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	Issues     []Issue `json:"issues"`
}

// Issue represents a Jira issue.
type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Self   string      `json:"self"`
	Fields IssueFields `json:"fields"`
}

// IssueFields contains the fields of a Jira issue.
type IssueFields struct {
	Summary     string      `json:"summary"`
	Description string      `json:"description,omitempty"`
	Status      *Status     `json:"status,omitempty"`
	IssueType   *IssueType  `json:"issuetype,omitempty"`
	Priority    *Priority   `json:"priority,omitempty"`
	Assignee    *User       `json:"assignee,omitempty"`
	Reporter    *User       `json:"reporter,omitempty"`
	Project     *Project    `json:"project,omitempty"`
	Created     string      `json:"created,omitempty"`
	Updated     string      `json:"updated,omitempty"`
	Labels      []string    `json:"labels,omitempty"`
	Components  []Component `json:"components,omitempty"`
	Comment     *Comments   `json:"comment,omitempty"`
}

// Status represents a Jira issue status.
type Status struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// IssueType represents a Jira issue type.
type IssueType struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// Priority represents a Jira issue priority.
type Priority struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// User represents a Jira user (Server 8: uses "name" field, not "accountId").
type User struct {
	Name         string `json:"name"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

// Project represents a Jira project.
type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	ID   string `json:"id"`
}

// Component represents a Jira component.
type Component struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// Comments represents the comment collection on an issue.
type Comments struct {
	Total    int       `json:"total"`
	Comments []Comment `json:"comments"`
}

// Comment represents a single comment.
type Comment struct {
	Author  *User  `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

// TransitionsResponse represents the response from GET /rest/api/2/issue/{key}/transitions.
type TransitionsResponse struct {
	Transitions []Transition `json:"transitions"`
}

// Transition represents a workflow transition.
type Transition struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	To   *Status `json:"to"`
}

// CreateIssueRequest is the POST body for /rest/api/2/issue.
type CreateIssueRequest struct {
	Fields CreateIssueFields `json:"fields"`
}

// CreateIssueFields contains the fields for creating an issue.
type CreateIssueFields struct {
	Project     ProjectRef   `json:"project"`
	Summary     string       `json:"summary"`
	IssueType   TypeRef      `json:"issuetype"`
	Description string       `json:"description,omitempty"`
	Assignee    *UserRef     `json:"assignee,omitempty"`
	Priority    *PriorityRef `json:"priority,omitempty"`
}

// ProjectRef references a project by key.
type ProjectRef struct {
	Key string `json:"key"`
}

// TypeRef references an issue type by name.
type TypeRef struct {
	Name string `json:"name"`
}

// UserRef references a user by name (Jira Server 8).
type UserRef struct {
	Name string `json:"name"`
}

// PriorityRef references a priority by name.
type PriorityRef struct {
	Name string `json:"name"`
}

// EditIssueRequest is the PUT body for /rest/api/2/issue/{key}.
// Uses map for partial updates — only changed fields are sent.
type EditIssueRequest struct {
	Fields map[string]any `json:"fields"`
}

// TransitionRequest is the POST body for /rest/api/2/issue/{key}/transitions.
type TransitionRequest struct {
	Transition TransitionRef `json:"transition"`
}

// TransitionRef references a transition by ID.
type TransitionRef struct {
	ID string `json:"id"`
}

// CreateIssueResponse is the response from POST /rest/api/2/issue.
type CreateIssueResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// ProjectStatuses represents the response from GET /rest/api/2/project/{key}/statuses.
// Returns issue types with their available statuses.
type IssueTypeWithStatuses struct {
	Name     string   `json:"name"`
	ID       string   `json:"id"`
	Statuses []Status `json:"statuses"`
}

// CreateMeta represents the response from GET /rest/api/2/issue/createmeta.
type CreateMeta struct {
	Projects []CreateMetaProject `json:"projects"`
}

// CreateMetaProject contains issue types available for creation in a project.
type CreateMetaProject struct {
	Key        string      `json:"key"`
	Name       string      `json:"name"`
	IssueTypes []IssueType `json:"issuetypes"`
}

// AddWorklogRequest is the POST body for /rest/api/2/issue/{key}/worklog.
type AddWorklogRequest struct {
	TimeSpent string `json:"timeSpent"`
	Started   string `json:"started,omitempty"`
	Comment   string `json:"comment,omitempty"`
}

// Worklog represents a single worklog entry.
type Worklog struct {
	ID             string `json:"id"`
	Self           string `json:"self"`
	Author         *User  `json:"author,omitempty"`
	UpdateAuthor   *User  `json:"updateAuthor,omitempty"`
	Created        string `json:"created"`
	Updated        string `json:"updated"`
	Started        string `json:"started"`
	TimeSpent      string `json:"timeSpent"`
	TimeSpentSecs  int    `json:"timeSpentSeconds"`
	Comment        string `json:"comment,omitempty"`
}

// WorklogsResponse is the response from GET /rest/api/2/issue/{key}/worklog.
type WorklogsResponse struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Worklogs   []Worklog `json:"worklogs"`
}

// JiraError represents a Jira API error response.
type JiraError struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}
