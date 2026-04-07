package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amplia/jira-cli/cmd/app"
	"github.com/amplia/jira-cli/internal/models"
	"github.com/spf13/cobra"
)

var transitionCmd = &cobra.Command{
	Use:   "transition ISSUE-KEY",
	Short: "Transition an issue to a new status",
	Args:  cobra.ExactArgs(1),
	RunE:  runTransition,
}

var transitionsCmd = &cobra.Command{
	Use:   "transitions ISSUE-KEY",
	Short: "List available transitions for an issue",
	Args:  cobra.ExactArgs(1),
	RunE:  runTransitions,
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

	var match *models.Transition
	for i, t := range transitions {
		if strings.EqualFold(t.Name, to) {
			match = &transitions[i]
			break
		}
	}

	if match == nil {
		names := make([]string, len(transitions))
		for i, t := range transitions {
			names[i] = t.Name
		}
		return fmt.Errorf("transition %q not found; available: %s", to, strings.Join(names, ", "))
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
		data, err := json.MarshalIndent(transitions, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	printTransitions(transitions)
	return nil
}
