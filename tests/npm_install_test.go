package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// popularPackageJSON 模拟常见前端项目的 dependencies
// 参考 github.com/pmndrs/zustand, github.com/TanStack/query 等流行仓库的依赖组合
const popularPackageJSON = `{
  "name": "ufshare-frontend-install-test",
  "version": "1.0.0",
  "private": true,
  "description": "Integration test: npm install via ufshare proxy (modeled after popular frontend projects)",
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "axios": "^1.6.8",
    "lodash": "^4.17.21",
    "dayjs": "^1.11.10"
  }
}
`

// TestNpm_Install_PopularFrontend 启动服务，用 npm install 安装主流前端依赖，验证代理和缓存
func TestNpm_Install_PopularFrontend(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping npm install network test (-short)")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not in PATH")
	}

	srv := newTestServer(t)

	// 创建隔离的项目目录和 npm 缓存目录
	projectDir := t.TempDir()
	cacheDir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(projectDir, "package.json"),
		[]byte(popularPackageJSON),
		0644,
	); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	registry := srv.BaseURL + "/npm"
	t.Logf("registry : %s", registry)
	t.Logf("project  : %s", projectDir)

	cmd := exec.Command("npm", "install",
		"--registry", registry,
		"--cache", cacheDir,
		"--prefer-online",
		"--no-audit",
		"--no-fund",
		"--loglevel", "http",
	)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	t.Logf("--- npm install output ---\n%s", out)

	if err != nil {
		t.Fatalf("npm install failed: %v", err)
	}

	// 验证每个顶层包都安装成功
	topLevel := []string{"react", "react-dom", "axios", "lodash", "dayjs"}
	for _, pkg := range topLevel {
		dir := filepath.Join(projectDir, "node_modules", pkg)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("package %q missing from node_modules", pkg)
		} else {
			t.Logf("✓ %s", pkg)
		}
	}

	// 验证这些包已被 ufshare 记录（metadata 缓存）
	req, _ := http.NewRequest("GET", srv.BaseURL+"/api/v1/npm/packages", nil)
	req.Header.Set("Authorization", "Bearer "+srv.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list packages API: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code int `json:"code"`
		Data []struct {
			Name         string `json:"name"`
			VersionCount int    `json:"version_count"`
			CachedCount  int    `json:"cached_count"`
		} `json:"data"`
	}
	json.Unmarshal(body, &result)

	t.Logf("--- ufshare cached packages (%d total) ---", len(result.Data))
	cached := map[string]bool{}
	for _, p := range result.Data {
		cached[p.Name] = true
		t.Logf("  %-30s  versions=%d  tarballs_cached=%d", p.Name, p.VersionCount, p.CachedCount)
	}

	for _, pkg := range topLevel {
		if !cached[pkg] {
			t.Errorf("package %q not found in ufshare after install", pkg)
		}
	}
}
