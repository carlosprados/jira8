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
	"strconv"
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
	fenceLineRe          = regexp.MustCompile("^```([A-Za-z0-9_+\\-]*)\\s*$")
	headingRe            = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	bulletListRe         = regexp.MustCompile(`^([ \t]*)[-*+]\s+(.*)$`)
	orderedListRe        = regexp.MustCompile(`^([ \t]*)\d+\.\s+(.*)$`)
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
		_, _ = fmt.Sscanf(m, "\x00B%d\x00", &idx)
		return "*" + bolds[idx] + "*"
	})
	s = codePlaceholderRe.ReplaceAllStringFunc(s, func(m string) string {
		var idx int
		_, _ = fmt.Sscanf(m, "\x00C%d\x00", &idx)
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

// splitTableCells splits a table row on the given delimiter, dropping the
// leading/trailing delimiter and trimming each cell. It backs both the
// Markdown ("|") and Wiki header ("||") splitters.
func splitTableCells(s, delim string) []string {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, delim)
	t = strings.TrimSuffix(t, delim)
	parts := strings.Split(t, delim)
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

func splitTableRow(s string) []string {
	return splitTableCells(s, "|")
}

// WikiToMarkdown converts a Jira Server 8 Wiki Markup string into Markdown.
// Empty input returns the empty string unchanged. The conversion is
// best-effort: elements with no clean Markdown equivalent (emoticons, {toc},
// +underline+, …) are left literal; {color}, {panel} and {anchor} tags are
// dropped, keeping their text. The output never contains HTML.
//
// AI agent context: this is the inverse of MarkdownToWiki. Together they let a
// caller live in Markdown end-to-end (publish in Markdown, read back in
// Markdown). Round-trip on the supported subset is approximately lossless.
func WikiToMarkdown(s string) string {
	if s == "" {
		return ""
	}

	queue := strings.Split(s, "\n")
	out := make([]string, 0, len(queue))

	fence := "" // "code" or "noformat" while inside a block; the other tag is literal there
	inQuote := false
	inTable := false

	for len(queue) > 0 {
		line := queue[0]
		queue = queue[1:]

		// Jira lets block tags share a line with text ("$ ls {code}",
		// "{code:java}x = 1;"). Split them off so each tag stands alone.
		if parts := splitBlockTag(line, fence); parts != nil {
			queue = append(parts, queue...)
			continue
		}

		if fence != "" {
			if m := wikiBlockLineRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil && m[1] == fence {
				fence = ""
				out = append(out, "```")
				continue
			}
			out = append(out, line)
			continue
		}

		if m := wikiBlockLineRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			switch m[1] {
			case "code", "noformat":
				fence = m[1]
				lang := ""
				if m[1] == "code" {
					lang = codeLang(m[2])
				}
				out = append(out, "```"+lang)
			case "quote":
				inQuote = !inQuote
			}
			continue // {panel} tags are dropped, their content stays
		}

		if inQuote {
			out = append(out, "> "+transformInlineWiki(line))
			continue
		}

		if isWikiTableHeader(line) {
			cells := splitWikiTableHeader(line)
			for j, c := range cells {
				cells[j] = transformInlineWiki(c)
			}
			out = append(out, "| "+strings.Join(cells, " | ")+" |", tableSeparator(len(cells)))
			inTable = true
			continue
		}
		if isWikiTableRow(line) {
			cells := splitTableRow(line)
			if !inTable {
				// GFM needs a header row; Jira tables may have none.
				out = append(out, "|"+strings.Repeat("  |", len(cells)), tableSeparator(len(cells)))
				inTable = true
			}
			for j, c := range cells {
				cells[j] = transformInlineWiki(c)
			}
			out = append(out, "| "+strings.Join(cells, " | ")+" |")
			continue
		}
		inTable = false

		if wikiHRRe.MatchString(line) {
			out = append(out, "---")
			continue
		}

		if m := wikiHeadingRe.FindStringSubmatch(line); m != nil {
			level, _ := strconv.Atoi(m[1])
			out = append(out, strings.Repeat("#", level)+" "+transformInlineWiki(m[2]))
			continue
		}

		if rest, ok := strings.CutPrefix(line, "bq. "); ok {
			out = append(out, "> "+transformInlineWiki(rest))
			continue
		}
		if line == "bq." {
			out = append(out, ">")
			continue
		}

		if m := wikiBulletListRe.FindStringSubmatch(line); m != nil {
			depth := len(m[1])
			indent := strings.Repeat("  ", depth-1)
			out = append(out, indent+"- "+transformInlineWiki(m[2]))
			continue
		}
		if m := wikiNumberedListRe.FindStringSubmatch(line); m != nil {
			depth := len(m[1])
			indent := strings.Repeat("  ", depth-1)
			out = append(out, indent+"1. "+transformInlineWiki(m[2]))
			continue
		}

		out = append(out, transformInlineWiki(line))
	}

	return strings.Join(out, "\n")
}

// splitBlockTag returns line split around its first block tag ({code},
// {noformat}, {quote}, {panel}) when that tag shares the line with other text,
// or nil when there is nothing to split. Inside a fence only the tag that
// closes it counts. Tags that are part of {{inline code}} are ignored.
func splitBlockTag(line, fence string) []string {
	for _, loc := range wikiBlockTagRe.FindAllStringSubmatchIndex(line, -1) {
		start, end := loc[0], loc[1]
		if (start > 0 && line[start-1] == '{') || (end < len(line) && line[end] == '}') {
			continue
		}
		if fence != "" && line[loc[2]:loc[3]] != fence {
			continue
		}
		before := strings.TrimRight(line[:start], " \t")
		after := strings.TrimLeft(line[end:], " \t")
		if before == "" && after == "" {
			return nil
		}
		var parts []string
		if before != "" {
			parts = append(parts, before)
		}
		parts = append(parts, line[start:end])
		if after != "" {
			parts = append(parts, after)
		}
		return parts
	}
	return nil
}

// codeLang extracts the language from {code} parameters: "java",
// "title=Foo.java|java", "language=java". Unknown shapes yield "".
func codeLang(params string) string {
	for p := range strings.SplitSeq(params, "|") {
		p = strings.TrimSpace(p)
		if k, v, ok := strings.Cut(p, "="); ok {
			if k == "language" || k == "lang" {
				p = v
			} else {
				continue
			}
		}
		if wikiLangRe.MatchString(p) {
			return p
		}
	}
	return ""
}

func tableSeparator(n int) string {
	return "|" + strings.Repeat(" --- |", n)
}

var (
	wikiBlockTagRe     = regexp.MustCompile(`\{(code|noformat|quote|panel)(?::([^}]*))?\}`)
	wikiBlockLineRe    = regexp.MustCompile(`^` + wikiBlockTagRe.String() + `$`)
	wikiLangRe         = regexp.MustCompile(`^[A-Za-z0-9_+\-]+$`)
	wikiHeadingRe      = regexp.MustCompile(`^h([1-6])\.\s+(.*)$`)
	wikiBulletListRe   = regexp.MustCompile(`^(\*+)\s+(.*)$`)
	wikiNumberedListRe = regexp.MustCompile(`^(#+)\s+(.*)$`)
	wikiHRRe           = regexp.MustCompile(`^-{4,}\s*$`)

	wikiBoldRe     = regexp.MustCompile(`(^|[^\w*])\*([^*\n]+)\*([^\w*]|$)`)
	wikiItalicRe   = regexp.MustCompile(`(^|[^\w_])_([^_\n]+)_([^\w_]|$)`)
	wikiStrikeRe   = regexp.MustCompile(`(^|[^\w-])-(\S[^-\n]*\S|\S)-([^\w-]|$)`)
	wikiLinkRe     = regexp.MustCompile(`\[([^\]|\n]+)\|([^\]\n]+)\]`)
	wikiImageRe    = regexp.MustCompile(`!([^!\s|]+)(?:\|[^!\n]*)?!`)
	wikiMentionRe  = regexp.MustCompile(`\[~([^\]\s]+)\]`)
	wikiBareURLRe  = regexp.MustCompile(`\[(https?://[^\]\s|]+)\]`)
	wikiIssueRefRe = regexp.MustCompile(`\[([A-Z][A-Z0-9_]+-\d+)\]`)
	wikiDroppedRe  = regexp.MustCompile(`\{(?:color(?::[^}]*)?|anchor(?::[^}]*)?)\}`)

	wikiCodePlaceholderRe = regexp.MustCompile(`\x00W(\d+)\x00`)
	wikiBoldPlaceholderRe = regexp.MustCompile(`\x00X(\d+)\x00`)
)

// transformInlineWiki applies inline conversions to a single Wiki line.
// Inline code, links, mentions and images are converted first and parked
// behind placeholders, so the bold/italic/strike passes cannot touch their
// content (an "_" in a URL or a "*" in code). Bold spans are placeholdered too
// so the italic pass can use simple regexes without ambiguity.
func transformInlineWiki(s string) string {
	if s == "" {
		return s
	}

	var kept []string
	keep := func(md string) string {
		kept = append(kept, md)
		return fmt.Sprintf("\x00W%d\x00", len(kept)-1)
	}

	s = replaceInlineCode(s, func(code string) string {
		if strings.Contains(code, "`") {
			return keep("`` " + code + " ``")
		}
		return keep("`" + code + "`")
	})

	s = wikiDroppedRe.ReplaceAllString(s, "")
	s = wikiMentionRe.ReplaceAllStringFunc(s, func(m string) string {
		return keep("@" + wikiMentionRe.FindStringSubmatch(m)[1])
	})
	s = wikiBareURLRe.ReplaceAllStringFunc(s, func(m string) string {
		return keep("<" + wikiBareURLRe.FindStringSubmatch(m)[1] + ">")
	})
	s = wikiIssueRefRe.ReplaceAllString(s, "${1}")
	s = wikiLinkRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := wikiLinkRe.FindStringSubmatch(m)
		return keep("[" + sub[1] + "](" + sub[2] + ")")
	})
	s = wikiImageRe.ReplaceAllStringFunc(s, func(m string) string {
		name := wikiImageRe.FindStringSubmatch(m)[1]
		return keep("![" + name + "](" + name + ")")
	})

	var bolds []string
	s = wikiBoldRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := wikiBoldRe.FindStringSubmatch(m)
		idx := len(bolds)
		bolds = append(bolds, sub[2])
		return sub[1] + fmt.Sprintf("\x00X%d\x00", idx) + sub[3]
	})

	s = wikiItalicRe.ReplaceAllString(s, "${1}*${2}*${3}")
	s = wikiStrikeRe.ReplaceAllString(s, "${1}~~${2}~~${3}")

	s = wikiBoldPlaceholderRe.ReplaceAllStringFunc(s, func(m string) string {
		var idx int
		_, _ = fmt.Sscanf(m, "\x00X%d\x00", &idx)
		return "**" + bolds[idx] + "**"
	})
	// Kept spans can nest (code inside link text), hence the loop.
	for wikiCodePlaceholderRe.MatchString(s) {
		s = wikiCodePlaceholderRe.ReplaceAllStringFunc(s, func(m string) string {
			var idx int
			_, _ = fmt.Sscanf(m, "\x00W%d\x00", &idx)
			return kept[idx]
		})
	}

	return s
}

