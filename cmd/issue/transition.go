package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var transitionCmd = &cobra.Command{
	Use:     "transition ISSUE-KEY",
	Short:   "Transition an issue to a new status",
	Example: "  jira8 issue transition ESA-123 --to \"Done\"",
	Args:    cobra.ExactArgs(1),
	RunE:    runTransition,
}

var transitionsCmd = &cobra.Command{
	Use:     "transitions ISSUE-KEY",
	Short:   "List available transitions for an issue",
	Example: "  jira8 issue transitions ESA-123",
	Args:    cobra.ExactArgs(1),
	RunE:    runTransitions,
}

func init() {
	transitionCmd.Flags().String("to", "", "Target transition name (required)")
	_ = transitionCmd.MarkFlagRequired("to")
}

func runTransition(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]
	to, _ := cmd.Flags().GetString("to")

	transitions, err := a.Client.GetTransitions(context.Background(), key)
	if err != nil {
		return err
	}

	transitionName := func(t models.Transition) string { return t.Name }
	match := app.MatchByName(transitions, to, transitionName)
	if match == nil {
		return fmt.Errorf("transition %q not found; available: %s", to, strings.Join(app.Names(transitions, transitionName), ", "))
	}

	req := &models.TransitionRequest{
		Transition: models.TransitionRef{ID: match.ID},
	}
	if err := a.Client.DoTransition(context.Background(), key, req); err != nil {
		return err
	}

	target := to
	if match.To != nil {
		target = match.To.Name
	}
	fmt.Printf("Transitioned %s → %s\n", key, target)
	return nil
}

func runTransitions(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	transitions, err := a.Client.GetTransitions(context.Background(), key)
	if err != nil {
		return err
	}

	if a.Output == "json" {
		return app.OutputJSON(transitions)
	}

	printTransitions(transitions)
	return nil
}
