package app

import (
	"context"
	"sync"

	"github.com/amplia/jira8/internal/client"
	"github.com/amplia/jira8/internal/config"
)

// State holds shared state accessible by all subcommands.
type State struct {
	Config *config.Config
	Client *client.Client
	Output string

	// Epic custom field IDs resolved lazily on first use via ResolveEpicFields.
	epicOnce       sync.Once
	epicNameID     string
	epicLinkID     string
	epicResolveErr error
}

var instance *State

// Set stores the shared application state.
func Set(s *State) {
	instance = s
}

// Get returns the shared application state.
func Get() *State {
	return instance
}

// EpicFieldIDs returns the Jira customfield_XXXXX IDs for Epic Name and Epic Link.
// The lookup is performed once per process via /rest/api/2/field and cached.
// Used by create/edit/view commands and MCP tools that touch Epic fields.
func (s *State) EpicFieldIDs(ctx context.Context) (epicNameID, epicLinkID string, err error) {
	s.epicOnce.Do(func() {
		s.epicNameID, s.epicLinkID, s.epicResolveErr = s.Client.ResolveEpicFields(ctx)
	})
	return s.epicNameID, s.epicLinkID, s.epicResolveErr
}
