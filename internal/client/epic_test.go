package client

import (
	"encoding/json"
	"testing"
)

func TestTallyChild(t *testing.T) {
	children := []string{
		`{"customfield_1":"E-1","status":{"statusCategory":{"key":"done"}},"timeoriginalestimate":7200,"timeestimate":600,"timespent":7000}`,
		`{"customfield_1":"E-1","status":{"statusCategory":{"key":"indeterminate"}},"timeoriginalestimate":3600,"timeestimate":1800,"timespent":null}`,
		`{"customfield_1":"E-1","status":{"statusCategory":{"key":"new"}},"timeoriginalestimate":null,"timeestimate":null}`,
		`{"customfield_1":"OTHER","status":{"statusCategory":{"key":"new"}}}`,
	}
	progress := map[string]*EpicProgress{"E-1": {}, "E-2": {}}
	for _, c := range children {
		var f map[string]json.RawMessage
		if err := json.Unmarshal([]byte(c), &f); err != nil {
			t.Fatal(err)
		}
		tallyChild(progress, f, "customfield_1")
	}

	want := EpicProgress{Children: 3, Todo: 1, Doing: 1, Done: 1, Unestimated: 1,
		OriginalEstimate: 10800, RemainingEstimate: 1800, TimeSpent: 7000}
	if got := *progress["E-1"]; got != want {
		t.Errorf("E-1 = %+v, want %+v", got, want)
	}
	if got := *progress["E-2"]; got != (EpicProgress{}) {
		t.Errorf("E-2 = %+v, want zero", got)
	}
}
