package cmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/amplia/jira8/internal/client"
)

func TestExitCode(t *testing.T) {
	api := func(status int) error {
		return fmt.Errorf("created ABC-1 but failed: %w", &client.APIError{StatusCode: status})
	}
	cases := []struct {
		err  error
		want int
	}{
		{nil, 0},
		{errors.New("transition \"x\" not found"), ExitError},
		{usageError{errors.New("unknown flag")}, ExitUsage},
		{api(401), ExitAuth},
		{api(403), ExitAuth},
		{api(404), ExitNotFound},
		{api(400), ExitRejected},
		{api(409), ExitRejected},
		{api(429), ExitUnavailable},
		{api(502), ExitUnavailable},
		{fmt.Errorf("%w: dial tcp", client.ErrUnreachable), ExitUnreachable},
	}
	for _, c := range cases {
		if got := ExitCode(c.err); got != c.want {
			t.Errorf("ExitCode(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}
