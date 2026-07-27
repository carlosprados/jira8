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
	Attachment  []Attachment    `json:"attachment,omitempty"`
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
	ID      string `json:"id"`
	Author  *User  `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
	Updated string `json:"updated,omitempty"`
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

// IssueLinkType is a Jira issue link type (e.g. "Relates", "Blocks").
// Inward/Outward are the human-readable relationship phrases
// (e.g. "is blocked by" / "blocks").
type IssueLinkType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// IssueLinkTypesResponse is the response from GET /rest/api/2/issueLinkType.
type IssueLinkTypesResponse struct {
	IssueLinkTypes []IssueLinkType `json:"issueLinkTypes"`
}

// IssueLinkTypeRef references a link type by name in a link request.
type IssueLinkTypeRef struct {
	Name string `json:"name"`
}

// LinkedIssueRef references an issue by key inside an issue-link request.
type LinkedIssueRef struct {
	Key string `json:"key"`
}

// IssueLinkComment is an optional comment posted alongside an issue link.
type IssueLinkComment struct {
	Body string `json:"body"`
}

// IssueLinkRequest is the POST body for /rest/api/2/issueLink. The outward
// issue is the subject of the relationship ("OUTWARD <outward-phrase> INWARD",
// e.g. "ESA-207 blocks ESA-214"). For symmetric types like "Relates" the
// direction is irrelevant.
type IssueLinkRequest struct {
	Type         IssueLinkTypeRef  `json:"type"`
	InwardIssue  LinkedIssueRef    `json:"inwardIssue"`
	OutwardIssue LinkedIssueRef    `json:"outwardIssue"`
	Comment      *IssueLinkComment `json:"comment,omitempty"`
}

// CreateIssueResponse is the response from POST /rest/api/2/issue.
type CreateIssueResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// IssueTypeWithStatuses is a single issue type with its available statuses,
// one element of the response from GET /rest/api/2/project/{key}/statuses.
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

// Attachment represents a Jira issue attachment (returned in IssueFields.Attachment
// and by POST /rest/api/2/issue/{key}/attachments).
type Attachment struct {
	ID        string `json:"id"`
	Self      string `json:"self,omitempty"`
	Filename  string `json:"filename"`
	Author    *User  `json:"author,omitempty"`
	Created   string `json:"created,omitempty"`
	Size      int64  `json:"size,omitempty"`
	MimeType  string `json:"mimeType,omitempty"`
	Content   string `json:"content,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
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

// Board is an Agile (Greenhopper) board as returned by
// GET /rest/agile/1.0/board. Type is "kanban" or "scrum".
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// BoardsResponse is the paginated envelope returned by GET /rest/agile/1.0/board.
// The Agile API paginates with isLast rather than the startAt/total arithmetic
// used by /rest/api/2/search.
type BoardsResponse struct {
	MaxResults int     `json:"maxResults"`
	StartAt    int     `json:"startAt"`
	Total      int     `json:"total"`
	IsLast     bool    `json:"isLast"`
	Values     []Board `json:"values"`
}

// BoardConfiguration is returned by GET /rest/agile/1.0/board/{id}/configuration.
// It is the only source for two things ranking needs: the board's saved filter
// (to scope a JQL query to exactly what the board shows) and the column→statuses
// mapping (to know which statuses make up a kanban column).
type BoardConfiguration struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Type         string            `json:"type,omitempty"`
	Filter       BoardFilter       `json:"filter"`
	ColumnConfig BoardColumnConfig `json:"columnConfig"`
	// Ranking carries the instance's rank custom field ID. Some boards return an
	// empty object here, so callers must fall back to resolving SchemaRank via
	// GET /rest/api/2/field.
	Ranking *BoardRanking `json:"ranking,omitempty"`
}

// BoardFilter references the saved filter backing a board. The ID is a string in
// the Agile API payload even though it is numeric.
type BoardFilter struct {
	ID string `json:"id"`
}

// BoardColumnConfig holds the ordered columns of a board, left to right.
type BoardColumnConfig struct {
	Columns []BoardColumn `json:"columns"`
}

// BoardColumn is a single board column. A column maps to zero or more statuses;
// an issue belongs to the column whose Statuses contain the issue's status ID.
// Columns with no statuses (a disabled kanban Backlog, for instance) hold nothing.
type BoardColumn struct {
	Name     string              `json:"name"`
	Statuses []BoardColumnStatus `json:"statuses"`
}

// BoardColumnStatus references a status mapped to a column.
type BoardColumnStatus struct {
	ID string `json:"id"`
}

// BoardRanking carries the numeric ID of the LexoRank custom field used by the
// board (e.g. 10200 for customfield_10200).
type BoardRanking struct {
	RankCustomFieldID int `json:"rankCustomFieldId,omitempty"`
}

// RankIssuesRequest is the PUT body for /rest/agile/1.0/issue/rank. Exactly one
// of RankBeforeIssue / RankAfterIssue must be set; the moved issues keep their
// relative order and land immediately before (or after) the anchor. Jira accepts
// at most 50 issues per call.
type RankIssuesRequest struct {
	Issues          []string `json:"issues"`
	RankBeforeIssue string   `json:"rankBeforeIssue,omitempty"`
	RankAfterIssue  string   `json:"rankAfterIssue,omitempty"`
	// RankCustomFieldID is optional per the API docs, but Jira Server instances
	// can expose more than one rank field (an obsolete global rank alongside the
	// LexoRank one), so we always send the resolved ID to stay unambiguous.
	RankCustomFieldID int `json:"rankCustomFieldId,omitempty"`
}
