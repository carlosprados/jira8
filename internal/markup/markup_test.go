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
