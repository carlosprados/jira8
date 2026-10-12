package issue

import (
	"reflect"
	"strings"
	"testing"

	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

func TestTransitionFields(t *testing.T) {
	done := &models.Transition{Name: "Done", Fields: map[string]models.FieldMeta{
		"resolution":  {Name: "Resolution", Required: true, Schema: models.FieldSchema{Type: "resolution"}, AllowedValues: []models.AllowedValue{{ID: "1", Name: "Fixed"}, {ID: "2", Name: "Won't Fix"}}},
		"fixVersions": {Name: "Fix Version/s", Schema: models.FieldSchema{Type: "array", Items: "version"}},
	}}
	noScreen := &models.Transition{Name: "Start"}
	run := func(tr *models.Transition, fields ...string) (map[string]any, error) {
		cmd := &cobra.Command{}
		cmd.Flags().StringArray("field", nil, "")
		for _, f := range fields {
			_ = cmd.Flags().Set("field", f)
		}
		return transitionFields(cmd, tr)
	}

	got, err := run(done, "resolution=won't fix")
	if want := map[string]any{"resolution": map[string]any{"id": "2"}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("resolution given: got %v, %v", got, err)
	}
	if _, err := run(done); err == nil || !strings.Contains(err.Error(), `requires Resolution`) || !strings.Contains(err.Error(), "Won't Fix") {
		t.Errorf("missing required: got %v", err)
	}
	if _, err := run(done, "Fix Version/s=2.4"); err == nil || !strings.Contains(err.Error(), "requires Resolution") {
		t.Errorf("optional only: got %v", err)
	}
	if got, err := run(noScreen); err != nil || got != nil {
		t.Errorf("no screen, no fields: got %v, %v", got, err)
	}
	if _, err := run(noScreen, "Resolution=Fixed"); err == nil || !strings.Contains(err.Error(), "has no screen") {
		t.Errorf("field on no screen: got %v", err)
	}
}
