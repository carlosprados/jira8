package issue

import (
	"fmt"
	"strings"

	"github.com/amplia/jira8/internal/models"
	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	keyStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	labelStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
	statusDone  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	statusWIP   = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	statusTodo  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

func colorStatus(status string) string {
	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "done") || strings.Contains(lower, "closed") || strings.Contains(lower, "resolved"):
		return statusDone.Render(status)
	case strings.Contains(lower, "progress") || strings.Contains(lower, "review"):
		return statusWIP.Render(status)
	default:
		return statusTodo.Render(status)
	}
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// pad truncates or pads s to exactly width runes.
func pad(s string, width int) string {
	runes := []rune(s)
	if len(runes) > width {
		return string(runes[:width-1]) + "…"
	}
	if len(runes) < width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	return s
}

func userName(u *models.User) string {
	if u == nil {
		return "-"
	}
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Name
}

func statusName(s *models.Status) string {
	if s == nil {
		return "-"
	}
	return s.Name
}

func typeName(t *models.IssueType) string {
	if t == nil {
		return "-"
	}
	return t.Name
}

func priorityName(p *models.Priority) string {
	if p == nil {
		return "-"
	}
	return p.Name
}

func printIssueTable(issues []models.Issue) {
	if len(issues) == 0 {
		fmt.Println("No issues found.")
		return
	}

	// Column widths
	const (
		wKey      = 12
		wType     = 10
		wStatus   = 15
		wPriority = 10
		wAssignee = 20
		wSummary  = 55
		sep       = "  "
	)

	// Header
	fmt.Println(
		headerStyle.Render(pad("KEY", wKey)) + sep +
			headerStyle.Render(pad("TYPE", wType)) + sep +
			headerStyle.Render(pad("STATUS", wStatus)) + sep +
			headerStyle.Render(pad("PRIORITY", wPriority)) + sep +
			headerStyle.Render(pad("ASSIGNEE", wAssignee)) + sep +
			headerStyle.Render("SUMMARY"),
	)

	for _, issue := range issues {
		status := statusName(issue.Fields.Status)
		fmt.Println(
			keyStyle.Render(pad(issue.Key, wKey)) + sep +
				pad(typeName(issue.Fields.IssueType), wType) + sep +
				colorStatus(pad(status, wStatus)) + sep +
				pad(priorityName(issue.Fields.Priority), wPriority) + sep +
				pad(userName(issue.Fields.Assignee), wAssignee) + sep +
				truncate(issue.Fields.Summary, wSummary),
		)
	}

	fmt.Printf("\n%d issue(s)\n", len(issues))
}

// printIssueDetailWithEpic renders the issue, additionally showing Epic Name and
// Epic Link columns when the corresponding custom field IDs have been resolved.
// Pass empty strings to skip the Epic rendering.
func printIssueDetailWithEpic(issue *models.Issue, epicNameID, epicLinkID string) {
	f := issue.Fields

	fmt.Println()
	fmt.Printf("%s  %s\n", keyStyle.Render(issue.Key), headerStyle.Render(f.Summary))
	fmt.Println(strings.Repeat("─", 60))

	printField("Type", typeName(f.IssueType))
	printField("Status", statusName(f.Status))
	printField("Priority", priorityName(f.Priority))
	printField("Assignee", userName(f.Assignee))
	printField("Reporter", userName(f.Reporter))

	if epicNameID != "" {
		if name := f.CustomString(epicNameID); name != "" {
			printField("Epic Name", name)
		}
	}
	if epicLinkID != "" {
		if link := f.CustomString(epicLinkID); link != "" {
			printField("Epic", link)
		}
	}

	if f.Parent != nil {
		parentStr := f.Parent.Key
		if f.Parent.Fields != nil && f.Parent.Fields.Summary != "" {
			parentStr += " — " + f.Parent.Fields.Summary
		}
		printField("Parent", parentStr)
	}

	if f.Project != nil {
		printField("Project", fmt.Sprintf("%s (%s)", f.Project.Name, f.Project.Key))
	}

	if len(f.Labels) > 0 {
		printField("Labels", strings.Join(f.Labels, ", "))
	}
	if len(f.Components) > 0 {
		names := make([]string, len(f.Components))
		for i, c := range f.Components {
			names[i] = c.Name
		}
		printField("Components", strings.Join(names, ", "))
	}

	if f.Created != "" {
		printField("Created", f.Created)
	}
	if f.Updated != "" {
		printField("Updated", f.Updated)
	}

	if f.Description != "" {
		fmt.Println()
		fmt.Println(labelStyle.Render("Description:"))
		fmt.Println(f.Description)
	}

	if len(f.Attachment) > 0 {
		fmt.Println()
		fmt.Printf(labelStyle.Render("Attachments (%d):")+"\n", len(f.Attachment))
		for _, att := range f.Attachment {
			fmt.Printf("  %s  %s  %s\n",
				labelStyle.Render("#"+att.ID),
				att.Filename,
				labelStyle.Render(humanSize(att.Size)),
			)
		}
	}

	if f.Comment != nil && len(f.Comment.Comments) > 0 {
		fmt.Println()
		fmt.Printf(labelStyle.Render("Comments (%d):")+"\n", f.Comment.Total)
		comments := f.Comment.Comments
		// Show last 5 comments
		start := 0
		if len(comments) > 5 {
			start = len(comments) - 5
			fmt.Printf("  ... showing last 5 of %d comments\n", len(comments))
		}
		for _, c := range comments[start:] {
			fmt.Printf("\n  %s  %s\n", headerStyle.Render(userName(c.Author)), labelStyle.Render(c.Created))
			for line := range strings.SplitSeq(c.Body, "\n") {
				fmt.Printf("  %s\n", line)
			}
		}
	}

	fmt.Println()
}

// humanSize renders a byte count as a short human-readable string (e.g. "12.3 KB").
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func printField(label, value string) {
	fmt.Printf("  %s %s\n", labelStyle.Render(fmt.Sprintf("%-12s", label+":")), value)
}

func printTransitions(transitions []models.Transition) {
	if len(transitions) == 0 {
		fmt.Println("No transitions available.")
		return
	}

	fmt.Printf("%s  %-25s  %s\n",
		headerStyle.Render(fmt.Sprintf("%-6s", "ID")),
		headerStyle.Render("TRANSITION"),
		headerStyle.Render("TARGET STATUS"),
	)

	for _, t := range transitions {
		target := "-"
		if t.To != nil {
			target = t.To.Name
		}
		fmt.Printf("%-6s  %-25s  %s\n", t.ID, t.Name, target)
	}
}
