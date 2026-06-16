package client

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// doMultipartUpload posts one or more files to a Jira multipart endpoint
// (currently only /issue/{key}/attachments). Files are streamed using io.Pipe
// so large attachments do not need to be buffered in memory.
//
// Requires the X-Atlassian-Token: no-check header, which Jira mandates for
// attachment uploads to defeat XSRF protection. Without it the server replies
// 403 with an XSRF error.
//
// Retries are not attempted: the request body is a one-shot pipe and the
// underlying os.File readers cannot be rewound safely. Callers see network
// or HTTP errors as-is.
func (c *Client) doMultipartUpload(ctx context.Context, path string, files []string) ([]byte, error) {
	// Pre-validate every path before opening the request so we fail fast
	// without leaking file handles or sending a half-built multipart body.
	for _, p := range files {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("attachment %q: %w", p, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("attachment %q is a directory", p)
		}
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		var err error
		defer func() {
			// Closing the writer flushes the trailing boundary; closing the
			// pipe end signals EOF (or the error) to the HTTP transport.
			if cerr := mw.Close(); err == nil {
				err = cerr
			}
			_ = pw.CloseWithError(err)
		}()

		for _, p := range files {
			var f *os.File
			f, err = os.Open(p)
			if err != nil {
				return
			}
			var part io.Writer
			part, err = mw.CreateFormFile("file", filepath.Base(p))
			if err != nil {
				_ = f.Close()
				return
			}
			if _, err = io.Copy(part, f); err != nil {
				_ = f.Close()
				return
			}
			if err = f.Close(); err != nil {
				return
			}
		}
	}()

	fullURL := c.baseURL + apiBasePath + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, pr)
	if err != nil {
		_ = pr.Close()
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Atlassian-Token", "no-check")
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.uploadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, parseAPIError(resp.StatusCode, respBody)
	}

	return respBody, nil
}
