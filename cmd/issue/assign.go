package issue

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var assignCmd = &cobra.Command{
	Use:   "assign ISSUE-KEY USER",
	Short: "Assign an issue (me, a username, or \"\" to unassign)",
	Long: `Assigns the issue to USER: a Jira username ('jira8 user search' finds it), "me",
or "" to leave it unassigned. Needs only the Assign Issues permission, so it
works where 'issue edit --assignee' is refused.`,
	Example: "  jira8 issue assign ABC-123 me\n  jira8 issue assign ABC-123 jdoe\n  jira8 issue assign ABC-123 \"\"",
	Args:    cobra.ExactArgs(2),
	RunE:    runAssign,
}

var watchCmd = &cobra.Command{
	Use:     "watch ISSUE-KEY [USER]",
	Short:   "Watch an issue (you, or USER)",
	Example: "  jira8 issue watch ABC-123\n  jira8 issue watch ABC-123 jdoe",
	Args:    cobra.RangeArgs(1, 2),
	RunE:    func(cmd *cobra.Command, args []string) error { return runWatch(args, true) },
}

var unwatchCmd = &cobra.Command{
	Use:     "unwatch ISSUE-KEY [USER]",
	Short:   "Stop watching an issue (you, or USER)",
	Example: "  jira8 issue unwatch ABC-123\n  jira8 issue unwatch ABC-123 jdoe",
	Args:    cobra.RangeArgs(1, 2),
	RunE:    func(cmd *cobra.Command, args []string) error { return runWatch(args, false) },
}

var watchersCmd = &cobra.Command{
	Use:     "watchers ISSUE-KEY",
	Short:   "List who watches an issue",
	Example: "  jira8 issue watchers ABC-123",
	Args:    cobra.ExactArgs(1),
	RunE:    runWatchers,
}

func runAssign(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key, user := args[0], args[1]
	if user != "" {
		var err error
		if user, err = a.Client.ResolveAssignee(context.Background(), user); err != nil {
			return err
		}
	}
	if err := a.Client.Assign(context.Background(), key, user); err != nil {
		return err
	}
	if user == "" {
		return app.Done(map[string]any{"key": key, "assignee": nil}, "%s unassigned", key)
	}
	return app.Done(map[string]any{"key": key, "assignee": user}, "%s assigned to %s", key, user)
}

func runWatch(args []string, watch bool) error {
	a := app.Get()
	key, user := args[0], "me"
	if len(args) == 2 {
		user = args[1]
	}
	user, err := a.Client.ResolveAssignee(context.Background(), user)
	if err != nil {
		return err
	}
	verb := "now watches"
	if watch {
		err = a.Client.AddWatcher(context.Background(), key, user)
	} else {
		verb = "no longer watches"
		err = a.Client.RemoveWatcher(context.Background(), key, user)
	}
	if err != nil {
		// Jira answers an unknown username with a bare 400.
		return fmt.Errorf("watcher %s (an existing username? see 'jira8 user search'): %w", user, err)
	}
	return app.Done(map[string]any{"key": key, "watcher": user, "watching": watch}, "%s %s %s", user, verb, key)
}

func runWatchers(cmd *cobra.Command, args []string) error {
	a := app.Get()
	w, err := a.Client.GetWatchers(context.Background(), args[0])
	if err != nil {
		return err
	}
	if a.Output == "json" {
		return app.OutputJSON(w)
	}
	if len(w.Watchers) == 0 {
		fmt.Printf("Nobody watches %s.\n", args[0])
		return nil
	}
	for _, u := range w.Watchers {
		fmt.Printf("%-24s %s\n", u.Name, u.DisplayName)
	}
	if w.IsWatching {
		fmt.Println("\n(you are watching it)")
	}
	return nil
}