// replaceInlineCode replaces each {{code}} span with fn(code). A span ends at
// the first "}}", so {{GET /devices/{id}/x}} keeps its inner braces; an
// unclosed "{{" followed by a later one restarts at the later one, so a typo
// does not swallow the text up to the next span.
func replaceInlineCode(s string, fn func(string) string) string {
	var b strings.Builder
	for {
		open := strings.Index(s, "{{")
		if open < 0 {
			break
		}
		closing := strings.Index(s[open+2:], "}}")
		if closing < 0 {
			break
		}
		closing += open + 2
		if next := strings.Index(s[open+2:closing], "{{"); next >= 0 {
			cut := open + 2 + next
			b.WriteString(s[:cut])
			s = s[cut:]
			continue
		}
		if closing == open+2 { // "{{}}" is not a span
			b.WriteString(s[:closing+2])
			s = s[closing+2:]
			continue
		}
		b.WriteString(s[:open])
		b.WriteString(fn(s[open+2 : closing]))
		s = s[closing+2:]
	}
	b.WriteString(s)
	return b.String()
}
func isWikiTableHeader(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "||") && strings.HasSuffix(t, "||") && len(t) >= 4
}

func isWikiTableRow(s string) bool {
	t := strings.TrimSpace(s)
	if len(t) < 2 {
		return false
	}
	if strings.HasPrefix(t, "||") {
		return false
	}
	return strings.HasPrefix(t, "|") && strings.HasSuffix(t, "|")
}

func splitWikiTableHeader(s string) []string {
	return splitTableCells(s, "||")
}
