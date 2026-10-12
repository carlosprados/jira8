package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/markup"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var markupCmd = &cobra.Command{
	Use:   "markup",
	Short: "Convert between Markdown and Jira Wiki Markup, offline",
	Long: `Runs the same conversion as --markdown, without Jira: see what a Markdown
text will look like as Wiki Markup before sending it, or read Wiki Markup as
Markdown. Input is a file, or stdin when the file is "-" or missing.`,
	// No Jira config needed: overrides the root hook for the whole subtree.
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return nil
	},
}

var md2wikiCmd = &cobra.Command{
	Use:     "md2wiki [FILE]",
	Short:   "Convert Markdown to Wiki Markup",
	Example: "  jira8 markup md2wiki spec.md\n  echo '**bold** and `code`' | jira8 markup md2wiki",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return convert(args, "wiki", markup.MarkdownToWiki)
	},
}

var wiki2mdCmd = &cobra.Command{
	Use:     "wiki2md [FILE]",
	Short:   "Convert Wiki Markup to Markdown",
	Example: "  jira8 markup wiki2md description.txt\n  echo 'h2. Title' | jira8 markup wiki2md",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return convert(args, "markdown", markup.WikiToMarkdown)
	},
}

func convert(args []string, key string, fn func(string) string) error {
	var (
		data []byte
		err  error
	)
	if len(args) == 0 || args[0] == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(args[0])
	}
	if err != nil {
		return err
	}
	out := fn(string(data))
	if viper.GetString("output") == "json" {
		return app.OutputJSON(map[string]string{key: out})
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	fmt.Print(out)
	return nil
}

func init() {
	markupCmd.AddCommand(md2wikiCmd, wiki2mdCmd)
}
