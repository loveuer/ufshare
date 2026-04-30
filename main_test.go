package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testToken = "12345678901234567890123456789012"

func putRequest(path, token, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPut, "http://example.com"+path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func decodeUploadResponse(t *testing.T, rec *httptest.ResponseRecorder) uploadResponse {
	t.Helper()

	var resp uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode upload response: %v; body: %s", err, rec.Body.String())
	}
	return resp
}

func TestValidateUploadToken(t *testing.T) {
	if err := validateUploadToken(""); err != nil {
		t.Fatalf("empty token should disable upload without error: %v", err)
	}
	if err := validateUploadToken("short"); err == nil {
		t.Fatal("short token should fail validation")
	}
	if err := validateUploadToken(testToken); err != nil {
		t.Fatalf("valid token should pass validation: %v", err)
	}
}

func TestUploadDisabled(t *testing.T) {
	baseDir := t.TempDir()
	rec := httptest.NewRecorder()

	handleRequest(rec, putRequest("/file.txt", testToken, "content"), baseDir, false, "")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("disabled upload should not create file, stat err: %v", err)
	}
}

func TestUploadUnauthorized(t *testing.T) {
	baseDir := t.TempDir()

	for name, req := range map[string]*http.Request{
		"missing": putRequest("/file.txt", "", "content"),
		"wrong":   putRequest("/file.txt", "wrong-token-wrong-token-wrong-token", "content"),
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			handleRequest(rec, req, baseDir, false, testToken)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestUploadCreatesParentDirsAndOverwrites(t *testing.T) {
	baseDir := t.TempDir()
	target := filepath.Join(baseDir, "a", "b", "file.txt")

	rec := httptest.NewRecorder()
	handleRequest(rec, putRequest("/a/b/file.txt", testToken, "first"), baseDir, false, testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want %d", rec.Code, http.StatusOK)
	}
	resp := decodeUploadResponse(t, rec)
	if resp.Status != http.StatusOK || resp.Action != "uploaded" {
		t.Fatalf("response = %+v, want status 200 and action uploaded", resp)
	}
	if resp.File.Name != "file.txt" || resp.File.Path != "a/b/file.txt" || resp.File.URL != "http://example.com/a/b/file.txt" || resp.File.Size != 5 {
		t.Fatalf("file response = %+v", resp.File)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	if string(content) != "first" {
		t.Fatalf("content = %q, want %q", content, "first")
	}

	rec = httptest.NewRecorder()
	handleRequest(rec, putRequest("/a/b/file.txt", testToken, "second"), baseDir, false, testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("overwrite status = %d, want %d", rec.Code, http.StatusOK)
	}
	resp = decodeUploadResponse(t, rec)
	if resp.Status != http.StatusOK || resp.Action != "updated" {
		t.Fatalf("response = %+v, want status 200 and action updated", resp)
	}
	if resp.File.Size != 6 {
		t.Fatalf("file size = %d, want %d", resp.File.Size, 6)
	}
	content, err = os.ReadFile(target)
	if err != nil {
		t.Fatalf("read overwritten file: %v", err)
	}
	if string(content) != "second" {
		t.Fatalf("content = %q, want %q", content, "second")
	}
}

func TestUploadRejectsDirectoryTarget(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(baseDir, "dir"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	rec := httptest.NewRecorder()
	handleRequest(rec, putRequest("/dir", testToken, "content"), baseDir, false, testToken)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUploadHiddenPathRespectsHiddenFlag(t *testing.T) {
	baseDir := t.TempDir()

	rec := httptest.NewRecorder()
	handleRequest(rec, putRequest("/.env", testToken, "secret"), baseDir, false, testToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("hidden disabled status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	rec = httptest.NewRecorder()
	handleRequest(rec, putRequest("/.env", testToken, "secret"), baseDir, true, testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("hidden enabled status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUploadResponseUsesForwardedURL(t *testing.T) {
	baseDir := t.TempDir()
	req := putRequest("/file%20name.txt?ignored=1", testToken, "content")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "files.example.test")
	rec := httptest.NewRecorder()

	handleRequest(rec, req, baseDir, false, testToken)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	resp := decodeUploadResponse(t, rec)
	if resp.File.URL != "https://files.example.test/file%20name.txt" {
		t.Fatalf("url = %q", resp.File.URL)
	}
}

func TestUploadPathRejectsEscape(t *testing.T) {
	baseDir := t.TempDir()

	if _, _, err := uploadPath(baseDir, "../../../outside.txt"); err == nil {
		t.Fatal("relative traversal should be rejected")
	}
}

func TestServeFavicon(t *testing.T) {
	baseDir := t.TempDir()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/favicon.svg", nil)
	rec := httptest.NewRecorder()

	handleRequest(rec, req, baseDir, false, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/svg+xml; charset=utf-8" {
		t.Fatalf("content-type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "<svg") {
		t.Fatalf("favicon response is not svg: %q", rec.Body.String())
	}
}
