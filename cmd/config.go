package cmd

import (
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect the configuration in use",
	// Reads the config without the root hook's checks: it must work precisely
	// when the config is broken.
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the effective configuration, secrets redacted",
	Long: `Shows the configuration jira8 would use: the file read, each value with its
source (flag, env, file) and the problems that stop it from working. Token
and password are never printed, only whether they are set.`,
	Example: "  jira8 config show\n  jira8 config show -o json",
	Args:    cobra.NoArgs,
	RunE:    runConfigShow,
}

// ConfigValue is one configuration key as config show reports it.
type ConfigValue struct {
	Value  string `json:"value,omitempty"`
	Set    bool   `json:"set"`
	Source string `json:"source,omitempty"`
}

func runConfigShow(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Read(configFile)
	if err != nil {
		return err
	}
	flag := func(name string) bool {
		f := cmd.Flags().Lookup(name)
		return f != nil && f.Changed
	}
	value := func(key, v string, secret bool) ConfigValue {
		cv := ConfigValue{Set: v != "", Source: config.Source(key, flag(key))}
		if !secret {
			cv.Value = v
		}
		return cv
	}
	values := map[string]ConfigValue{
		"url":      value("url", cfg.URL, false),
		"project":  value("project", cfg.Project, false),
		"user":     value("user", cfg.User, false),
		"token":    value("token", cfg.Token, true),
		"password": value("password", cfg.Password, true),
	}
	problems := []string{}
	if err := cfg.Validate(); err != nil {
		problems = append(problems, err.Error())
	}

	if viper.GetString("output") == "json" {
		return app.OutputJSON(map[string]any{
			"file":     config.File(),
			"auth":     cfg.Auth(),
			"values":   values,
			"problems": problems,
		})
	}
	file := config.File()
	if file == "" {
		file = "(none)"
	}
	fmt.Printf("file      %s\n", file)
	fmt.Printf("auth      %s\n", orNone(cfg.Auth()))
	for _, k := range []string{"url", "project", "user", "token", "password"} {
		v := values[k]
		shown := v.Value
		if v.Set && shown == "" {
			shown = "(set)"
		}
		if v.Source != "" {
			shown += "  [" + v.Source + "]"
		}
		fmt.Printf("%-9s %s\n", k, orNone(shown))
	}
	for _, p := range problems {
		fmt.Printf("problem   %s\n", p)
	}
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func init() {
	configCmd.AddCommand(configShowCmd)
}
