package issue

import (
	"fmt"
	"strings"

	"github.com/amplia/jira-cli/internal/models"
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
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
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

	// Header
	fmt.Printf("%s  %-10s  %-15s  %-10s  %-15s  %s\n",
		headerStyle.Render(fmt.Sprintf("%-12s", "KEY")),
		headerStyle.Render("TYPE"),
		headerStyle.Render("STATUS"),
		headerStyle.Render("PRIORITY"),
		headerStyle.Render("ASSIGNEE"),
		headerStyle.Render("SUMMARY"),
	)

	for _, issue := range issues {
		status := statusName(issue.Fields.Status)
		fmt.Printf("%s  %-10s  %-15s  %-10s  %-15s  %s\n",
			keyStyle.Render(fmt.Sprintf("%-12s", issue.Key)),
			truncate(typeName(issue.Fields.IssueType), 10),
			colorStatus(truncate(status, 15)),
			truncate(priorityName(issue.Fields.Priority), 10),
			truncate(userName(issue.Fields.Assignee), 15),
			truncate(issue.Fields.Summary, 60),
		)
	}

	fmt.Printf("\n%d issue(s)\n", len(issues))
}

func printIssueDetail(issue *models.Issue) {
	f := issue.Fields

	fmt.Println()
	fmt.Printf("%s  %s\n", keyStyle.Render(issue.Key), headerStyle.Render(f.Summary))
	fmt.Println(strings.Repeat("─", 60))

	printField("Type", typeName(f.IssueType))
	printField("Status", statusName(f.Status))
	printField("Priority", priorityName(f.Priority))
	printField("Assignee", userName(f.Assignee))
	printField("Reporter", userName(f.Reporter))

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
			for _, line := range strings.Split(c.Body, "\n") {
				fmt.Printf("  %s\n", line)
			}
		}
	}

	fmt.Println()
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
