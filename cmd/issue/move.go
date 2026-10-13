package issue

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/client"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var moveCmd = &cobra.Command{
	Use:   "move ISSUE-KEY",
	Short: "Move an issue to a board column (and optionally to its top or bottom)",
	Long: `Moves the card as on the board: finds the transition, available now, that
leads to one of the column's statuses and runs it, then with --top or --bottom
ranks the card at that end of the column. Screen fields and a comment go as in
'issue transition'.

The column is a column of the board ('jira8 board view'), not a status. When
the project has several boards, pass --board (id or name). When the column can
be reached through several transitions, the error lists them: use
'issue transition --to' to pick one.`,
	Example: `  jira8 issue move ABC-123 --column "In Progress" --top
  jira8 issue move ABC-123 --column Done --field "Resolution=Fixed" --comment "Released in 2.4"
  jira8 issue move ABC-123 --column "Test / Pull Req" --board "Team Kanban"`,
	Args: cobra.ExactArgs(1),
	RunE: runMove,
}

func init() {
	moveCmd.Flags().String("column", "", "Board column to move the issue to (required)")
	moveCmd.Flags().String("board", "", "Board id or name (required when the project has several)")
	moveCmd.Flags().Bool("top", false, "Then rank it at the top of the column")
	moveCmd.Flags().Bool("bottom", false, "Then rank it at the bottom of the column")
	addTransitionFlags(moveCmd)
	_ = moveCmd.MarkFlagRequired("column")
	moveCmd.MarkFlagsMutuallyExclusive("top", "bottom")
}

func runMove(cmd *cobra.Command, args []string) error {
	a := app.Get()
	ctx := context.Background()
	key := args[0]
	column, _ := cmd.Flags().GetString("column")
	boardRef, _ := cmd.Flags().GetString("board")

	issue, err := a.Client.GetIssue(ctx, key)
	if err != nil {
		return err
	}
	project := ""
	if issue.Fields.Project != nil {
		project = issue.Fields.Project.Key
	}
	board, col, err := a.Client.ResolveColumn(ctx, project, boardRef, column)
	if err != nil {
		return err
	}
	statuses := make([]string, len(col.Statuses))
	for i, st := range col.Statuses {
		statuses[i] = st.ID
	}

	out := map[string]any{"key": key, "board": board.Name, "column": col.Name}
	status := issue.Fields.Status
	target := ""
	if status != nil && slices.Contains(statuses, status.ID) {
		out["moved"] = false
		out["status"] = status.Name
		target = status.ID
	} else {
		t, err := columnTransition(ctx, key, status, col, statuses)
		if err != nil {
			return err
		}
		res, _, err := performTransition(cmd, key, t)
		if err != nil {
			return err
		}
		for k, v := range res {
			out[k] = v
		}
		out["moved"] = true
		target = t.To.ID
	}

	pos := client.RankPosition("")
	if top, _ := cmd.Flags().GetBool("top"); top {
		pos = client.RankTop
	} else if bottom, _ := cmd.Flags().GetBool("bottom"); bottom {
		pos = client.RankBottom
	}
	if pos != "" {
		r, err := a.Client.RankIssuesRelative(ctx, client.RankRequest{
			Keys: []string{key}, Position: pos, Board: strconv.Itoa(board.ID), StatusID: target,
		})
		if err != nil {
			return fmt.Errorf("moved %s to %s but failed to rank it: %w", key, col.Name, err)
		}
		out["rank"] = string(pos)
		if r.Anchor != "" {
			out["anchor"] = r.Anchor
		}
	}

	text := fmt.Sprintf("Moved %s to %s", key, col.Name)
	if out["moved"] == false {
		text = fmt.Sprintf("%s is already in %s", key, col.Name)
	}
	if pos != "" {
		text += fmt.Sprintf(" (%s)", pos)
	}
	return app.Done(out, "%s", text)
}

// columnTransition picks the one transition, available now, that leads into
// the column. Several reachable statuses are ambiguous: the error lists them.
func columnTransition(ctx context.Context, key string, from *models.Status, col *models.BoardColumn, statuses []string) (*models.Transition, error) {
	transitions, err := app.Get().Client.GetTransitions(ctx, key)
	if err != nil {
		return nil, err
	}
	var hits []models.Transition
	for _, t := range transitions {
		if t.To != nil && slices.Contains(statuses, t.To.ID) {
			hits = append(hits, t)
		}
	}
	describe := func(ts []models.Transition) string {
		out := make([]string, len(ts))
		for i, t := range ts {
			to := "?"
			if t.To != nil {
				to = t.To.Name
			}
			out[i] = fmt.Sprintf("%q → %s", t.Name, to)
		}
		return strings.Join(out, ", ")
	}
	switch len(hits) {
	case 1:
		return &hits[0], nil
	case 0:
		return nil, fmt.Errorf("%s (%s) cannot move to column %q now: no available transition leads to its statuses; available: %s",
			key, statusName(from), col.Name, describe(transitions))
	}
	return nil, fmt.Errorf("column %q can be reached from %s through several transitions: %s; use 'issue transition --to'",
		col.Name, statusName(from), describe(hits))
}
