// Package epic implements the `jira8 epic` subcommand tree: ergonomic wrappers
// around the generic issue CRUD that default issuetype=Epic and surface the
// Epic-specific fields (Epic Name, Epic Link children).
package epic

import "github.com/spf13/cobra"

// EpicCmd is the parent command for all Epic operations.
var EpicCmd = &cobra.Command{
	Use:     "epic",
	Aliases: []string{"e"},
	Short:   "Work with Epics (issuetype=Epic) and their children",
}

func init() {
	EpicCmd.AddCommand(listCmd)
	EpicCmd.AddCommand(viewCmd)
	EpicCmd.AddCommand(createCmd)
	EpicCmd.AddCommand(editCmd)
	EpicCmd.AddCommand(childrenCmd)
}
