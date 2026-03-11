package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// tokenEntry 缓存的 token
type tokenEntry struct {
	token     string
	expiresAt time.Time
}

// tokenCache 按 scope 缓存上游 registry token
type tokenCache struct {
	mu    sync.RWMutex
	cache map[string]tokenEntry
}

func newTokenCache() *tokenCache {
	return &tokenCache{
		cache: make(map[string]tokenEntry),
	}
}

func (tc *tokenCache) get(scope string) (string, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	entry, ok := tc.cache[scope]
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.token, true
}

func (tc *tokenCache) set(scope, token string, expiresIn int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	ttl := time.Duration(expiresIn) * time.Second
	if ttl <= 0 {
		ttl = 300 * time.Second // 默认 5 分钟
	}
	// 提前 30 秒过期，避免边界条件
	tc.cache[scope] = tokenEntry{
		token:     token,
		expiresAt: time.Now().Add(ttl - 30*time.Second),
	}
}

// upstreamAuth 处理上游 registry 的 token 认证
type upstreamAuth struct {
	client     func() *http.Client
	tokenCache *tokenCache
}

func newUpstreamAuth(clientFn func() *http.Client) *upstreamAuth {
	return &upstreamAuth{
		client:     clientFn,
		tokenCache: newTokenCache(),
	}
}

// doWithAuth 执行带认证的 HTTP 请求
// 流程：
// 1. 尝试直接请求（可能用缓存 token）
// 2. 收到 401 → 解析 WWW-Authenticate → 获取新 token → 重试
func (a *upstreamAuth) doWithAuth(ctx context.Context, method, requestURL, scope string) (*http.Response, error) {
	// 尝试用缓存 token
	if token, ok := a.tokenCache.get(scope); ok {
		resp, err := a.doRequest(ctx, method, requestURL, token)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 401 {
			return resp, nil
		}
		resp.Body.Close()
	}

	// 无缓存 token 或 token 已失效，先发一个探测请求
	resp, err := a.doRequest(ctx, method, requestURL, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 401 {
		return resp, nil
	}

	// 解析 WWW-Authenticate 获取 token
	wwwAuth := resp.Header.Get("Www-Authenticate")
	resp.Body.Close()

	token, err := a.fetchToken(ctx, wwwAuth, scope)
	if err != nil {
		return nil, fmt.Errorf("auth token: %w", err)
	}

	// 用新 token 重试
	return a.doRequest(ctx, method, requestURL, token)
}

// doRequest 发送 HTTP 请求
func (a *upstreamAuth) doRequest(ctx context.Context, method, requestURL, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	// Docker client 兼容 Accept 头
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
		"*/*",
	}, ", "))
	return a.client().Do(req)
}

// fetchToken 从 WWW-Authenticate 头解析认证信息并获取 token
// 格式：Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/nginx:pull"
func (a *upstreamAuth) fetchToken(ctx context.Context, wwwAuth, scope string) (string, error) {
	params := parseWWWAuthenticate(wwwAuth)
	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("no realm in WWW-Authenticate: %s", wwwAuth)
	}

	svc := params["service"]

	// 如果 scope 为空，从 WWW-Authenticate 中取
	if scope == "" {
		scope = params["scope"]
	}

	// 构建 token 请求 URL
	tokenURL := fmt.Sprintf("%s?service=%s", realm, svc)
	if scope != "" {
		tokenURL += "&scope=" + scope
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := a.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	token := tokenResp.Token
	if token == "" {
		token = tokenResp.AccessToken
	}
	if token == "" {
		return "", fmt.Errorf("empty token in response")
	}

	a.tokenCache.set(scope, token, tokenResp.ExpiresIn)
	return token, nil
}

// parseWWWAuthenticate 解析 WWW-Authenticate 头
// 输入：Bearer realm="https://...",service="...",scope="..."
// 输出：map[realm:https://... service:... scope:...]
func parseWWWAuthenticate(header string) map[string]string {
	result := make(map[string]string)
	// 去掉 "Bearer " 前缀
	header = strings.TrimSpace(header)
	if idx := strings.Index(header, " "); idx > 0 {
		header = header[idx+1:]
	}

	for _, part := range splitRespectingQuotes(header, ',') {
		part = strings.TrimSpace(part)
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(part[:eq])
		val := strings.TrimSpace(part[eq+1:])
		val = strings.Trim(val, `"`)
		result[key] = val
	}
	return result
}

// splitRespectingQuotes 按分隔符分割，但忽略引号内的分隔符
func splitRespectingQuotes(s string, sep byte) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	for i := range len(s) {
		c := s[i]
		if c == '"' {
			inQuote = !inQuote
			current.WriteByte(c)
		} else if c == sep && !inQuote {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}
