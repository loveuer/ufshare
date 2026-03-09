package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestNpm_Ping 验证服务基本可达
func TestNpm_Ping(t *testing.T) {
	srv := newTestServer(t)

	resp, err := http.Get(srv.BaseURL + "/npm/-/ping")
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestNpm_GetPackument_Proxy 请求一个本地不存在的包，应代理上游并返回完整 packument
func TestNpm_GetPackument_Proxy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test (-short)")
	}

	srv := newTestServer(t)

	resp, err := http.Get(srv.BaseURL + "/npm/lodash")
	if err != nil {
		t.Fatalf("get packument: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var pack struct {
		Name     string                     `json:"name"`
		Versions map[string]json.RawMessage `json:"versions"`
		DistTags map[string]string          `json:"dist-tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pack); err != nil {
		t.Fatalf("decode packument: %v", err)
	}
	if pack.Name != "lodash" {
		t.Errorf("name: want lodash, got %q", pack.Name)
	}
	if len(pack.Versions) == 0 {
		t.Error("versions should not be empty")
	}
	latest := pack.DistTags["latest"]
	if latest == "" {
		t.Error("dist-tags.latest should be set")
	}
	t.Logf("lodash: %d versions, latest=%s", len(pack.Versions), latest)
}

// TestNpm_TarballURL_Rewritten 确认 packument 中 tarball URL 已改写为本地地址
func TestNpm_TarballURL_Rewritten(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test (-short)")
	}

	srv := newTestServer(t)

	resp, err := http.Get(srv.BaseURL + "/npm/lodash")
	if err != nil {
		t.Fatalf("get packument: %v", err)
	}
	defer resp.Body.Close()

	var pack struct {
		Versions map[string]struct {
			Dist struct {
				Tarball string `json:"tarball"`
			} `json:"dist"`
		} `json:"versions"`
		DistTags map[string]string `json:"dist-tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pack); err != nil {
		t.Fatalf("decode: %v", err)
	}

	latest := pack.DistTags["latest"]
	ver, ok := pack.Versions[latest]
	if !ok {
		t.Fatalf("latest version %s not found in versions map", latest)
	}

	tarball := ver.Dist.Tarball
	if !strings.HasPrefix(tarball, srv.BaseURL) {
		t.Errorf("tarball URL not rewritten: expected prefix %s, got %s", srv.BaseURL, tarball)
	}
	t.Logf("tarball URL correctly rewritten: %s", tarball)
}

// TestNpm_GetTarball_Proxy 下载 tarball，应代理上游并缓存到磁盘
func TestNpm_GetTarball_Proxy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test (-short)")
	}

	srv := newTestServer(t)

	// 先拿 packument 确认 latest
	resp, err := http.Get(srv.BaseURL + "/npm/lodash")
	if err != nil {
		t.Fatalf("get packument: %v", err)
	}
	var pack struct {
		DistTags map[string]string `json:"dist-tags"`
	}
	json.NewDecoder(resp.Body).Decode(&pack)
	resp.Body.Close()

	latest := pack.DistTags["latest"]
	if latest == "" {
		t.Fatal("could not determine latest version")
	}

	tarballURL := fmt.Sprintf("%s/npm/lodash/-/lodash-%s.tgz", srv.BaseURL, latest)
	resp2, err := http.Get(tarballURL)
	if err != nil {
		t.Fatalf("get tarball: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 200 {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200, got %d: %s", resp2.StatusCode, body)
	}

	data, _ := io.ReadAll(resp2.Body)
	if len(data) < 1024 {
		t.Errorf("tarball suspiciously small: %d bytes", len(data))
	}
	t.Logf("lodash@%s tarball: %d bytes", latest, len(data))
}

// TestNpm_GetPackument_Cache 第二次请求应从本地 DB 返回，不再访问上游
func TestNpm_GetPackument_Cache(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test (-short)")
	}

	srv := newTestServer(t)

	// 第一次：代理上游
	r1, err := http.Get(srv.BaseURL + "/npm/axios")
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	io.Copy(io.Discard, r1.Body)
	r1.Body.Close()
	if r1.StatusCode != 200 {
		t.Fatalf("first request failed: %d", r1.StatusCode)
	}

	// 第二次：应命中本地缓存
	r2, err := http.Get(srv.BaseURL + "/npm/axios")
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	defer r2.Body.Close()
	if r2.StatusCode != 200 {
		t.Fatalf("second request failed: %d", r2.StatusCode)
	}

	var pack struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r2.Body).Decode(&pack)
	if pack.Name != "axios" {
		t.Errorf("name: want axios, got %q", pack.Name)
	}
}

// TestNpm_ListPackages_API 代理包后通过管理 API 能查到该包
func TestNpm_ListPackages_API(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test (-short)")
	}

	srv := newTestServer(t)

	// 触发代理
	r, _ := http.Get(srv.BaseURL + "/npm/lodash")
	if r != nil {
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
	}

	// 查询管理 API
	req, _ := http.NewRequest("GET", srv.BaseURL+"/api/v1/npm/packages", nil)
	req.Header.Set("Authorization", "Bearer "+srv.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list packages: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int `json:"code"`
		Data []struct {
			Name         string `json:"name"`
			VersionCount int    `json:"version_count"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Code != 0 {
		t.Errorf("expected code=0, got %d", result.Code)
	}

	found := false
	for _, p := range result.Data {
		if p.Name == "lodash" {
			found = true
			t.Logf("lodash in list: %d versions", p.VersionCount)
			break
		}
	}
	if !found {
		t.Error("lodash not found in package list after proxy")
	}
}

// TestNpm_PackageNotFound 请求不存在的包应返回 404
func TestNpm_PackageNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test (-short)")
	}

	srv := newTestServer(t)

	resp, err := http.Get(srv.BaseURL + "/npm/this-package-absolutely-does-not-exist-xyzxyz123")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}
