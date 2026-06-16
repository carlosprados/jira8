package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link OUTWARD-KEY INWARD-KEY",
	Short: "Link two issues (e.g. Relates, Blocks, Duplicate)",
	Long: "Create a link between two issues. The first key is the subject of the " +
		"relationship and the second its object (\"OUTWARD <phrase> INWARD\", e.g. " +
		"\"ESA-207 blocks ESA-214\"). For symmetric types like Relates the order is irrelevant.\n\n" +
		"Run 'jira8 issue link-types' to see the available type names.",
	Example: "  jira8 issue link ESA-207 ESA-214 --type \"Relates\"\n" +
		"  jira8 issue link ESA-9 ESA-10 --type \"Blocks\" --comment \"blocked until merge\"",
	Args: cobra.ExactArgs(2),
	RunE: runLink,
}

var linkTypesCmd = &cobra.Command{
	Use:     "link-types",
	Short:   "List the available issue link types",
	Example: "  jira8 issue link-types",
	Args:    cobra.NoArgs,
	RunE:    runLinkTypes,
}

func init() {
	linkCmd.Flags().String("type", "Relates", "Link type name (e.g. Relates, Blocks, Duplicate)")
	linkCmd.Flags().String("comment", "", "Optional comment to add alongside the link")
}

func runLink(cmd *cobra.Command, args []string) error {
	a := app.Get()
	outward, inward := args[0], args[1]
	typeName, _ := cmd.Flags().GetString("type")
	comment, _ := cmd.Flags().GetString("comment")

	// Validate the link type against the configured ones so the user gets a
	// helpful error listing the valid names (same UX as 'issue transition').
	types, err := a.Client.GetIssueLinkTypes(context.Background())
	if err != nil {
		return err
	}
	match := matchLinkType(types, typeName)
	if match == nil {
		return fmt.Errorf("link type %q not found; available: %s", typeName, strings.Join(linkTypeNames(types), ", "))
	}

	req := &models.IssueLinkRequest{
		Type:         models.IssueLinkTypeRef{Name: match.Name},
		OutwardIssue: models.LinkedIssueRef{Key: outward},
		InwardIssue:  models.LinkedIssueRef{Key: inward},
	}
	if comment != "" {
		req.Comment = &models.IssueLinkComment{Body: comment}
	}
	if err := a.Client.LinkIssues(context.Background(), req); err != nil {
		return err
	}

	fmt.Printf("Linked %s %s %s\n", outward, match.Outward, inward)
	return nil
}

func runLinkTypes(cmd *cobra.Command, args []string) error {
	a := app.Get()
	types, err := a.Client.GetIssueLinkTypes(context.Background())
	if err != nil {
		return err
	}

	if a.Output == "json" {
		data, err := json.MarshalIndent(types, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(types) == 0 {
		fmt.Println("No issue link types available.")
		return nil
	}
	fmt.Printf("%s  %s  %s\n",
		headerStyle.Render(fmt.Sprintf("%-16s", "NAME")),
		headerStyle.Render(fmt.Sprintf("%-24s", "OUTWARD")),
		headerStyle.Render("INWARD"),
	)
	for _, t := range types {
		fmt.Printf("%-16s  %-24s  %s\n", t.Name, t.Outward, t.Inward)
	}
	return nil
}

// matchLinkType returns the link type whose name matches (case-insensitive), or nil.
func matchLinkType(types []models.IssueLinkType, name string) *models.IssueLinkType {
	for i := range types {
		if strings.EqualFold(types[i].Name, name) {
			return &types[i]
		}
	}
	return nil
}

func linkTypeNames(types []models.IssueLinkType) []string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.Name
	}
	return names
}
