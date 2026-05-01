package app

import (
	"encoding/json"

	"github.com/amplia/jira8/internal/markup"
	"github.com/amplia/jira8/internal/models"
)

// RenderIssueAsMarkdown converts the description and any embedded comment
// bodies of an issue from Jira Wiki Markup to Markdown in place. Used by read
// commands and tools when the user has opted in via --markdown / format=markdown.
//
// IssueFields.MarshalJSON re-emits the raw payload received from Jira to
// preserve custom fields, so we also overwrite the relevant entries in Raw
// whenever it is populated; otherwise the JSON output would still contain the
// original Wiki Markup despite the typed fields being converted.
func RenderIssueAsMarkdown(issue *models.Issue) {
	if issue == nil {
		return
	}
	issue.Fields.Description = markup.WikiToMarkdown(issue.Fields.Description)
	if issue.Fields.Comment != nil {
		for i := range issue.Fields.Comment.Comments {
			issue.Fields.Comment.Comments[i].Body = markup.WikiToMarkdown(issue.Fields.Comment.Comments[i].Body)
		}
	}
	syncRawWithTyped(&issue.Fields)
}

// RenderIssuesAsMarkdown applies RenderIssueAsMarkdown to every entry in the
// slice. Mutates in place.
func RenderIssuesAsMarkdown(issues []models.Issue) {
	for i := range issues {
		RenderIssueAsMarkdown(&issues[i])
	}
}

// RenderCommentsAsMarkdown converts each comment body in the slice from Jira
// Wiki Markup to Markdown in place.
func RenderCommentsAsMarkdown(comments []models.Comment) {
	for i := range comments {
		comments[i].Body = markup.WikiToMarkdown(comments[i].Body)
	}
}

// RenderWorklogsAsMarkdown converts each worklog comment in the slice from
// Jira Wiki Markup to Markdown in place.
func RenderWorklogsAsMarkdown(worklogs []models.Worklog) {
	for i := range worklogs {
		worklogs[i].Comment = markup.WikiToMarkdown(worklogs[i].Comment)
	}
}

// syncRawWithTyped overwrites the description and comment entries in
// IssueFields.Raw with the current typed values. No-op when Raw is empty
// (synthetic instances are marshalled from typed fields directly).
func syncRawWithTyped(f *models.IssueFields) {
	if len(f.Raw) == 0 {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(f.Raw, &raw); err != nil {
		return
	}
	if b, err := json.Marshal(f.Description); err == nil {
		raw["description"] = b
	}
	if f.Comment != nil {
		if b, err := json.Marshal(f.Comment); err == nil {
			raw["comment"] = b
		}
	}
	if newRaw, err := json.Marshal(raw); err == nil {
		f.Raw = newRaw
	}
}
