// Package markup converts a (small) subset of CommonMark-style Markdown into
// Jira Server 8 Wiki Markup, the format expected by `description`, comment
// `body`, and worklog `comment` fields.
//
// Conversion is line-based: headings, lists, blockquotes, tables, and fenced
// code blocks are detected per-line. Inline transformations (bold, italic,
// strikethrough, inline code, links) are then applied to lines that are not
// inside a fenced code block.
//
// AI agent context: this is an opt-in conversion. Callers that already produce
// Wiki Markup should not pass the input through MarkdownToWiki — the converter
// is not idempotent (e.g. a Wiki `*bold*` looks like Markdown italic and would
// be wrapped in `_..._`).
package markup

import (
	"fmt"
	"regexp"
	"strings"
)

// MarkdownToWiki converts a Markdown string into Jira Server 8 Wiki Markup.
// Empty input returns the empty string unchanged.
func MarkdownToWiki(s string) string {
	if s == "" {
		return ""
	}

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))

	inFence := false
	inTable := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if m := fenceLineRe.FindStringSubmatch(line); m != nil {
			if !inFence {
				inFence = true
				lang := strings.TrimSpace(m[1])
				if lang != "" {
					out = append(out, "{code:"+lang+"}")
				} else {
					out = append(out, "{code}")
				}
			} else {
				inFence = false
				out = append(out, "{code}")
			}
			continue
		}
		if inFence {
			out = append(out, line)
			continue
		}

		if !inTable && isTableRow(line) && i+1 < len(lines) && isTableSeparator(lines[i+1]) {
			cells := splitTableRow(line)
			for j, c := range cells {
				cells[j] = transformInline(c)
			}
			out = append(out, "||"+strings.Join(cells, "||")+"||")
			i++ // skip separator row
			inTable = true
			continue
		}
		if inTable {
			if isTableRow(line) {
				cells := splitTableRow(line)
				for j, c := range cells {
					cells[j] = transformInline(c)
				}
				out = append(out, "|"+strings.Join(cells, "|")+"|")
				continue
			}
			inTable = false
		}

		if isHorizontalRule(line) {
			out = append(out, "----")
			continue
		}

		if rest, ok := strings.CutPrefix(line, "> "); ok {
			out = append(out, "bq. "+transformInline(rest))
			continue
		}
		if strings.TrimRight(line, " \t") == ">" {
			out = append(out, "bq. ")
			continue
		}

		if m := headingRe.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			out = append(out, fmt.Sprintf("h%d. %s", level, transformInline(m[2])))
			continue
		}

		if m := bulletListRe.FindStringSubmatch(line); m != nil {
			depth := indentDepth(m[1])
			out = append(out, strings.Repeat("*", depth)+" "+transformInline(m[2]))
			continue
		}
		if m := orderedListRe.FindStringSubmatch(line); m != nil {
			depth := indentDepth(m[1])
			out = append(out, strings.Repeat("#", depth)+" "+transformInline(m[2]))
			continue
		}

		out = append(out, transformInline(line))
	}

	return strings.Join(out, "\n")
}

// indentDepth maps a leading-whitespace string to a 1-based list depth.
// Two spaces (or one tab) per nesting level — the convention emitted by most
// LLMs and what `prettier --prose-wrap` uses by default.
func indentDepth(indent string) int {
	spaces := 0
	for _, r := range indent {
		switch r {
		case ' ':
			spaces++
		case '\t':
			spaces += 2
		}
	}
	return spaces/2 + 1
}

