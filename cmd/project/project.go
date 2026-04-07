package project

import "github.com/spf13/cobra"

// ProjectCmd is the parent command for project metadata operations.
var ProjectCmd = &cobra.Command{
	Use:     "project",
	Aliases: []string{"p"},
	Short:   "Query project metadata (types, statuses, priorities)",
}

func init() {
	ProjectCmd.AddCommand(typesCmd)
	ProjectCmd.AddCommand(statusesCmd)
	ProjectCmd.AddCommand(prioritiesCmd)
}
