package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open ISSUE-KEY",
	Short: "Open an issue in the browser",
	Long: `Opens {url}/browse/KEY in the default browser. With -o json, or where there
is no browser to open (no display, no launcher), it prints the URL instead.`,
	Example: "  jira8 open ABC-123\n  jira8 open ABC-123 -o json",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a := app.Get()
		key := args[0]
		url := a.Client.BrowseURL(key)
		if a.Output == "json" {
			return app.OutputJSON(map[string]string{"key": key, "url": url})
		}
		launcher := browserCommand(url)
		if launcher == nil || launcher.Start() != nil {
			fmt.Println(url)
			return nil
		}
		fmt.Printf("Opening %s\n", url)
		return nil
	},
}

// browserCommand returns the command that opens url in the default browser,
// or nil when there is none to run.
func browserCommand(url string) *exec.Cmd {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return nil
		}
		name = "xdg-open"
	}
	if _, err := exec.LookPath(name); err != nil {
		return nil
	}
	return exec.Command(name, append(args, url)...)
}
