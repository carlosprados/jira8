package app

import (
	"encoding/json"
	"fmt"
)

// OutputJSON marshals v as indented JSON and prints it to stdout. It is the
// single implementation behind every `if a.Output == "json"` branch in the
// CLI subcommands, so the JSON output format stays consistent across commands.
func OutputJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// Done reports a write that Jira answers without a body: the formatted text
// for humans, v with -o json, so scripts always get JSON when they ask for it.
func Done(v any, format string, args ...any) error {
	if Get().Output == "json" {
		return OutputJSON(v)
	}
	fmt.Printf(format+"\n", args...)
	return nil
}
