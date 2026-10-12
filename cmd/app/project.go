package app

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ProjectKey returns --project, or the configured default project.
func ProjectKey(cmd *cobra.Command) (string, error) {
	if p, _ := cmd.Flags().GetString("project"); p != "" {
		return p, nil
	}
	if p := Get().Config.Project; p != "" {
		return p, nil
	}
	return "", fmt.Errorf("no project: pass --project KEY or set 'project' in ~/.jira.yaml (JIRA_PROJECT)")
}
