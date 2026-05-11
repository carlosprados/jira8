package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amplia/jira8/internal/config"
)

// TestAddAttachments_MultipartBody verifies the multipart body is built
// correctly: every file ends up as a "file" form part with the right filename
// and contents, and the required X-Atlassian-Token header is set.
func TestAddAttachments_MultipartBody(t *testing.T) {
	tmp := t.TempDir()
	pathA := filepath.Join(tmp, "alpha.txt")
	pathB := filepath.Join(tmp, "beta.bin")
	if err := os.WriteFile(pathA, []byte("alpha-body"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, []byte{0x01, 0x02, 0x03}, 0o600); err != nil {
		t.Fatal(err)
	}

	var gotFiles map[string]string
	var gotXSRF, gotAuth, gotContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rest/api/2/issue/ESA-1/attachments") {
			http.NotFound(w, r)
			return
		}
		gotXSRF = r.Header.Get("X-Atlassian-Token")
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")

		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		gotFiles = map[string]string{}
		for _, fh := range r.MultipartForm.File["file"] {
			f, err := fh.Open()
			if err != nil {
				t.Fatalf("opening part %q: %v", fh.Filename, err)
			}
			data, _ := io.ReadAll(f)
			_ = f.Close()
			gotFiles[fh.Filename] = string(data)
		}

		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "10", "filename": "alpha.txt", "size": 10},
			{"id": "11", "filename": "beta.bin", "size": 3},
		})
	}))
	defer srv.Close()

	c := New(&config.Config{URL: srv.URL, Token: "secret"})
	atts, err := c.AddAttachments(context.Background(), "ESA-1", []string{pathA, pathB})
	if err != nil {
		t.Fatalf("AddAttachments: %v", err)
	}
	if len(atts) != 2 {
		t.Fatalf("len(attachments) = %d, want 2", len(atts))
	}

	if gotXSRF != "no-check" {
		t.Errorf("X-Atlassian-Token = %q, want no-check", gotXSRF)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("Authorization = %q, want Bearer secret", gotAuth)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data prefix", gotContentType)
	}
	if got := gotFiles["alpha.txt"]; got != "alpha-body" {
		t.Errorf("alpha.txt body = %q, want alpha-body", got)
	}
	if got := gotFiles["beta.bin"]; len(got) != 3 || got[0] != 0x01 {
		t.Errorf("beta.bin body = %v, want 3 bytes starting with 0x01", []byte(got))
	}
}

// TestAddAttachments_MissingFile guarantees the call fails locally without
// hitting the network when one of the paths doesn't exist.
func TestAddAttachments_MissingFile(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := New(&config.Config{URL: srv.URL, Token: "x"})
	_, err := c.AddAttachments(context.Background(), "ESA-1", []string{"/no/such/file.png"})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if called {
		t.Error("server was contacted; expected pre-validation to abort the call")
	}
}

func TestAddAttachments_NoFiles(t *testing.T) {
	c := New(&config.Config{URL: "http://localhost", Token: "x"})
	if _, err := c.AddAttachments(context.Background(), "ESA-1", nil); err == nil {
		t.Fatal("expected error when no files are passed, got nil")
	}
}

// TestAddAttachments_DirectoryRejected ensures we don't try to upload directories.
func TestAddAttachments_DirectoryRejected(t *testing.T) {
	dir := t.TempDir()
	c := New(&config.Config{URL: "http://localhost", Token: "x"})
	_, err := c.AddAttachments(context.Background(), "ESA-1", []string{dir})
	if err == nil {
		t.Fatal("expected error when passing a directory, got nil")
	}
}

func TestListAttachments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rest/api/2/issue/ESA-2") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":  "1",
			"key": "ESA-2",
			"fields": map[string]any{
				"summary": "with attachments",
				"attachment": []map[string]any{
					{"id": "100", "filename": "report.pdf", "size": 4096},
					{"id": "101", "filename": "trace.log", "size": 1024},
				},
			},
		})
	}))
	defer srv.Close()

	c := New(&config.Config{URL: srv.URL, Token: "x"})
	atts, err := c.ListAttachments(context.Background(), "ESA-2")
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}
	if len(atts) != 2 {
		t.Fatalf("len = %d, want 2", len(atts))
	}
	if atts[0].Filename != "report.pdf" || atts[0].Size != 4096 {
		t.Errorf("attachment[0] = %+v", atts[0])
	}
}

func TestDeleteAttachment(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(&config.Config{URL: srv.URL, Token: "x"})
	if err := c.DeleteAttachment(context.Background(), "12345"); err != nil {
		t.Fatalf("DeleteAttachment: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/rest/api/2/attachment/12345") {
		t.Errorf("path = %q, want suffix /rest/api/2/attachment/12345", gotPath)
	}
}

// TestAddAttachments_ServerError verifies that a 4xx response surfaces as an
// APIError carrying the Jira error messages.
func TestAddAttachments_ServerError(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "x.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errorMessages": []string{"XSRF check failed"},
		})
	}))
	defer srv.Close()

	c := New(&config.Config{URL: srv.URL, Token: "x"})
	_, err := c.AddAttachments(context.Background(), "ESA-1", []string{path})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Error(), "XSRF check failed") {
		t.Errorf("error message = %q, want it to mention XSRF", apiErr.Error())
	}
}
