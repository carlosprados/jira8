package app

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

// AddFieldFlags registers --field and --labels, shared by create and edit.
func AddFieldFlags(cmd *cobra.Command) {
	cmd.Flags().StringArray("field", nil, `Set a field by name or id: "Story Points=5", "Component/s=core,ui", customfield_10010='{"value":"X"}' (repeatable; see 'project fields' / 'issue fields')`)
	cmd.Flags().StringSlice("labels", nil, "Labels, comma-separated; replaces the existing ones (\"\" clears on edit)")
}

// AddLabelEditFlags registers --add-label and --remove-label for edit commands.
func AddLabelEditFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("add-label", nil, "Add labels, keeping the others (comma-separated or repeatable)")
	cmd.Flags().StringSlice("remove-label", nil, "Remove labels, keeping the others (comma-separated or repeatable)")
}

// FieldFlagsSet reports whether any flag of AddFieldFlags/AddLabelEditFlags was used.
func FieldFlagsSet(cmd *cobra.Command) bool {
	for _, f := range []string{"field", "labels", "add-label", "remove-label"} {
		if cmd.Flags().Lookup(f) != nil && cmd.Flags().Changed(f) {
			return true
		}
	}
	return false
}

// Labels returns --labels when it was passed, validated. A bare "" yields an
// empty, non-nil slice, which clears the labels on edit.
func Labels(cmd *cobra.Command) ([]string, bool, error) {
	if !cmd.Flags().Changed("labels") {
		return nil, false, nil
	}
	v, _ := cmd.Flags().GetStringSlice("labels")
	labels, err := cleanLabels(v)
	return labels, true, err
}

// LabelUpdates turns --add-label/--remove-label into Jira "update" verbs.
func LabelUpdates(cmd *cobra.Command) ([]map[string]any, error) {
	var ops []map[string]any
	for _, verb := range []string{"add", "remove"} {
		v, _ := cmd.Flags().GetStringSlice(verb + "-label")
		labels, err := cleanLabels(v)
		if err != nil {
			return nil, err
		}
		for _, l := range labels {
			ops = append(ops, map[string]any{verb: l})
		}
	}
	return ops, nil
}

func cleanLabels(in []string) ([]string, error) {
	out := []string{}
	for _, l := range in {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if strings.ContainsAny(l, " \t") {
			return nil, fmt.Errorf("label %q has a space: Jira labels cannot contain spaces", l)
		}
		out = append(out, l)
	}
	return out, nil
}

// CreateFields gathers --labels and --field for a create request. Fields are
// checked against the create screen of the project's issue type.
func CreateFields(cmd *cobra.Command, project, issueType string) (map[string]any, error) {
	extra := map[string]any{}
	if cmd.Flags().Changed("field") {
		screen, err := Get().Client.GetCreateFields(context.Background(), project, issueType)
		if err != nil {
			return nil, err
		}
		if extra, err = FieldValues(context.Background(), cmd, screen); err != nil {
			return nil, err
		}
	}
	labels, set, err := Labels(cmd)
	if err != nil {
		return nil, err
	}
	if set {
		extra["labels"] = labels
	}
	return extra, nil
}

// EditFields adds --labels and --field to fields and returns the label
// add/remove verbs for the "update" part of an edit. Fields are checked
// against the issue's edit screen.
func EditFields(cmd *cobra.Command, key string, fields map[string]any) (map[string][]map[string]any, error) {
	if cmd.Flags().Changed("field") {
		screen, err := Get().Client.GetEditFields(context.Background(), key)
		if err != nil {
			return nil, err
		}
		values, err := FieldValues(context.Background(), cmd, screen)
		if err != nil {
			return nil, err
		}
		maps.Copy(fields, values)
	}
	labels, set, err := Labels(cmd)
	if err != nil {
		return nil, err
	}
	if set {
		fields["labels"] = labels
	}
	ops, err := LabelUpdates(cmd)
	if err != nil || len(ops) == 0 {
		return nil, err
	}
	if set {
		return nil, fmt.Errorf("--labels replaces all labels: do not combine it with --add-label/--remove-label")
	}
	return map[string][]map[string]any{"labels": ops}, nil
}

