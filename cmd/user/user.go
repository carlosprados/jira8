package user

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

// UserCmd is the parent command for user lookups.
var UserCmd = &cobra.Command{
	Use:   "user",
	Short: "Find users (usernames for --assignee and user fields)",
}

var searchCmd = &cobra.Command{
	Use:   "search QUERY",
	Short: "Find active users by username, name or email fragment",
	Example: `  jira8 user search garcia
  jira8 user search ana --project ABC     # only users assignable in ABC
  jira8 user search ana -o json | jq -r '.[].name'`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

var meCmd = &cobra.Command{
	Use:     "me",
	Short:   "Show the authenticated user",
	Example: "  jira8 user me -o json",
	Args:    cobra.NoArgs,
	RunE:    runMe,
}

func init() {
	searchCmd.Flags().String("project", "", "Only users assignable to issues in this project")
	searchCmd.Flags().Int("max", 20, "Maximum number of users")
	UserCmd.AddCommand(searchCmd)
	UserCmd.AddCommand(meCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	project, _ := cmd.Flags().GetString("project")
	max, _ := cmd.Flags().GetInt("max")
	users, err := app.Get().Client.SearchUsers(context.Background(), args[0], project, max)
	if err != nil {
		return err
	}
	if app.Get().Output == "json" {
		return app.OutputJSON(users)
	}
	if len(users) == 0 {
		fmt.Printf("No users match %q.\n", args[0])
		return nil
	}
	for _, u := range users {
		printUser(u)
	}
	return nil
}

func runMe(cmd *cobra.Command, args []string) error {
	u, err := app.Get().Client.Me(context.Background())
	if err != nil {
		return err
	}
	if app.Get().Output == "json" {
		return app.OutputJSON(u)
	}
	printUser(*u)
	return nil
}

func printUser(u models.UserInfo) {
	fmt.Printf("  %-28s %-30s %s\n", u.Name, u.DisplayName, u.EmailAddress)
}
