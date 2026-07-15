package filebase

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func newCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

// TestServeFileAsOK verifies the streaming contract mirrors Java's
// DownloadFileUtil.download: octet-stream, explicit Content-Disposition with
// the download name, Content-Length, and the full body.
func TestServeFileAsOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lic.bin")
	body := []byte("license-bytes-12345")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	c, w := newCtx()
	serveFileAs(c, path, "original.lic")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", got)
	}
	if got := w.Header().Get("Content-Disposition"); got != `attachment; filename="original.lic"` {
		t.Errorf("Content-Disposition = %q", got)
	}
	if got := w.Header().Get("Content-Length"); got != "19" {
		t.Errorf("Content-Length = %q, want 19", got)
	}
	if w.Body.String() != string(body) {
		t.Errorf("body = %q, want %q", w.Body.String(), string(body))
	}
}

// TestServeFileAsMissing mirrors Java's empty-200 when the file is absent.
func TestServeFileAsMissing(t *testing.T) {
	c, w := newCtx()
	serveFileAs(c, filepath.Join(t.TempDir(), "nope.bin"), "x.bin")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (empty)", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("body len = %d, want 0", w.Body.Len())
	}
}

// TestParamHelpers verifies elementId / fileId parsing used by the download
// handlers.
func TestParamHelpers(t *testing.T) {
	c, _ := newCtx()
	c.Request, _ = http.NewRequest("GET", "/x?elementId=42", nil)
	if got := elementIDParam(c); got != 42 {
		t.Errorf("elementIDParam = %d, want 42", got)
	}

	c2, _ := newCtx()
	c2.Request, _ = http.NewRequest("GET", "/x?fileId=17", nil)
	if got := fileIdParam(c2); got != 17 {
		t.Errorf("fileIdParam = %d, want 17", got)
	}

	c3, _ := newCtx()
	c3.Request, _ = http.NewRequest("GET", "/x", nil)
	if got := fileIdParam(c3); got != 0 {
		t.Errorf("fileIdParam (absent) = %d, want 0", got)
	}
}
