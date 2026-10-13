package issue

import "testing"

func TestWithOrder(t *testing.T) {
	cases := []struct{ jql, want string }{
		{"project = ABC", "project = ABC ORDER BY created ASC"},
		{"project = ABC order by updated DESC", "project = ABC ORDER BY created ASC"},
		{"", "ORDER BY created ASC"},
	}
	for _, c := range cases {
		if got := withOrder(c.jql, "created", true); got != c.want {
			t.Errorf("withOrder(%q) = %q, want %q", c.jql, got, c.want)
		}
	}
}
