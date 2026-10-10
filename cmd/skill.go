package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amplia/jira8/internal/skill"
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Agent skill (SKILL.md) shipped with this binary",
	// No Jira config needed: overrides the root hook for the whole subtree.
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
	Long: `jira8 ships with a skill document for coding agents (Claude Code and anything
else that reads a skills directory). It covers the commands, the JSON output,
Markdown conversion and the Jira Server 8 pitfalls, so an agent uses the CLI
correctly instead of guessing.

The document is embedded in this binary, so it always matches it:

  jira8 skill install             into ~/.claude/skills/jira8/SKILL.md
  jira8 skill install --project   into ./.claude/skills/jira8/SKILL.md
  jira8 skill show                print it
  jira8 skill path                where install would write

Re-run 'jira8 skill install --force' after upgrading the binary.`,
}

var skillShowCmd = &cobra.Command{
	Use:     "show",
	Short:   "Print the embedded skill document",
	Example: "  jira8 skill show | less",
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := os.Stdout.Write(skill.Content)
		return err
	},
}

var skillPathCmd = &cobra.Command{
	Use:     "path",
	Short:   "Print where the skill would be installed",
	Example: "  jira8 skill path\n  jira8 skill path --project",
	RunE: func(cmd *cobra.Command, _ []string) error {
		p, err := skillPath(cmd)
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	},
}

var skillInstallCmd = &cobra.Command{
	Use:     "install",
	Short:   "Write the embedded skill where an agent will find it",
	Example: "  jira8 skill install\n  jira8 skill install --force      # after upgrading jira8\n  jira8 skill install --project    # only for the current repo",
	RunE: func(cmd *cobra.Command, _ []string) error {
		p, err := skillPath(cmd)
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		if cur, err := os.ReadFile(p); err == nil {
			if bytes.Equal(cur, skill.Content) {
				fmt.Println("Already up to date:", p)
				return nil
			}
			if !force {
				return fmt.Errorf("%s exists and differs from this version: use --force to overwrite", p)
			}
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, skill.Content, 0o644); err != nil {
			return err
		}
		fmt.Println("Installed", p)
		return nil
	},
}

func skillPath(cmd *cobra.Command) (string, error) {
	var base string
	if project, _ := cmd.Flags().GetBool("project"); project {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		base = wd
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = home
	}
	return filepath.Join(base, ".claude", "skills", skill.Name, "SKILL.md"), nil
}

func init() {
	for _, c := range []*cobra.Command{skillInstallCmd, skillPathCmd} {
		c.Flags().Bool("project", false, "Use ./.claude/skills instead of ~/.claude/skills")
	}
	skillInstallCmd.Flags().Bool("force", false, "Overwrite an existing, different SKILL.md")
	skillCmd.AddCommand(skillInstallCmd, skillShowCmd, skillPathCmd)
}
