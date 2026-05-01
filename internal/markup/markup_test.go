package markup

import "testing"

func TestMarkdownToWiki(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain", "just text", "just text"},

		{"h1", "# Title", "h1. Title"},
		{"h3", "### Section", "h3. Section"},
		{"h6", "###### Tiny", "h6. Tiny"},
		{"too-deep heading is plain", "####### Plain", "####### Plain"},

		{"bold star", "this is **bold** here", "this is *bold* here"},
		{"bold underscore", "this is __bold__ here", "this is *bold* here"},
		{"italic star", "this is *italic* here", "this is _italic_ here"},
		{"italic underscore", "this is _italic_ here", "this is _italic_ here"},
		{"strikethrough", "~~gone~~", "-gone-"},
		{"inline code", "use `make build`", "use {{make build}}"},
		{"link", "see [docs](https://example.com)", "see [docs|https://example.com]"},

		{"bold then italic", "**bold** and *italic*", "*bold* and _italic_"},
		{
			"underscore inside identifier is not italic",
			"call my_helper_func",
			"call my_helper_func",
		},

		{
			"fenced code preserves content",
			"before\n```go\nfunc f() {}\n```\nafter",
			"before\n{code:go}\nfunc f() {}\n{code}\nafter",
		},
		{
			"fenced code without lang",
			"```\nplain text\n```",
			"{code}\nplain text\n{code}",
		},
		{
			"markdown markers inside fence are NOT transformed",
			"```\n**not bold**\n# not heading\n```",
			"{code}\n**not bold**\n# not heading\n{code}",
		},

		{
			"unordered list",
			"- one\n- two\n- three",
			"* one\n* two\n* three",
		},
		{
			"nested unordered list",
			"- one\n  - one.a\n  - one.b\n- two",
			"* one\n** one.a\n** one.b\n* two",
		},
		{
			"ordered list",
			"1. first\n2. second\n3. third",
			"# first\n# second\n# third",
		},
		{
			"mixed list with asterisk bullets",
			"* a\n* b",
			"* a\n* b",
		},

		{
			"blockquote single line",
			"> quoted text",
			"bq. quoted text",
		},
		{
			"blockquote with markdown inline",
			"> **important** note",
			"bq. *important* note",
		},

		{
			"horizontal rule dashes",
			"above\n\n---\n\nbelow",
			"above\n\n----\n\nbelow",
		},
		{
			"horizontal rule asterisks",
			"***",
			"----",
		},

		{
			"table basic",
			"| Name | Role |\n|------|------|\n| Alice | dev |\n| Bob | ops |",
			"||Name||Role||\n|Alice|dev|\n|Bob|ops|",
		},
		{
			"table with inline formatting",
			"| Field | Value |\n|-------|-------|\n| `id` | **42** |",
			"||Field||Value||\n|{{id}}|*42*|",
		},

		{
			"complex mixed document",
			"# Plan\n\nWe need to ship **soon**.\n\n- spec the `API`\n- write tests\n\n```go\nfunc main() {}\n```\n\nSee [tracker](https://example.com).",
			"h1. Plan\n\nWe need to ship *soon*.\n\n* spec the {{API}}\n* write tests\n\n{code:go}\nfunc main() {}\n{code}\n\nSee [tracker|https://example.com].",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MarkdownToWiki(tc.in)
			if got != tc.want {
				t.Errorf("MarkdownToWiki(%q)\n  got  %q\n  want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestWikiToMarkdown(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain", "just text", "just text"},

		{"h1", "h1. Title", "# Title"},
		{"h3", "h3. Section", "### Section"},
		{"h6", "h6. Tiny", "###### Tiny"},

		{"bold", "this is *bold* here", "this is **bold** here"},
		{"italic", "this is _italic_ here", "this is *italic* here"},
		{"strike", "this is -gone- here", "this is ~~gone~~ here"},
		{"inline code", "use {{make build}}", "use `make build`"},
		{"link", "see [docs|https://example.com]", "see [docs](https://example.com)"},
		{"image", "!screenshot.png!", "![screenshot.png](screenshot.png)"},

		{"bold then italic", "*bold* and _italic_", "**bold** and *italic*"},

		{
			"underscore inside identifier is not italic",
			"call my_helper_func",
			"call my_helper_func",
		},
		{
			"hyphens in dates are not strike",
			"on 2026-04-30 we shipped",
			"on 2026-04-30 we shipped",
		},

		{
			"fenced code preserves content",
			"before\n{code:go}\nfunc f() {}\n{code}\nafter",
			"before\n```go\nfunc f() {}\n```\nafter",
		},
		{
			"fenced code without lang",
			"{code}\nplain text\n{code}",
			"```\nplain text\n```",
		},
		{
			"wiki markers inside fence are NOT transformed",
			"{code}\n*not bold*\nh1. not heading\n{code}",
			"```\n*not bold*\nh1. not heading\n```",
		},

		{
			"unordered list",
			"* one\n* two\n* three",
			"- one\n- two\n- three",
		},
		{
			"nested unordered list",
			"* one\n** one.a\n** one.b\n* two",
			"- one\n  - one.a\n  - one.b\n- two",
		},
		{
			"ordered list",
			"# first\n# second\n# third",
			"1. first\n1. second\n1. third",
		},

		{
			"blockquote single line",
			"bq. quoted text",
			"> quoted text",
		},
		{
			"blockquote with wiki inline",
			"bq. *important* note",
			"> **important** note",
		},
		{
			"quote block",
			"{quote}\nfirst line\nsecond line\n{quote}",
			"> first line\n> second line",
		},

		{
			"horizontal rule",
			"above\n\n----\n\nbelow",
			"above\n\n---\n\nbelow",
		},

		{
			"table basic",
			"||Name||Role||\n|Alice|dev|\n|Bob|ops|",
			"| Name | Role |\n| --- | --- |\n| Alice | dev |\n| Bob | ops |",
		},
		{
			"table with inline formatting",
			"||Field||Value||\n|{{id}}|*42*|",
			"| Field | Value |\n| --- | --- |\n| `id` | **42** |",
		},

		{
			"complex mixed document",
			"h1. Plan\n\nWe need to ship *soon*.\n\n* spec the {{API}}\n* write tests\n\n{code:go}\nfunc main() {}\n{code}\n\nSee [tracker|https://example.com].",
			"# Plan\n\nWe need to ship **soon**.\n\n- spec the `API`\n- write tests\n\n```go\nfunc main() {}\n```\n\nSee [tracker](https://example.com).",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := WikiToMarkdown(tc.in)
			if got != tc.want {
				t.Errorf("WikiToMarkdown(%q)\n  got  %q\n  want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestRoundTrip checks the lossless subset: Markdown → Wiki → Markdown should
// produce text equivalent to the original. The full superset of Markdown is
// not lossless (e.g. * vs - for unordered bullets, _italic_ vs *italic*), so
// the test inputs are deliberately written in the "canonical" form that the
// converters target.
func TestRoundTrip(t *testing.T) {
	canonical := []string{
		"# Title\n\nA paragraph with **bold**, *italic*, ~~strike~~, and `code`.",
		"- one\n- two\n  - nested\n- three",
		"```go\nfunc f() {}\n```",
		"> quoted line",
		"| h1 | h2 |\n| --- | --- |\n| c1 | c2 |",
		"See [the link](https://example.com).",
		"---",
	}

	for _, original := range canonical {
		t.Run(original[:min(len(original), 30)], func(t *testing.T) {
			wiki := MarkdownToWiki(original)
			back := WikiToMarkdown(wiki)
			if back != original {
				t.Errorf("round trip differs:\n  original %q\n  via wiki %q\n  got back %q", original, wiki, back)
			}
		})
	}
}
