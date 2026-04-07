package cmd

import (
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/cmd/issue"
	"github.com/amplia/jira8/cmd/project"
	"github.com/amplia/jira8/internal/client"
	"github.com/amplia/jira8/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configFile string

var rootCmd = &cobra.Command{
	Use:   "jira8",
	Short: "CLI for Jira Server 8",
	Long:  "A command-line interface for interacting with Jira Server 8 REST API.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}

		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		app.Set(&app.State{
			Config: cfg,
			Client: client.New(cfg),
			Output: viper.GetString("output"),
		})
		return nil
	},
	SilenceUsage:  false,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default ~/.jira.yaml)")
	rootCmd.PersistentFlags().String("url", "", "Jira server URL")
	rootCmd.PersistentFlags().String("token", "", "Personal access token")
	rootCmd.PersistentFlags().String("project", "", "Default project key")
	rootCmd.PersistentFlags().StringP("output", "o", "text", "Output format: text or json")

	_ = viper.BindPFlag("url", rootCmd.PersistentFlags().Lookup("url"))
	_ = viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token"))
	_ = viper.BindPFlag("project", rootCmd.PersistentFlags().Lookup("project"))
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	rootCmd.AddCommand(issue.IssueCmd)
	rootCmd.AddCommand(project.ProjectCmd)
	rootCmd.AddCommand(mcpCmd)
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(rootCmd.ErrOrStderr(), "Error: %s\n", err)
		return err
	}
	return nil
}
