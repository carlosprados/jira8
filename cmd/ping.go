package cmd

import (
	"context"
	"time"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Check that Jira answers: version, base URL and latency",
	Long: `Calls /serverInfo, which Jira answers without credentials: it checks the
network and the URL, not the login (use 'jira8 user me' for that).`,
	Example: "  jira8 ping\n  jira8 ping -o json",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		start := time.Now()
		info, err := app.Get().Client.ServerInfo(context.Background())
		if err != nil {
			return err
		}
		ms := time.Since(start).Milliseconds()
		return app.Done(map[string]any{
			"baseUrl": info.BaseURL, "version": info.Version, "deploymentType": info.DeploymentType,
			"serverTitle": info.ServerTitle, "ms": ms,
		}, "%s: Jira %s (%s) at %s, %d ms", info.ServerTitle, info.Version, info.DeploymentType, info.BaseURL, ms)
	},
}
