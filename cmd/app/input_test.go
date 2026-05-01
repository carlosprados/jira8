package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("description", "", "")
	cmd.Flags().String("description-file", "", "")
	return cmd
}

func TestReadTextInput_InlineOnly(t *testing.T) {
	cmd := newTestCmd()
	if err := cmd.ParseFlags([]string{"--description", "hello"}); err != nil {
		t.Fatal(err)
	}
	got, set, err := ReadTextInput(cmd, "description", "description-file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !set || got != "hello" {
		t.Errorf("got (%q, %v), want (%q, true)", got, set, "hello")
	}
}

func TestReadTextInput_FileOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "body.md")
	content := "# heading\n\nbody with `code` and **bold**"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newTestCmd()
	if err := cmd.ParseFlags([]string{"--description-file", path}); err != nil {
		t.Fatal(err)
	}
	got, set, err := ReadTextInput(cmd, "description", "description-file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !set || got != content {
		t.Errorf("got (%q, %v), want (%q, true)", got, set, content)
	}
}

func TestReadTextInput_Conflict(t *testing.T) {
	cmd := newTestCmd()
	if err := cmd.ParseFlags([]string{"--description", "x", "--description-file", "/tmp/y"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := ReadTextInput(cmd, "description", "description-file")
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error %q does not mention mutual exclusion", err)
	}
}

func TestReadTextInput_FileMissing(t *testing.T) {
	cmd := newTestCmd()
	if err := cmd.ParseFlags([]string{"--description-file", "/nonexistent/path.md"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := ReadTextInput(cmd, "description", "description-file")
	if err == nil {
		t.Fatal("expected file-not-found error, got nil")
	}
}

func TestReadTextInput_NeitherSet(t *testing.T) {
	cmd := newTestCmd()
	if err := cmd.ParseFlags([]string{}); err != nil {
		t.Fatal(err)
	}
	got, set, err := ReadTextInput(cmd, "description", "description-file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set || got != "" {
		t.Errorf("got (%q, %v), want (\"\", false)", got, set)
	}
}

func TestReadTextInput_Stdin(t *testing.T) {
	cmd := newTestCmd()
	if err := cmd.ParseFlags([]string{"--description-file", "-"}); err != nil {
		t.Fatal(err)
	}
	prev := stdinReader
	stdinReader = strings.NewReader("piped content")
	defer func() { stdinReader = prev }()

	got, set, err := ReadTextInput(cmd, "description", "description-file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !set || got != "piped content" {
		t.Errorf("got (%q, %v), want (%q, true)", got, set, "piped content")
	}
}

func TestReadTextInput_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newTestCmd()
	if err := cmd.ParseFlags([]string{"--description-file", path}); err != nil {
		t.Fatal(err)
	}
	got, set, err := ReadTextInput(cmd, "description", "description-file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !set || got != "" {
		t.Errorf("got (%q, %v), want (\"\", true)", got, set)
	}
}
