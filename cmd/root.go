package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/cmd/board"
	"github.com/amplia/jira8/cmd/epic"
	"github.com/amplia/jira8/cmd/issue"
	"github.com/amplia/jira8/cmd/project"
	"github.com/amplia/jira8/cmd/user"
	"github.com/amplia/jira8/internal/client"
	"github.com/amplia/jira8/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configFile string
	dryRun     bool

	// Set by GoReleaser via ldflags.
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "jira8",
	Short: "CLI for Jira Server 8",
	Long: `A command-line interface for Jira Server / Data Center 8 (REST API v2).

Configuration: ~/.jira.yaml (or --config) with 'url' plus 'token' (Bearer) or
'user' + 'password' (Basic), and optionally a default 'project'. The same keys
work as JIRA_URL, JIRA_TOKEN, JIRA_USER, JIRA_PASSWORD and JIRA_PROJECT.

Use -o json to read the output from a script. Coding agents: run
'jira8 skill install' (or read 'jira8 skill show') before using this CLI.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Flags and args are already validated here, so from now on an error
		// is a runtime one (config, Jira API): print it without the usage.
		cmd.SilenceUsage = true
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}

		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		c := client.New(cfg)
		app.Set(&app.State{
			Config: cfg,
			Client: c,
			Output: viper.GetString("output"),
		})
		if dryRun {
			// What a write command prints describes writes that never
			// happened: Execute prints the planned requests instead, or
			// replays this output when the command wrote nothing.
			c.SetDryRun(true)
			f, err := os.CreateTemp("", "jira8-dry-run-*")
			if err != nil {
				return err
			}
			_ = os.Remove(f.Name())
			os.Stdout = f
		}
		return nil
	},
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default ~/.jira.yaml)")
	rootCmd.PersistentFlags().String("url", "", "Jira server URL")
	rootCmd.PersistentFlags().String("token", "", "Personal access token")
	rootCmd.PersistentFlags().String("project", "", "Default project key")
	rootCmd.PersistentFlags().StringP("output", "o", "text", "Output format: text or json")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Validate against Jira and print the writes instead of sending them")

	_ = viper.BindPFlag("url", rootCmd.PersistentFlags().Lookup("url"))
	_ = viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token"))
	_ = viper.BindPFlag("project", rootCmd.PersistentFlags().Lookup("project"))
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	rootCmd.Version = version
	rootCmd.SetVersionTemplate(fmt.Sprintf("jira8 %s (commit: %s, built: %s)\n", version, commit, date))

	rootCmd.AddCommand(issue.IssueCmd)
	rootCmd.AddCommand(epic.EpicCmd)
	rootCmd.AddCommand(project.ProjectCmd)
	rootCmd.AddCommand(board.BoardCmd)
	rootCmd.AddCommand(user.UserCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(pingCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(markupCmd)
	rootCmd.AddCommand(skillCmd)
}

// strictGroups makes an unknown subcommand ("jira8 issue nope") a usage error.
// Cobra would print the group's help and exit 0, which a script reads as
// success.
func strictGroups(c *cobra.Command) {
	for _, sub := range c.Commands() {
		if sub.HasSubCommands() && sub.Run == nil && sub.RunE == nil {
			sub.Args = cobra.NoArgs
			sub.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
		}
		strictGroups(sub)
	}
}

// Execute runs the root command. Pass its error to ExitCode.
func Execute() error {
	strictGroups(rootCmd) // here, not in init: other files' init add subcommands
	stdout := os.Stdout
	c, err := rootCmd.ExecuteC()
	captured := os.Stdout
	os.Stdout = stdout
	if err != nil {
		_, _ = fmt.Fprintf(rootCmd.ErrOrStderr(), "Error: %s\n", err)
		// Every command hook sets SilenceUsage once flags and args are valid:
		// still unset means cobra rejected the command line itself. Cobra
		// checks required and exclusive flags after the hooks, so ask again.
		if !c.SilenceUsage || c.ValidateRequiredFlags() != nil || c.ValidateFlagGroups() != nil {
			return usageError{err}
		}
		return err
	}
	s := app.Get()
	if !dryRun || s == nil || !s.Client.DryRun() {
		return nil
	}
	if len(s.Client.Planned()) == 0 {
		return replay(captured, stdout)
	}
	return app.PrintPlan(stdout)
}

// replay copies the output a --dry-run captured for a command that turned out
// to write nothing, such as a read.
func replay(captured, stdout *os.File) error {
	if captured == stdout {
		return nil
	}
	defer func() { _ = captured.Close() }()
	if _, err := captured.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err := io.Copy(stdout, captured)
	return err
}
