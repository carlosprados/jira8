package app

import (
	"github.com/amplia/jira-cli/internal/client"
	"github.com/amplia/jira-cli/internal/config"
)

// State holds shared state accessible by all subcommands.
type State struct {
	Config *config.Config
	Client *client.Client
	Output string
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
