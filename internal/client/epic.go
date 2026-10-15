package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// EpicProgress counts an Epic's children (issues whose Epic Link is the Epic,
// not their sub-tasks) by status category, and sums their time tracking.
// Remaining only counts children not done; seconds are 0 when nobody estimated.
type EpicProgress struct {
	Children          int   `json:"children"`
	Todo              int   `json:"todo"`
	Doing             int   `json:"doing"`
	Done              int   `json:"done"`
	Unestimated       int   `json:"unestimated"` // children without an original estimate
	OriginalEstimate  int64 `json:"originalEstimateSeconds"`
	RemainingEstimate int64 `json:"remainingEstimateSeconds"`
	TimeSpent         int64 `json:"timeSpentSeconds"`
}

// epicKeysPerQuery keeps the `"Epic Link" in (…)` JQL well under URL limits.
const epicKeysPerQuery = 100

// EpicsProgress returns the progress of each Epic in keys, with one paginated
// search per 100 Epics instead of one per Epic. Epics without children are
// present with zero counts.
func (c *Client) EpicsProgress(ctx context.Context, epicLinkID string, keys []string) (map[string]*EpicProgress, error) {
	out := make(map[string]*EpicProgress, len(keys))
	for _, k := range keys {
		out[k] = &EpicProgress{}
	}
	fields := "status,timeoriginalestimate,timeestimate,timespent," + epicLinkID
	for start := 0; start < len(keys); start += epicKeysPerQuery {
		chunk := keys[start:min(start+epicKeysPerQuery, len(keys))]
		jql := `"Epic Link" in (` + strings.Join(chunk, ",") + `)`
		for startAt := 0; ; {
			path := fmt.Sprintf("/search?jql=%s&startAt=%d&maxResults=%d&fields=%s",
				url.QueryEscape(jql), startAt, pageSize, fields)
			data, err := c.do(ctx, http.MethodGet, path, nil)
			if err != nil {
				return nil, err
			}
			var page struct {
				Total  int `json:"total"`
				Issues []struct {
					Fields map[string]json.RawMessage `json:"fields"`
				} `json:"issues"`
			}
			if err := json.Unmarshal(data, &page); err != nil {
				return nil, fmt.Errorf("parsing epic children: %w", err)
			}
			for _, i := range page.Issues {
				tallyChild(out, i.Fields, epicLinkID)
			}
			startAt += len(page.Issues)
			if len(page.Issues) == 0 || startAt >= page.Total {
				break
			}
		}
	}
	return out, nil
}

// tallyChild adds one child, given its raw fields, to its Epic's progress.
func tallyChild(progress map[string]*EpicProgress, f map[string]json.RawMessage, epicLinkID string) {
	var epic string
	_ = json.Unmarshal(f[epicLinkID], &epic)
	p := progress[epic]
	if p == nil {
		return
	}
	var status struct {
		StatusCategory struct {
			Key string `json:"key"`
		} `json:"statusCategory"`
	}
	_ = json.Unmarshal(f["status"], &status)
	var orig, remaining, spent *int64
	_ = json.Unmarshal(f["timeoriginalestimate"], &orig)
	_ = json.Unmarshal(f["timeestimate"], &remaining)
	_ = json.Unmarshal(f["timespent"], &spent)

	p.Children++
	done := status.StatusCategory.Key == "done"
	switch {
	case done:
		p.Done++
	case status.StatusCategory.Key == "indeterminate":
		p.Doing++
	default:
		p.Todo++
	}
	if orig == nil {
		p.Unestimated++
	} else {
		p.OriginalEstimate += *orig
	}
	if remaining != nil && !done {
		p.RemainingEstimate += *remaining
	}
	if spent != nil {
		p.TimeSpent += *spent
	}
}
