package app

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// PrintPlan reports the writes a --dry-run recorded instead of sending.
func PrintPlan(w io.Writer) error {
	planned := Get().Client.Planned()
	if Get().Output == "json" {
		data, err := json.MarshalIndent(map[string]any{"dry_run": true, "requests": planned}, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(data))
		return err
	}
	if len(planned) == 0 {
		_, err := fmt.Fprintln(w, "Dry run: nothing would be sent.")
		return err
	}
	_, _ = fmt.Fprintf(w, "Dry run: nothing was sent. %d request(s) would be:\n", len(planned))
	for _, p := range planned {
		_, _ = fmt.Fprintf(w, "\n%s %s\n", p.Method, p.Path)
		if len(p.Files) > 0 {
			_, _ = fmt.Fprintf(w, "files: %s\n", strings.Join(p.Files, ", "))
		}
		if p.Body != nil {
			data, err := json.MarshalIndent(p.Body, "", "  ")
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(w, string(data))
		}
	}
	return nil
}
