package project

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var prioritiesCmd = &cobra.Command{
	Use:     "priorities",
	Short:   "List available priorities",
	Example: "  jira8 project priorities",
	RunE:    runPriorities,
}

func runPriorities(cmd *cobra.Command, args []string) error {
	a := app.Get()

	priorities, err := a.Client.GetPriorities(context.Background())
	if err != nil {
		return err
	}

	if a.Output == "json" {
		return app.OutputJSON(priorities)
	}

	fmt.Println("Priorities:")
	for _, p := range priorities {
		fmt.Printf("  %-20s (id: %s)\n", p.Name, p.ID)
	}
	return nil
}