var (
	fenceLineRe   = regexp.MustCompile("^```([A-Za-z0-9_+\\-]*)\\s*$")
	headingRe     = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	bulletListRe  = regexp.MustCompile(`^([ \t]*)[-*+]\s+(.*)$`)
	orderedListRe = regexp.MustCompile(`^([ \t]*)\d+\.\s+(.*)$`)
	tableSeparatorCellRe = regexp.MustCompile(`^:?-{3,}:?$`)

	inlineCodeRe  = regexp.MustCompile("`([^`\n]+)`")
	boldStarRe    = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
	boldUnderRe   = regexp.MustCompile(`__([^_\n]+)__`)
	italicStarRe  = regexp.MustCompile(`\*([^*\n]+)\*`)
	italicUnderRe = regexp.MustCompile(`(^|\W)_([^_\n]+)_(\W|$)`)
	strikeRe      = regexp.MustCompile(`~~([^~\n]+)~~`)
	linkRe        = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)

	codePlaceholderRe = regexp.MustCompile(`\x00C(\d+)\x00`)
	boldPlaceholderRe = regexp.MustCompile(`\x00B(\d+)\x00`)
)

// transformInline applies inline conversions to a single line of text. Inline
// code spans are extracted to placeholders first so their content is not
// re-processed; bold spans are also placeholdered so the italic pass can use
// simple regexes without worrying about `**bold**` vs `*italic*` ambiguity.
func transformInline(s string) string {
	if s == "" {
		return s
	}

	var codes []string
	s = inlineCodeRe.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[1 : len(m)-1]
		idx := len(codes)
		codes = append(codes, inner)
		return fmt.Sprintf("\x00C%d\x00", idx)
	})

	var bolds []string
	stash := func(m string, marker int) string {
		inner := m[marker : len(m)-marker]
		idx := len(bolds)
		bolds = append(bolds, inner)
		return fmt.Sprintf("\x00B%d\x00", idx)
	}
	s = boldStarRe.ReplaceAllStringFunc(s, func(m string) string { return stash(m, 2) })
	s = boldUnderRe.ReplaceAllStringFunc(s, func(m string) string { return stash(m, 2) })

	s = italicStarRe.ReplaceAllString(s, "_${1}_")
	s = italicUnderRe.ReplaceAllString(s, "${1}_${2}_${3}")

	s = strikeRe.ReplaceAllString(s, "-${1}-")
	s = linkRe.ReplaceAllString(s, "[${1}|${2}]")

	s = boldPlaceholderRe.ReplaceAllStringFunc(s, func(m string) string {
		var idx int
		fmt.Sscanf(m, "\x00B%d\x00", &idx)
		return "*" + bolds[idx] + "*"
	})
	s = codePlaceholderRe.ReplaceAllStringFunc(s, func(m string) string {
		var idx int
		fmt.Sscanf(m, "\x00C%d\x00", &idx)
		return "{{" + codes[idx] + "}}"
	})

	return s
}

// isHorizontalRule reports whether the line is a Markdown thematic break:
// three or more `-`, `_`, or `*` of the same kind, optionally separated by
// spaces. Go's RE2 regexp engine has no backreferences, so this is a
// hand-rolled check rather than a regex.
func isHorizontalRule(line string) bool {
	t := strings.TrimSpace(line)
	if len(t) < 3 {
		return false
	}
	first := t[0]
	if first != '-' && first != '_' && first != '*' {
		return false
	}
	count := 0
	for i := 0; i < len(t); i++ {
		switch t[i] {
		case first:
			count++
		case ' ', '\t':
			// allowed separator between markers
		default:
			return false
		}
	}
	return count >= 3
}

func isTableRow(s string) bool {
	t := strings.TrimSpace(s)
	if !strings.Contains(t, "|") {
		return false
	}
	return strings.HasPrefix(t, "|") || strings.HasSuffix(t, "|")
}

func isTableSeparator(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	t = strings.TrimPrefix(t, "|")
	t = strings.TrimSuffix(t, "|")
	parts := strings.Split(t, "|")
	if len(parts) == 0 {
		return false
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !tableSeparatorCellRe.MatchString(p) {
			return false
		}
	}
	return true
}

func splitTableRow(s string) []string {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "|")
	t = strings.TrimSuffix(t, "|")
	parts := strings.Split(t, "|")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}
