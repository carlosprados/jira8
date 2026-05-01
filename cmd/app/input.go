package app

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// ReadTextInput resolves a pair of flags: an inline string flag (e.g.
// "description") and a corresponding "<flag>-file" flag (e.g.
// "description-file"). The file flag accepts "-" to mean stdin, enabling
// pipelines like `cat foo.md | jira8 issue create --description-file -`.
//
// Returns the resolved text, plus a boolean indicating whether the user set
// either flag (useful for edit subcommands that only update fields the user
// actually touched). Errors when both flags are set, or when the file cannot
// be read.
//
// AI agent context: this helper exists because pasting long Markdown / Wiki
// payloads inline forces ugly bash escaping for backticks, dollars, quotes
// and embedded JSON. File / stdin input sidesteps the shell entirely.
func ReadTextInput(cmd *cobra.Command, inlineFlag, fileFlag string) (text string, set bool, err error) {
	inlineSet := cmd.Flags().Changed(inlineFlag)
	fileSet := cmd.Flags().Changed(fileFlag)

	if inlineSet && fileSet {
		return "", true, fmt.Errorf("--%s and --%s are mutually exclusive", inlineFlag, fileFlag)
	}

	if fileSet {
		path, _ := cmd.Flags().GetString(fileFlag)
		data, err := readFromPathOrStdin(path)
		if err != nil {
			return "", true, err
		}
		return string(data), true, nil
	}

	if inlineSet {
		v, _ := cmd.Flags().GetString(inlineFlag)
		return v, true, nil
	}

	v, _ := cmd.Flags().GetString(inlineFlag)
	return v, false, nil
}

// stdinReader is overridable so tests can feed synthetic input.
var stdinReader io.Reader = os.Stdin

func readFromPathOrStdin(path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(stdinReader)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return data, nil
}
