package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalImageFiles(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"a.png", "img/b.png", "other/a.png"} {
		p := filepath.Join(dir, f)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := localImageFiles([]string{"a.png", "img/b.png", "./a.png"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "a.png"), filepath.Join(dir, "img/b.png")}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", got, want)
	}

	if _, err := localImageFiles([]string{"missing.png"}, dir); err == nil {
		t.Error("missing file: expected error")
	}
	if _, err := localImageFiles([]string{"a.png", "other/a.png"}, dir); err == nil ||
		!strings.Contains(err.Error(), "share the name") {
		t.Errorf("same name, different files: got %v", err)
	}
	if _, err := localImageFiles([]string{"img"}, dir); err == nil {
		t.Error("directory: expected error")
	}
}
