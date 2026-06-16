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
