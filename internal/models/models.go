package models

import (
	"bytes"
	"encoding/json"
)

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
// Raw preserves the raw JSON payload so callers can look up custom fields (e.g.
// customfield_10011 for Epic Name) by ID without the model knowing them upfront.
type IssueFields struct {
	Summary     string          `json:"summary"`
	Description string          `json:"description,omitempty"`
	Status      *Status         `json:"status,omitempty"`
	IssueType   *IssueType      `json:"issuetype,omitempty"`
	Priority    *Priority       `json:"priority,omitempty"`
	Assignee    *User           `json:"assignee,omitempty"`
	Reporter    *User           `json:"reporter,omitempty"`
	Project     *Project        `json:"project,omitempty"`
	Parent      *ParentIssue    `json:"parent,omitempty"`
	Created     string          `json:"created,omitempty"`
	Updated     string          `json:"updated,omitempty"`
	Labels      []string        `json:"labels,omitempty"`
	Components  []Component     `json:"components,omitempty"`
	Comment     *Comments       `json:"comment,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

// UnmarshalJSON stores the full raw payload in Raw while decoding the typed fields.
func (f *IssueFields) UnmarshalJSON(data []byte) error {
	type alias IssueFields
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*f = IssueFields(tmp)
	f.Raw = append(f.Raw[:0], data...)
	return nil
}

// MarshalJSON re-emits the original Jira payload when available, preserving
// custom fields (e.g. customfield_10011 / Epic Name) that are not modelled as
// typed fields. Falls back to the typed encoding for synthetic instances.
func (f IssueFields) MarshalJSON() ([]byte, error) {
	if len(f.Raw) > 0 {
		return f.Raw, nil
	}
	type alias IssueFields
	return json.Marshal(alias(f))
}

// CustomString returns a string-valued custom field by ID (e.g. "customfield_10011").
// Returns empty string if the field is missing, null, or not a string.
func (f *IssueFields) CustomString(id string) string {
	if len(f.Raw) == 0 || id == "" {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(f.Raw, &m); err != nil {
		return ""
	}
	raw, ok := m[id]
	if !ok || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// ParentIssue represents the parent of a Sub-task issue.
type ParentIssue struct {
	ID     string        `json:"id"`
	Key    string        `json:"key"`
	Self   string        `json:"self,omitempty"`
	Fields *ParentFields `json:"fields,omitempty"`
}

// ParentFields contains the summary of a parent issue (returned by Jira inside parent.fields).
type ParentFields struct {
	Summary string `json:"summary"`
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

// AddCommentRequest is the POST body for /rest/api/2/issue/{key}/comment.
type AddCommentRequest struct {
	Body string `json:"body"`
}

// CommentsResponse is the response from GET /rest/api/2/issue/{key}/comment.
type CommentsResponse struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Comments   []Comment `json:"comments"`
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
// Extra holds arbitrary custom fields (e.g. Epic Name, Epic Link) keyed by their
// customfield_XXXXX ID, and is merged into the same top-level JSON object as the
// typed fields via CreateIssueFields.MarshalJSON.
type CreateIssueFields struct {
	Project     ProjectRef     `json:"project"`
	Summary     string         `json:"summary"`
	IssueType   TypeRef        `json:"issuetype"`
	Description string         `json:"description,omitempty"`
	Assignee    *UserRef       `json:"assignee,omitempty"`
	Priority    *PriorityRef   `json:"priority,omitempty"`
	Parent      *IssueKeyRef   `json:"parent,omitempty"`
	Extra       map[string]any `json:"-"`
}

// MarshalJSON merges the typed fields with Extra into a single flat JSON object.
// Typed fields win over Extra on key collision to avoid accidental overrides.
func (f CreateIssueFields) MarshalJSON() ([]byte, error) {
	type alias CreateIssueFields
	typed, err := json.Marshal(alias(f))
	if err != nil {
		return nil, err
	}
	if len(f.Extra) == 0 {
		return typed, nil
	}

	var typedMap map[string]json.RawMessage
	if err := json.Unmarshal(typed, &typedMap); err != nil {
		return nil, err
	}
	for k, v := range f.Extra {
		if _, exists := typedMap[k]; exists {
			continue
		}
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		typedMap[k] = raw
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(typedMap); err != nil {
		return nil, err
	}
	// Encode appends a newline; trim it for clean output.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// IssueKeyRef references an issue by key (used for parent in sub-task creation).
type IssueKeyRef struct {
	Key string `json:"key"`
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
	ID            string `json:"id"`
	Self          string `json:"self"`
	Author        *User  `json:"author,omitempty"`
	UpdateAuthor  *User  `json:"updateAuthor,omitempty"`
	Created       string `json:"created"`
	Updated       string `json:"updated"`
	Started       string `json:"started"`
	TimeSpent     string `json:"timeSpent"`
	TimeSpentSecs int    `json:"timeSpentSeconds"`
	Comment       string `json:"comment,omitempty"`
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

// Field represents a Jira field descriptor returned by GET /rest/api/2/field.
// Custom fields carry a Schema.Custom string identifying their plugin origin
// (e.g. "com.pyxis.greenhopper.jira:gh-epic-label" for Epic Name).
type Field struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Custom bool         `json:"custom"`
	Schema *FieldSchema `json:"schema,omitempty"`
}

// FieldSchema identifies the type and origin of a Jira field.
type FieldSchema struct {
	Type   string `json:"type,omitempty"`
	Items  string `json:"items,omitempty"`
	Custom string `json:"custom,omitempty"`
}
