package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/amplia/jira8/internal/config"
	"github.com/amplia/jira8/internal/models"
)

// TestDryRun checks that writes are recorded and never reach Jira, while reads
// still do, and that a planned create hands DryRunKey to the calls after it.
func TestDryRun(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a.png")
	if err := os.WriteFile(file, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	reads := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("dry run sent %s %s", r.Method, r.URL.Path)
		}
		reads++
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := New(&config.Config{URL: srv.URL, Token: "t"})
	c.SetDryRun(true)
	ctx := context.Background()

	resp, err := c.CreateIssue(ctx, &models.CreateIssueRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Key != DryRunKey {
		t.Fatalf("create key = %q, want %q", resp.Key, DryRunKey)
	}
	if _, err := c.AddAttachments(ctx, resp.Key, []string{file}); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteComment(ctx, "ABC-1", "7"); err != nil {
		t.Fatal(err)
	}
	_, _ = c.GetIssueLinkTypes(ctx)
	if reads != 1 {
		t.Errorf("reads = %d, want 1", reads)
	}

	got := c.Planned()
	want := []struct{ method, path string }{
		{http.MethodPost, "/rest/api/2/issue"},
		{http.MethodPost, "/rest/api/2/issue/NEW/attachments"},
		{http.MethodDelete, "/rest/api/2/issue/ABC-1/comment/7"},
	}
	if len(got) != len(want) {
		t.Fatalf("planned %d requests, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Method != w.method || got[i].Path != w.path {
			t.Errorf("planned[%d] = %s %s, want %s %s", i, got[i].Method, got[i].Path, w.method, w.path)
		}
	}
	if len(got[1].Files) != 1 || got[1].Files[0] != file {
		t.Errorf("planned upload files = %v, want [%s]", got[1].Files, file)
	}
}
