package client

import (
	"encoding/json"
	"net/http"
	"path/filepath"
)

// DryRunKey stands for the key of an issue a dry run would have created, so
// the requests planned after it (attachments, images) still have a path.
const DryRunKey = "NEW"

// PlannedRequest is a write a dry run recorded instead of sending.
type PlannedRequest struct {
	Method string   `json:"method"`
	Path   string   `json:"path"`
	Body   any      `json:"body,omitempty"`
	Files  []string `json:"files,omitempty"`
}

// SetDryRun makes every write (anything but GET) be recorded instead of sent.
// Reads still go to Jira, so names and screens are validated as usual.
func (c *Client) SetDryRun(on bool) { c.dryRun = on }

// DryRun reports whether writes are being recorded instead of sent.
func (c *Client) DryRun() bool { return c.dryRun }

// Planned returns the writes recorded in dry-run mode, in order.
func (c *Client) Planned() []PlannedRequest { return c.planned }

// plan records a write and returns the stand-in answer for it.
func (c *Client) plan(method, apiPath string, body any, files []string) []byte {
	p := PlannedRequest{Method: method, Path: apiPath, Body: body}
	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			abs = f
		}
		p.Files = append(p.Files, abs)
	}
	c.planned = append(c.planned, p)
	switch {
	case files != nil:
		return []byte("[]")
	case method == http.MethodPost && apiPath == apiBasePath+"/issue":
		data, _ := json.Marshal(map[string]string{"key": DryRunKey})
		return data
	default:
		return []byte("{}")
	}
}
