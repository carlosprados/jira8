package cmd

import (
	"errors"
	"net/http"

	"github.com/amplia/jira8/internal/client"
)

// Exit codes, documented in the README and the skill: scripts and agents
// branch on them, so do not renumber.
const (
	ExitError       = 1 // caught by jira8 before sending: names, required fields, config, files
	ExitUsage       = 2 // unknown command or flag, wrong arguments
	ExitAuth        = 3 // 401, 403
	ExitNotFound    = 4 // 404
	ExitRejected    = 5 // Jira rejected the request: 400, 409, other 4xx
	ExitUnavailable = 6 // 5xx, or 429 after the retries
	ExitUnreachable = 7 // no answer: network, VPN, timeout
)

// usageError marks an error cobra raised before running the command.
type usageError struct{ error }

func (e usageError) Unwrap() error { return e.error }

// ExitCode maps an error returned by Execute to the process exit code.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ue usageError
	if errors.As(err, &ue) {
		return ExitUsage
	}
	if errors.Is(err, client.ErrUnreachable) {
		return ExitUnreachable
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		return ExitError
	}
	switch s := apiErr.StatusCode; {
	case s == http.StatusUnauthorized || s == http.StatusForbidden:
		return ExitAuth
	case s == http.StatusNotFound:
		return ExitNotFound
	case s == http.StatusTooManyRequests || s >= 500:
		return ExitUnavailable
	case s >= 400:
		return ExitRejected
	}
	return ExitError
}
