package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ProjectSummary is one project of `project list`. Category and Archived are
// only set when Jira sends them (archiving is a Data Center feature).
type ProjectSummary struct {
	Key       string `json:"key"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Lead      string `json:"lead,omitempty"`      // username
	LeadName  string `json:"leadName,omitempty"`  // display name
	LeadEmail string `json:"leadEmail,omitempty"` // to join with other systems
	Category  string `json:"category,omitempty"`
	Archived  *bool  `json:"archived,omitempty"`
	URL       string `json:"url"`
}

// ListProjects returns every project the user can browse, archived ones too
// where Jira supports it. The lead's email is not in the project payload: it
// costs one user lookup per distinct lead.
func (c *Client) ListProjects(ctx context.Context) ([]ProjectSummary, error) {
	data, err := c.do(ctx, http.MethodGet, "/project?expand=lead&includeArchived=true", nil)
	if err != nil {
		return nil, err
	}
	var raw []struct {
		Key      string `json:"key"`
		ID       string `json:"id"`
		Name     string `json:"name"`
		TypeKey  string `json:"projectTypeKey"`
		Archived *bool  `json:"archived"`
		Lead     *struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"lead"`
		Category *struct {
			Name string `json:"name"`
		} `json:"projectCategory"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing projects: %w", err)
	}

	emails := map[string]string{}
	out := make([]ProjectSummary, 0, len(raw))
	for _, p := range raw {
		s := ProjectSummary{Key: p.Key, ID: p.ID, Name: p.Name, Type: p.TypeKey,
			Archived: p.Archived, URL: c.baseURL + "/projects/" + url.PathEscape(p.Key)}
		if p.Category != nil {
			s.Category = p.Category.Name
		}
		if p.Lead != nil {
			s.Lead, s.LeadName = p.Lead.Name, p.Lead.DisplayName
			email, seen := emails[s.Lead]
			if !seen {
				email = c.userEmail(ctx, s.Lead)
				emails[s.Lead] = email
			}
			s.LeadEmail = email
		}
		out = append(out, s)
	}
	return out, nil
}

// userEmail looks a user up by username; a deleted or hidden user gives "".
func (c *Client) userEmail(ctx context.Context, username string) string {
	data, err := c.do(ctx, http.MethodGet, "/user?username="+url.QueryEscape(username), nil)
	if err != nil {
		return ""
	}
	var u struct {
		EmailAddress string `json:"emailAddress"`
	}
	if json.Unmarshal(data, &u) != nil {
		return ""
	}
	return u.EmailAddress
}
