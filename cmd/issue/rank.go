package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/client"
	"github.com/spf13/cobra"
)

var rankCmd = &cobra.Command{
	Use:   "rank ISSUE-KEY... (--top | --bottom | --before KEY | --after KEY)",
	Short: "Reorder issues inside a board column",
	Long: "Change the vertical position of one or more issues on an Agile board.\n\n" +
		"Ranking moves an issue up or down within its column; it never changes the " +
		"issue's status. To move an issue to a different column use 'issue transition'.\n\n" +
		"--top and --bottom resolve the issue's own column on the board and move it to " +
		"the first or last position of that column. When the project has more than one " +
		"board, pass --board with an ID or name (the error lists the candidates).\n\n" +
		"Several keys can be ranked at once (up to 50); they keep their relative order.",
	Example: "  jira8 issue rank PHO-5050 --top\n" +
		"  jira8 issue rank PHO-5050 --bottom --board \"Phoenix Kanban\"\n" +
		"  jira8 issue rank PHO-5050 --before PHO-4941\n" +
		"  jira8 issue rank PHO-5050 PHO-5021 --top --board 110",
	Args: cobra.MinimumNArgs(1),
	RunE: runRank,
}

func init() {
	rankCmd.Flags().Bool("top", false, "Move to the first position of the issue's column")
	rankCmd.Flags().Bool("bottom", false, "Move to the last position of the issue's column")
	rankCmd.Flags().String("before", "", "Move immediately above this issue key")
	rankCmd.Flags().String("after", "", "Move immediately below this issue key")
	rankCmd.Flags().String("board", "", "Board ID or name; only needed by --top/--bottom when the project has several boards")

	rankCmd.MarkFlagsMutuallyExclusive("top", "bottom", "before", "after")
	rankCmd.MarkFlagsOneRequired("top", "bottom", "before", "after")
}

func runRank(cmd *cobra.Command, args []string) error {
	a := app.Get()
	top, _ := cmd.Flags().GetBool("top")
	bottom, _ := cmd.Flags().GetBool("bottom")
	before, _ := cmd.Flags().GetString("before")
	after, _ := cmd.Flags().GetString("after")
	board, _ := cmd.Flags().GetString("board")

	req := client.RankRequest{Keys: args, Board: board}
	switch {
	case top:
		req.Position = client.RankTop
	case bottom:
		req.Position = client.RankBottom
	case before != "":
		req.Position, req.Anchor = client.RankBefore, before
	case after != "":
		req.Position, req.Anchor = client.RankAfter, after
	}

	result, err := a.Client.RankIssuesRelative(context.Background(), req)
	if err != nil {
		return err
	}

	if a.Output == "json" {
		return app.OutputJSON(result)
	}

	fmt.Println(describeRank(result))
	return nil
}

// describeRank renders a human-readable summary of a completed rank operation,
// including the anchor that was resolved so the user can verify the move.
func describeRank(r *client.RankResult) string {
	issues := strings.Join(r.Issues, ", ")
	edge := "top"
	if r.Position == client.RankBottom {
		edge = "bottom"
	}

	switch r.Position {
	case client.RankTop, client.RankBottom:
		where := fmt.Sprintf("the %s of %q", edge, r.Column)
		if r.Board != nil {
			where += fmt.Sprintf(" (board %q)", r.Board.Name)
		}
		if r.NoOp {
			return fmt.Sprintf("%s already at %s; nothing to do", issues, where)
		}
		relation := "above"
		if r.Position == client.RankBottom {
			relation = "below"
		}
		return fmt.Sprintf("Ranked %s to %s, %s %s", issues, where, relation, r.Anchor)
	case client.RankBefore:
		return fmt.Sprintf("Ranked %s above %s", issues, r.Anchor)
	default:
		return fmt.Sprintf("Ranked %s below %s", issues, r.Anchor)
	}
}
