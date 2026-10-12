package app

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/amplia/jira8/internal/models"
)

var testScreen = map[string]models.FieldMeta{
	"labels":            {Name: "Labels", Schema: models.FieldSchema{Type: "array", Items: "string"}},
	"components":        {Name: "Component/s", Schema: models.FieldSchema{Type: "array", Items: "component"}, AllowedValues: []models.AllowedValue{{ID: "1", Name: "core"}, {ID: "2", Name: "ui"}}},
	"customfield_10008": {Name: "Story Points", Schema: models.FieldSchema{Type: "number"}},
	"customfield_10300": {Name: "Customer Owner", Schema: models.FieldSchema{Type: "string"}},
	"customfield_10003": {Name: "Severity", Schema: models.FieldSchema{Type: "option"}, AllowedValues: []models.AllowedValue{{ID: "10", Value: "High"}, {ID: "11", Value: "Low"}}},
	"customfield_10400": {Name: "Team", Schema: models.FieldSchema{Type: "option"}},
	"customfield_10401": {Name: "Team", Schema: models.FieldSchema{Type: "string"}},
	"timetracking":      {Name: "Time Tracking", Schema: models.FieldSchema{Type: "timetracking"}},
}

func TestResolveField(t *testing.T) {
	cases := map[string]string{"story points": "customfield_10008", "customfield_10300": "customfield_10300", "Component/s": "components"}
	for in, want := range cases {
		if got, err := resolveField(testScreen, in); err != nil || got != want {
			t.Errorf("resolveField(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := resolveField(testScreen, "Sprint"); err == nil || !strings.Contains(err.Error(), "Story Points") {
		t.Errorf("unknown field: want the available list, got %v", err)
	}
	if _, err := resolveField(testScreen, "team"); err == nil || !strings.Contains(err.Error(), "customfield_10400, customfield_10401") {
		t.Errorf("ambiguous name: want both ids, got %v", err)
	}
}

func TestCoerceField(t *testing.T) {
	cases := []struct {
		id   string
		vals []string
		want any
	}{
		{"customfield_10008", []string{"5"}, 5.0},
		{"customfield_10300", []string{"ACME, Inc."}, "ACME, Inc."},
		{"customfield_10003", []string{"high"}, map[string]any{"id": "10"}},
		{"customfield_10400", []string{"Core"}, map[string]any{"value": "Core"}},
		{"components", []string{"core, ui"}, []any{map[string]any{"id": "1"}, map[string]any{"id": "2"}}},
		{"components", []string{"core", "2"}, []any{map[string]any{"id": "1"}, map[string]any{"id": "2"}}},
		{"labels", []string{"a,b"}, []any{"a", "b"}},
		{"labels", []string{""}, []any{}},
		{"customfield_10300", []string{""}, nil},
		{"customfield_10400", []string{`{"id":"7"}`}, map[string]any{"id": "7"}},
		{"timetracking", []string{`{"originalEstimate":"3d"}`}, map[string]any{"originalEstimate": "3d"}},
	}
	for _, c := range cases {
		got, err := coerceField(context.Background(), c.id, testScreen[c.id], c.vals)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("coerceField(%s, %q) = %#v, %v; want %#v", c.id, c.vals, got, err, c.want)
		}
	}
	bad := []struct {
		id   string
		vals []string
		msg  string
	}{
		{"customfield_10008", []string{"five"}, "takes a number"},
		{"customfield_10003", []string{"Medium"}, "allowed: High, Low"},
		{"components", []string{"core,web"}, "allowed: core, ui"},
		{"customfield_10300", []string{"a", "b"}, "takes one value"},
		{"customfield_10400", []string{"{bad"}, "invalid JSON"},
		{"timetracking", []string{"3d"}, "--original-estimate"},
	}
	for _, c := range bad {
		if _, err := coerceField(context.Background(), c.id, testScreen[c.id], c.vals); err == nil || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("coerceField(%s, %q): want error with %q, got %v", c.id, c.vals, c.msg, err)
		}
	}
}

func TestCleanLabels(t *testing.T) {
	got, err := cleanLabels([]string{" a ", "", "b"})
	if err != nil || !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("cleanLabels = %q, %v", got, err)
	}
	if _, err := cleanLabels([]string{"two words"}); err == nil {
		t.Error("cleanLabels accepted a label with a space")
	}
}

func TestClosest(t *testing.T) {
	allowed := []string{"web-1.0", "odm-9.0", "odm-10.1", "odm-10.2", "ui-2"}
	if got := closest(allowed, "x", 10); got != "allowed: web-1.0, odm-9.0, odm-10.1, odm-10.2, ui-2" {
		t.Errorf("closest under max = %q", got)
	}
	if got := closest(allowed, "odm-10.9", 2); !strings.HasPrefix(got, "closest of 5 allowed: odm-10.1, odm-10.2 (") {
		t.Errorf("closest over max = %q", got)
	}
}
