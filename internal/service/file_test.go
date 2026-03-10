package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

// setupFileService 创建一个测试用 FileService（内存 SQLite + 临时目录）
func setupFileService(t *testing.T) (*FileService, string) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.FileEntry{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	dataDir := t.TempDir()
	svc := NewFileService(db, dataDir)
	return svc, dataDir
}

// sha256hex 计算内容的 SHA256 十六进制字符串
func sha256hex(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// ─── Upload ─────────────────────────────────────────────────────────────────

func TestUpload_SimpleFile(t *testing.T) {
	svc, dataDir := setupFileService(t)
	content := []byte("hello ufshare")

	entry, err := svc.Upload(context.Background(), "hello.txt", bytes.NewReader(content), 1, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Path != "hello.txt" {
		t.Errorf("path = %q, want %q", entry.Path, "hello.txt")
	}
	if entry.Size != int64(len(content)) {
		t.Errorf("size = %d, want %d", entry.Size, len(content))
	}
	if entry.SHA256 != sha256hex(content) {
		t.Errorf("sha256 mismatch: got %s", entry.SHA256)
	}
	if entry.Uploader != "alice" {
		t.Errorf("uploader = %q, want %q", entry.Uploader, "alice")
	}

	// 验证磁盘文件真实存在且内容正确
	diskPath := filepath.Join(dataDir, "file-store", "hello.txt")
	got, err := os.ReadFile(diskPath)
	if err != nil {
		t.Fatalf("read disk file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("disk content mismatch")
	}
}

func TestUpload_LeadingSlash(t *testing.T) {
	// handler 从 ursa catchAll param 拿到的 path 带前导 /
	svc, _ := setupFileService(t)
	content := []byte("leading slash test")

	entry, err := svc.Upload(context.Background(), "/leading.txt", bytes.NewReader(content), 1, "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// normalizePath 应去掉前导 /，存储为 "leading.txt"
	if entry.Path != "leading.txt" {
		t.Errorf("path = %q, want %q", entry.Path, "leading.txt")
	}
}

func TestUpload_NestedPath(t *testing.T) {
	svc, dataDir := setupFileService(t)
	content := []byte("nested content")

	entry, err := svc.Upload(context.Background(), "v1.0/releases/app.tar.gz", bytes.NewReader(content), 1, "ci")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Path != "v1.0/releases/app.tar.gz" {
		t.Errorf("path = %q", entry.Path)
	}

	diskPath := filepath.Join(dataDir, "file-store", "v1.0", "releases", "app.tar.gz")
	if _, err := os.Stat(diskPath); err != nil {
		t.Errorf("disk file not found: %v", err)
	}
}

func TestUpload_Overwrite(t *testing.T) {
	svc, _ := setupFileService(t)

	_, err := svc.Upload(context.Background(), "data.bin", bytes.NewReader([]byte("v1")), 1, "user")
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}

	newContent := []byte("v2-updated")
	entry, err := svc.Upload(context.Background(), "data.bin", bytes.NewReader(newContent), 2, "user2")
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if entry.Size != int64(len(newContent)) {
		t.Errorf("size after overwrite = %d, want %d", entry.Size, len(newContent))
	}
	if entry.SHA256 != sha256hex(newContent) {
		t.Errorf("sha256 after overwrite mismatch")
	}
}

func TestUpload_EmptyPath(t *testing.T) {
	svc, _ := setupFileService(t)

	_, err := svc.Upload(context.Background(), "", bytes.NewReader([]byte("x")), 1, "u")
	if err == nil {
		t.Fatal("expected ErrInvalidPath, got nil")
	}
	if err != ErrInvalidPath {
		t.Errorf("expected ErrInvalidPath, got %v", err)
	}
}

func TestUpload_PathTraversal(t *testing.T) {
	svc, _ := setupFileService(t)

	// 这些路径都应被拒绝
	mustFail := []string{
		"../evil.txt",
		"../../etc/passwd",
		"foo/../../etc/shadow",
		"//etc/passwd", // 双斜杠：normalizePath 去掉一个后仍为绝对路径
	}
	for _, p := range mustFail {
		_, err := svc.Upload(context.Background(), p, bytes.NewReader([]byte("x")), 1, "u")
		if err != ErrInvalidPath {
			t.Errorf("Upload(%q): expected ErrInvalidPath, got %v", p, err)
		}
	}
}

func TestUpload_AbsPathNormalized(t *testing.T) {
	// /etc/passwd 带单个前导斜杠时，normalizePath 去掉后变为相对路径 "etc/passwd"
	// 文件安全地存储在 dataDir/file-store/etc/passwd 内，不是路径穿越漏洞
	svc, dataDir := setupFileService(t)

	entry, err := svc.Upload(context.Background(), "/etc/passwd", bytes.NewReader([]byte("test")), 1, "u")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Path != "etc/passwd" {
		t.Errorf("path = %q, want %q", entry.Path, "etc/passwd")
	}
	// 确认文件在 dataDir 内部，无法逃出沙箱
	diskPath := filepath.Join(dataDir, "file-store", "etc", "passwd")
	if _, err := os.Stat(diskPath); err != nil {
		t.Errorf("expected file at %s, got: %v", diskPath, err)
	}
}

func TestUpload_MimeType(t *testing.T) {
	svc, _ := setupFileService(t)

	cases := []struct {
		path     string
		wantMime string
	}{
		{"image.png", "image/png"},
		{"doc.json", "application/json"},
		{"noext", "application/octet-stream"},
	}
	for _, tc := range cases {
		entry, err := svc.Upload(context.Background(), tc.path, bytes.NewReader([]byte("data")), 1, "u")
		if err != nil {
			t.Fatalf("Upload(%q): %v", tc.path, err)
		}
		if !strings.HasPrefix(entry.MimeType, tc.wantMime) {
			t.Errorf("Upload(%q): MimeType = %q, want prefix %q", tc.path, entry.MimeType, tc.wantMime)
		}
	}
}

// ─── Download ────────────────────────────────────────────────────────────────

func TestDownload_Existing(t *testing.T) {
	svc, _ := setupFileService(t)
	content := []byte("download me")

	if _, err := svc.Upload(context.Background(), "dl.txt", bytes.NewReader(content), 1, "u"); err != nil {
		t.Fatalf("upload: %v", err)
	}

	entry, diskPath, err := svc.Download(context.Background(), "dl.txt")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if entry.Path != "dl.txt" {
		t.Errorf("path = %q", entry.Path)
	}
	got, err := os.ReadFile(diskPath)
	if err != nil {
		t.Fatalf("read disk: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch")
	}
}

func TestDownload_LeadingSlash(t *testing.T) {
	svc, _ := setupFileService(t)
	content := []byte("leading slash download")

	if _, err := svc.Upload(context.Background(), "leaddown.txt", bytes.NewReader(content), 1, "u"); err != nil {
		t.Fatalf("upload: %v", err)
	}

	// handler 传入的 path 带前导 /
	entry, _, err := svc.Download(context.Background(), "/leaddown.txt")
	if err != nil {
		t.Fatalf("download with leading slash: %v", err)
	}
	if entry.Path != "leaddown.txt" {
		t.Errorf("path = %q, want %q", entry.Path, "leaddown.txt")
	}
}

func TestDownload_NotFound(t *testing.T) {
	svc, _ := setupFileService(t)

	_, _, err := svc.Download(context.Background(), "ghost.txt")
	if err != ErrFileNotFound {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestDownload_EmptyPath(t *testing.T) {
	svc, _ := setupFileService(t)

	_, _, err := svc.Download(context.Background(), "")
	if err != ErrInvalidPath {
		t.Errorf("expected ErrInvalidPath, got %v", err)
	}
}

func TestDownload_PathTraversal(t *testing.T) {
	svc, _ := setupFileService(t)

	_, _, err := svc.Download(context.Background(), "../secret")
	if err != ErrInvalidPath {
		t.Errorf("expected ErrInvalidPath, got %v", err)
	}
}

func TestDownload_DBExistsButDiskMissing(t *testing.T) {
	svc, dataDir := setupFileService(t)
	content := []byte("ghost disk")

	if _, err := svc.Upload(context.Background(), "orphan.txt", bytes.NewReader(content), 1, "u"); err != nil {
		t.Fatalf("upload: %v", err)
	}

	// 手动删除磁盘文件，模拟文件丢失
	diskPath := filepath.Join(dataDir, "file-store", "orphan.txt")
	if err := os.Remove(diskPath); err != nil {
		t.Fatalf("remove: %v", err)
	}

	_, _, err := svc.Download(context.Background(), "orphan.txt")
	if err != ErrFileNotFound {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

// ─── Delete ──────────────────────────────────────────────────────────────────

func TestDelete_Existing(t *testing.T) {
	svc, dataDir := setupFileService(t)

	if _, err := svc.Upload(context.Background(), "todel.bin", bytes.NewReader([]byte("bye")), 1, "u"); err != nil {
		t.Fatalf("upload: %v", err)
	}

	if err := svc.Delete(context.Background(), "todel.bin"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// DB 记录应已删除
	_, _, err := svc.Download(context.Background(), "todel.bin")
	if err != ErrFileNotFound {
		t.Errorf("after delete, download expected ErrFileNotFound, got %v", err)
	}

	// 磁盘文件应已删除
	diskPath := filepath.Join(dataDir, "file-store", "todel.bin")
	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Errorf("disk file should not exist after delete")
	}
}

func TestDelete_LeadingSlash(t *testing.T) {
	svc, _ := setupFileService(t)

	if _, err := svc.Upload(context.Background(), "delslash.txt", bytes.NewReader([]byte("x")), 1, "u"); err != nil {
		t.Fatalf("upload: %v", err)
	}

	if err := svc.Delete(context.Background(), "/delslash.txt"); err != nil {
		t.Fatalf("delete with leading slash: %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	svc, _ := setupFileService(t)

	err := svc.Delete(context.Background(), "nonexistent.txt")
	if err != ErrFileNotFound {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestDelete_EmptyPath(t *testing.T) {
	svc, _ := setupFileService(t)

	err := svc.Delete(context.Background(), "")
	if err != ErrInvalidPath {
		t.Errorf("expected ErrInvalidPath, got %v", err)
	}
}

// ─── List ────────────────────────────────────────────────────────────────────

func TestList_Empty(t *testing.T) {
	svc, _ := setupFileService(t)

	entries, err := svc.List(context.Background(), "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0, got %d", len(entries))
	}
}

func TestList_AllFiles(t *testing.T) {
	svc, _ := setupFileService(t)

	files := []string{"a.txt", "b/c.txt", "d.bin"}
	for _, f := range files {
		if _, err := svc.Upload(context.Background(), f, bytes.NewReader([]byte("x")), 1, "u"); err != nil {
			t.Fatalf("upload %q: %v", f, err)
		}
	}

	entries, err := svc.List(context.Background(), "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != len(files) {
		t.Errorf("expected %d, got %d", len(files), len(entries))
	}
}

func TestList_PrefixFilter(t *testing.T) {
	svc, _ := setupFileService(t)

	uploads := []string{"v1/a.txt", "v1/b.txt", "v2/c.txt"}
	for _, f := range uploads {
		if _, err := svc.Upload(context.Background(), f, bytes.NewReader([]byte("x")), 1, "u"); err != nil {
			t.Fatalf("upload %q: %v", f, err)
		}
	}

	entries, err := svc.List(context.Background(), "v1/")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2, got %d", len(entries))
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Path, "v1/") {
			t.Errorf("unexpected path in result: %q", e.Path)
		}
	}
}

// ─── validatePath ────────────────────────────────────────────────────────────

func TestValidatePath(t *testing.T) {
	cases := []struct {
		path    string
		wantErr bool
	}{
		{"hello.txt", false},
		{"sub/dir/file.bin", false},
		{"", true},
		{"..", true},
		{"../foo", true},
		{"/absolute", true},
		{"a/../../b", true},
	}
	for _, tc := range cases {
		err := validatePath(tc.path)
		if tc.wantErr && err == nil {
			t.Errorf("validatePath(%q): expected error, got nil", tc.path)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("validatePath(%q): unexpected error: %v", tc.path, err)
		}
	}
}
