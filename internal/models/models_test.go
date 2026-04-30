package models

import (
	"encoding/json"
	"testing"
)

// TestCreateIssueFields_MarshalExtraMerged verifies that custom fields in Extra
// are flattened into the same top-level object as the typed fields.
func TestCreateIssueFields_MarshalExtraMerged(t *testing.T) {
	f := CreateIssueFields{
		Project:   ProjectRef{Key: "ESA"},
		Summary:   "Test epic",
		IssueType: TypeRef{Name: "Epic"},
		Extra: map[string]any{
			"customfield_10011": "Q2 Refactor",
			"customfield_10014": "ESA-42",
		},
	}

	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := map[string]string{
		"customfield_10011": "Q2 Refactor",
		"customfield_10014": "ESA-42",
	}
	for k, v := range want {
		got, ok := decoded[k]
		if !ok {
			t.Errorf("missing key %s in marshalled output", k)
			continue
		}
		if got != v {
			t.Errorf("%s = %v, want %v", k, got, v)
		}
	}
	if decoded["summary"] != "Test epic" {
		t.Errorf("summary lost; got %v", decoded["summary"])
	}
}

// TestCreateIssueFields_TypedWinsOverExtra ensures Extra cannot shadow typed fields.
func TestCreateIssueFields_TypedWinsOverExtra(t *testing.T) {
	f := CreateIssueFields{
		Project:   ProjectRef{Key: "ESA"},
		Summary:   "original",
		IssueType: TypeRef{Name: "Epic"},
		Extra: map[string]any{
			"summary": "hijacked",
		},
	}
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	if decoded["summary"] != "original" {
		t.Errorf("Extra overrode typed field; got %v", decoded["summary"])
	}
}

// TestIssueFields_CustomString verifies raw custom-field lookup after unmarshalling.
func TestIssueFields_CustomString(t *testing.T) {
	raw := `{"summary":"Hello","customfield_10011":"Q2 Refactor","customfield_10014":null}`
	var f IssueFields
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := f.CustomString("customfield_10011"); got != "Q2 Refactor" {
		t.Errorf("CustomString(10011) = %q, want Q2 Refactor", got)
	}
	if got := f.CustomString("customfield_10014"); got != "" {
		t.Errorf("CustomString(10014) = %q, want empty for null", got)
	}
	if got := f.CustomString("customfield_99999"); got != "" {
		t.Errorf("CustomString(unknown) = %q, want empty", got)
	}
}
