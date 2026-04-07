package project

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var statusesCmd = &cobra.Command{
	Use:     "statuses",
	Short:   "List statuses grouped by issue type",
	Example: "  jira8 project statuses --project ESA",
	RunE:    runStatuses,
}

func init() {
	statusesCmd.Flags().String("project", "", "Project key (default from config)")
}

func runStatuses(cmd *cobra.Command, args []string) error {
	a := app.Get()

	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = a.Config.Project
	}

	result, err := a.Client.GetProjectStatuses(context.Background(), project)
	if err != nil {
		return err
	}

	if a.Output == "json" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Statuses for %s:\n\n", project)
	for _, issueType := range result {
		names := make([]string, len(issueType.Statuses))
		for i, s := range issueType.Statuses {
			names[i] = s.Name
		}
		fmt.Printf("  %-20s %s\n", issueType.Name+":", strings.Join(names, ", "))
	}
	return nil
}