// FieldValues resolves every --field against the fields of the screen it will
// be sent to (createmeta or editmeta), so an unknown field, a field not on the
// screen or a value outside its options fails before anything is written.
// It returns Jira field values keyed by field id.
func FieldValues(ctx context.Context, cmd *cobra.Command, screen map[string]models.FieldMeta) (map[string]any, error) {
	raw, _ := cmd.Flags().GetStringArray("field")
	values := map[string][]string{}
	var order []string
	for _, r := range raw {
		name, val, ok := strings.Cut(r, "=")
		if !ok {
			return nil, fmt.Errorf("--field %q: use NAME=VALUE", r)
		}
		id, err := resolveField(screen, strings.TrimSpace(name))
		if err != nil {
			return nil, err
		}
		if _, seen := values[id]; !seen {
			order = append(order, id)
		}
		values[id] = append(values[id], val)
	}
	out := map[string]any{}
	for _, id := range order {
		v, err := coerceField(ctx, id, screen[id], values[id])
		if err != nil {
			return nil, err
		}
		out[id] = v
	}
	return out, nil
}

// resolveField finds a field by id or by name (case-insensitive).
func resolveField(screen map[string]models.FieldMeta, name string) (string, error) {
	if _, ok := screen[name]; ok {
		return name, nil
	}
	var ids []string
	for id, f := range screen {
		if strings.EqualFold(f.Name, name) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	switch len(ids) {
	case 1:
		return ids[0], nil
	case 0:
		var names []string
		for _, f := range screen {
			names = append(names, f.Name)
		}
		sort.Strings(names)
		return "", fmt.Errorf("field %q is not on this screen; available: %s", name, strings.Join(names, ", "))
	default:
		return "", fmt.Errorf("field name %q is ambiguous, use the id: %s", name, strings.Join(ids, ", "))
	}
}

// coerceField builds the JSON value Jira expects for the field's schema from
// the raw values given for it (one per --field; arrays also split on commas).
// A value starting with { or [ is sent as raw JSON; an empty one clears.
func coerceField(ctx context.Context, id string, f models.FieldMeta, vals []string) (any, error) {
	last := strings.TrimSpace(vals[len(vals)-1])
	if strings.HasPrefix(last, "{") || strings.HasPrefix(last, "[") {
		var v any
		if err := json.Unmarshal([]byte(last), &v); err != nil {
			return nil, fmt.Errorf("--field %s: invalid JSON: %w", f.Name, err)
		}
		return v, nil
	}
	if last == "" && len(vals) == 1 {
		if f.Schema.Type == "array" {
			return []any{}, nil
		}
		return nil, nil
	}
	if f.Schema.Type == "array" {
		var items []string
		for _, v := range vals {
			for _, item := range strings.Split(v, ",") {
				if item = strings.TrimSpace(item); item != "" {
					items = append(items, item)
				}
			}
		}
		out := []any{}
		for _, item := range items {
			v, err := scalar(ctx, f, f.Schema.Items, item)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	if len(vals) > 1 {
		return nil, fmt.Errorf("--field %s given %d times but takes one value", f.Name, len(vals))
	}
	return scalar(ctx, f, f.Schema.Type, last)
}

func scalar(ctx context.Context, f models.FieldMeta, kind, v string) (any, error) {
	if len(f.AllowedValues) > 0 {
		for _, a := range f.AllowedValues {
			if strings.EqualFold(a.Label(), v) || a.ID == v {
				return map[string]any{"id": a.ID}, nil
			}
		}
		labels := Names(f.AllowedValues, models.AllowedValue.Label)
		return nil, fmt.Errorf("--field %s: %q is not allowed; %s", f.Name, v, closest(labels, v, 20))
	}
	switch kind {
	case "number":
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("--field %s takes a number, got %q", f.Name, v)
		}
		return n, nil
	case "user":
		name, err := Get().Client.ResolveAssignee(ctx, v)
		if err != nil {
			return nil, err
		}
		return map[string]any{"name": name}, nil
	case "option":
		return map[string]any{"value": v}, nil
	case "version", "component", "priority", "resolution", "securitylevel", "group":
		return map[string]any{"name": v}, nil
	case "string", "any", "date", "datetime", "":
		return v, nil
	case "timetracking":
		return nil, fmt.Errorf("--field %s: use --original-estimate (or JSON {\"originalEstimate\":\"3d\"})", f.Name)
	}
	if slices.Contains([]string{"issuelinks", "attachment", "comments-page"}, kind) {
		return nil, fmt.Errorf("--field %s: use the dedicated command (link, attachment, comment-add)", f.Name)
	}
	return nil, fmt.Errorf("--field %s has type %s: pass the value as JSON", f.Name, kind)
}

// closest names the allowed values, or, when there are more than max, the max
// sharing the longest prefix with v (fields like versions have hundreds).
func closest(allowed []string, v string, max int) string {
	if len(allowed) <= max {
		return "allowed: " + strings.Join(allowed, ", ")
	}
	common := func(a string) int {
		a, b := strings.ToLower(a), strings.ToLower(v)
		n := 0
		for n < len(a) && n < len(b) && a[n] == b[n] {
			n++
		}
		return n
	}
	sorted := slices.Clone(allowed)
	slices.SortStableFunc(sorted, func(a, b string) int { return common(b) - common(a) })
	return fmt.Sprintf("closest of %d allowed: %s (all: 'project fields' / 'issue fields' -o json)", len(allowed), strings.Join(sorted[:max], ", "))
}

// FieldRow is the JSON shape of 'project fields' and 'issue fields'.
type FieldRow struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Required   bool     `json:"required"`
	Type       string   `json:"type"`
	Items      string   `json:"items,omitempty"`
	Custom     string   `json:"custom,omitempty"`
	Operations []string `json:"operations,omitempty"`
	Allowed    []string `json:"allowed,omitempty"`
}

// FieldRows flattens screen metadata, required fields first, then by name.
func FieldRows(screen map[string]models.FieldMeta) []FieldRow {
	rows := make([]FieldRow, 0, len(screen))
	for id, f := range screen {
		rows = append(rows, FieldRow{
			ID: id, Name: f.Name, Required: f.Required,
			Type: f.Schema.Type, Items: f.Schema.Items, Custom: f.Schema.Custom,
			Operations: f.Operations,
			Allowed:    Names(f.AllowedValues, models.AllowedValue.Label),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Required != rows[j].Required {
			return rows[i].Required
		}
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
	return rows
}

// PrintFieldRows renders FieldRows for humans.
func PrintFieldRows(rows []FieldRow) {
	for _, r := range rows {
		typ := r.Type
		if r.Items != "" {
			typ += " of " + r.Items
		}
		req := ""
		if r.Required {
			req = "  required"
		}
		fmt.Printf("  %-28s %-20s %s%s\n", r.Name, r.ID, typ, req)
		if len(r.Allowed) > 0 {
			allowed, more := r.Allowed, ""
			if n := len(allowed); n > 15 {
				// Versions come oldest first: the newest are the useful ones.
				if r.Type == "version" || r.Items == "version" {
					allowed, more = allowed[n-15:], fmt.Sprintf(" (latest 15 of %d, see -o json)", n)
				} else {
					allowed, more = allowed[:15], fmt.Sprintf(" … (+%d, see -o json)", n-15)
				}
			}
			fmt.Printf("  %-28s allowed: %s%s\n", "", strings.Join(allowed, ", "), more)
		}
	}
}

// UpdatedFields lists the field ids an edit touched, for the JSON answer.
func UpdatedFields(fields map[string]any, update map[string][]map[string]any) []string {
	ids := slices.Collect(maps.Keys(fields))
	for id := range update {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}
