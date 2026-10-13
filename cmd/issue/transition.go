package issue

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var transitionCmd = &cobra.Command{
	Use:   "transition ISSUE-KEY",
	Short: "Transition an issue to a new status",
	Example: `  jira8 issue transition ABC-123 --to "Done"
  jira8 issue transition ABC-123 --to "Done" --field "Resolution=Won't Fix" --comment "Duplicate of ABC-99"
  jira8 issue transitions ABC-123        # which transitions exist now and what their screens ask`,
	Args: cobra.ExactArgs(1),
	RunE: runTransition,
}

var transitionsCmd = &cobra.Command{
	Use:     "transitions ISSUE-KEY",
	Short:   "List available transitions for an issue",
	Example: "  jira8 issue transitions ABC-123",
	Args:    cobra.ExactArgs(1),
	RunE:    runTransitions,
}

func init() {
	transitionCmd.Flags().String("to", "", "Target transition name (required)")
	addTransitionFlags(transitionCmd)
	_ = transitionCmd.MarkFlagRequired("to")
}

func runTransition(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]
	to, _ := cmd.Flags().GetString("to")

	transitions, err := a.Client.GetTransitions(context.Background(), key)
	if err != nil {
		return err
	}

	transitionName := func(t models.Transition) string { return t.Name }
	match := app.MatchByName(transitions, to, transitionName)
	if match == nil {
		return fmt.Errorf("transition %q not found; available: %s", to, strings.Join(app.Names(transitions, transitionName), ", "))
	}

	out, uploaded, err := performTransition(cmd, key, match)
	if err != nil {
		return err
	}
	if err := app.Done(out, "Transitioned %s → %s", key, out["status"]); err != nil {
		return err
	}
	if a.Output != "json" {
		app.PrintUploadedImages(uploaded)
	}
	return nil
}

// addTransitionFlags registers the flags performTransition reads.
func addTransitionFlags(c *cobra.Command) {
	c.Flags().StringArray("field", nil, `Set a field on the transition screen, e.g. "Resolution=Fixed" (repeatable; see 'issue transitions')`)
	c.Flags().String("comment", "", "Add a comment after the transition")
	c.Flags().String("comment-file", "", "Read the comment from file (use - for stdin)")
	c.Flags().Bool("markdown", false, "Treat --comment as Markdown and convert it to Jira Wiki Markup; local images (![alt](path)) are attached to the issue")
}

// performTransition runs transition t with the screen fields and comment of
// the command's flags. It returns the JSON answer and the images uploaded.
func performTransition(cmd *cobra.Command, key string, t *models.Transition) (map[string]any, []models.Attachment, error) {
	a := app.Get()
	fields, err := transitionFields(cmd, t)
	if err != nil {
		return nil, nil, err
	}
	comment, err := app.ReadBody(cmd, "comment", "comment-file")
	if err != nil {
		return nil, nil, err
	}
	uploaded, err := app.UploadImages(context.Background(), key, comment.Images)
	if err != nil {
		return nil, nil, err
	}

	req := &models.TransitionRequest{Transition: models.TransitionRef{ID: t.ID}, Fields: fields}
	if err := a.Client.DoTransition(context.Background(), key, req); err != nil {
		return nil, nil, err
	}

	target := t.Name
	if t.To != nil {
		target = t.To.Name
	}
	out := map[string]any{"key": key, "transition": t.Name, "status": target}
	if len(fields) > 0 {
		out["updated"] = app.UpdatedFields(fields, nil)
	}
	if len(uploaded) > 0 {
		out["attachments"] = uploaded
	}
	// Sent apart: Jira silently drops a comment inside a transition that has
	// no screen, and a separate call also gives back the comment id.
	if comment.Set && comment.Text != "" {
		c, err := a.Client.AddComment(context.Background(), key, &models.AddCommentRequest{Body: comment.Text})
		if err != nil {
			return nil, nil, fmt.Errorf("transitioned %s to %s but failed to add the comment: %w", key, target, err)
		}
		out["comment"] = c.ID
	}
	return out, uploaded, nil
}

// transitionFields resolves --field against the transition screen and checks
// that every required field of that screen is given, before anything is sent.
func transitionFields(cmd *cobra.Command, t *models.Transition) (map[string]any, error) {
	var fields map[string]any
	if cmd.Flags().Changed("field") {
		if len(t.Fields) == 0 {
			return nil, fmt.Errorf("transition %q has no screen: it takes no --field (use 'issue edit' before or after)", t.Name)
		}
		var err error
		if fields, err = app.FieldValues(context.Background(), cmd, t.Fields); err != nil {
			return nil, err
		}
	}
	for _, id := range slices.Sorted(maps.Keys(t.Fields)) {
		f := t.Fields[id]
		if _, ok := fields[id]; ok || !f.Required {
			continue
		}
		hint := ""
		if len(f.AllowedValues) > 0 {
			hint = "; allowed: " + strings.Join(app.Names(f.AllowedValues, models.AllowedValue.Label), ", ")
		}
		return nil, fmt.Errorf("transition %q requires %s: pass --field \"%s=…\"%s", t.Name, f.Name, f.Name, hint)
	}
	return fields, nil
}

func runTransitions(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	transitions, err := a.Client.GetTransitions(context.Background(), key)
	if err != nil {
		return err
	}

	if a.Output == "json" {
		return app.OutputJSON(transitions)
	}

	printTransitions(transitions)
	return nil
}
