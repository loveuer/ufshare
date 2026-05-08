package main

import (
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	godaemon "github.com/sevlyar/go-daemon"
)

//go:embed templates/index.html
var htmlTemplate string

//go:embed templates/favicon.svg
var faviconSVG string

const defaultMaxUploadSize = 1 << 30

type uploadResponse struct {
	Status int                `json:"status"`
	Action string             `json:"action"`
	File   uploadResponseFile `json:"file"`
}

type apiErrorResponse struct {
	Status int    `json:"status"`
	Error  string `json:"error"`
}

type uploadResponseFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	URL     string `json:"url"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func main() {
	host := flag.String("host", "0.0.0.0", "监听主机")
	port := flag.String("port", "8000", "监听端口")
	dir := flag.String("dir", ".", "暴露的目录")
	hidden := flag.Bool("hidden", false, "显示以 . 开头的隐藏文件")
	daemon := flag.Bool("daemon", false, "以守护进程模式运行")
	pidFile := flag.String("pidfile", "ufshare.pid", "PID 文件路径")
	logFile := flag.String("logfile", "ufshare.log", "日志文件路径")
	flag.Parse()

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("获取目录绝对路径失败: %v", err)
	}

	uploadToken := os.Getenv("UFSHARE_TOKEN")
	if err := validateUploadToken(uploadToken); err != nil {
		log.Fatal(err)
	}
	maxUploadSize, err := loadMaxUploadSize()
	if err != nil {
		log.Fatal(err)
	}

	// 守护进程模式
	if *daemon {
		cntxt := &godaemon.Context{
			PidFileName: *pidFile,
			PidFilePerm: 0644,
			LogFileName: *logFile,
			LogFilePerm: 0640,
			WorkDir:     "./",
			Umask:       027,
		}

		d, err := cntxt.Reborn()
		if err != nil {
			log.Fatalf("无法启动守护进程: %v", err)
		}
		if d != nil {
			// 父进程退出，打印子进程 PID
			fmt.Printf("守护进程已启动，PID: %d\n", d.Pid)
			fmt.Printf("PID 文件: %s\n", *pidFile)
			fmt.Printf("日志文件: %s\n", *logFile)
			return
		}
		defer cntxt.Release()

		log.Printf("守护进程已启动，PID: %d", os.Getpid())
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(w, r, absDir, *hidden, uploadToken, maxUploadSize)
	})

	addr := fmt.Sprintf("%s:%s", *host, *port)
	log.Printf("在 %s 启动服务，暴露目录: %s", addr, absDir)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func validateUploadToken(token string) error {
	if token != "" && len(token) < 32 {
		return fmt.Errorf("UFSHARE_TOKEN 长度必须至少为 32")
	}
	return nil
}

func loadMaxUploadSize() (int64, error) {
	value := strings.TrimSpace(os.Getenv("UFSHARE_MAX_UPLOAD_SIZE"))
	if value == "" {
		return defaultMaxUploadSize, nil
	}

	size, err := strconv.ParseInt(value, 10, 64)
	if err != nil || size <= 0 {
		return 0, fmt.Errorf("UFSHARE_MAX_UPLOAD_SIZE 必须是大于 0 的字节数")
	}
	return size, nil
}

func handleRequest(w http.ResponseWriter, r *http.Request, baseDir string, showHidden bool, uploadToken string, maxUploadSize int64) {
	start := time.Now()
	defer func() {
		ip := getClientIP(r)
		duration := time.Since(start)
		log.Printf("[%s] %s %s %s %v",
			time.Now().Format("2006-01-02 15:04:05"),
			ip,
			r.Method,
			r.URL.Path,
			duration,
		)
	}()

	switch r.Method {
	case http.MethodGet:
		if r.URL.Path == "/favicon.svg" {
			serveFavicon(w)
			return
		}
		handleGet(w, r, baseDir, showHidden)
	case http.MethodHead:
		handleGet(w, r, baseDir, showHidden)
	case http.MethodPut:
		handleUpload(w, r, baseDir, showHidden, uploadToken, maxUploadSize)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

func serveFavicon(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	fmt.Fprint(w, faviconSVG)
}

func handleGet(w http.ResponseWriter, r *http.Request, baseDir string, showHidden bool) {
	path := filepath.Clean(r.URL.Path)
	relPath := strings.TrimPrefix(path, "/")
	fullPath := filepath.Join(baseDir, relPath)

	// 隐藏文件保护：路径中任意分段以 . 开头时，未开启 -hidden 则返回 404
	if !showHidden && hasHiddenSegment(relPath) {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if info.IsDir() {
		serveFileList(w, baseDir, relPath, showHidden)
		return
	}

	if r.URL.Query().Get("preview") == "1" {
		servePreview(w, r, fullPath, relPath)
		return
	}

	serveFile(w, r, baseDir, relPath)
}

func handleUpload(w http.ResponseWriter, r *http.Request, baseDir string, showHidden bool, uploadToken string, maxUploadSize int64) {
	if uploadToken == "" {
		writeAPIError(w, http.StatusMethodNotAllowed, "upload_disabled")
		return
	}

	if !authorized(r.Header.Get("Authorization"), uploadToken) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAPIError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if r.ContentLength > maxUploadSize {
		writeAPIError(w, http.StatusRequestEntityTooLarge, "upload_too_large")
		return
	}

	relPath, fullPath, err := uploadPath(baseDir, r.URL.Path)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_upload_path")
		return
	}

	if !showHidden && hasHiddenSegment(relPath) {
		writeAPIError(w, http.StatusNotFound, "not_found")
		return
	}

	action := "uploaded"
	if info, err := os.Stat(fullPath); err == nil {
		if info.IsDir() {
			writeAPIError(w, http.StatusBadRequest, "target_is_directory")
			return
		}
		action = "updated"
	} else if err != nil && !os.IsNotExist(err) {
		writeAPIError(w, http.StatusInternalServerError, "stat_target_failed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := saveUpload(fullPath, r.Body); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "upload_too_large")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "save_upload_failed")
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "stat_upload_failed")
		return
	}

	writeUploadResponse(w, r, relPath, action, info)
}

func writeUploadResponse(w http.ResponseWriter, r *http.Request, relPath, action string, info os.FileInfo) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	resp := uploadResponse{
		Status: http.StatusOK,
		Action: action,
		File: uploadResponseFile{
			Name:    info.Name(),
			Path:    relPath,
			URL:     downloadURL(r, relPath),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		},
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("写入上传响应失败: %v", err)
	}
}

func writeAPIError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	resp := apiErrorResponse{
		Status: status,
		Error:  code,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("写入 API 错误响应失败: %v", err)
	}
}

func downloadURL(r *http.Request, relPath string) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	u := *r.URL
	u.Scheme = scheme
	u.Host = host
	u.Path = "/" + relPath
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func authorized(header, token string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got := strings.TrimPrefix(header, prefix)
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

func uploadPath(baseDir, requestPath string) (string, string, error) {
	cleanPath := filepath.Clean(requestPath)
	relPath := strings.TrimPrefix(cleanPath, "/")
	if relPath == "" || relPath == "." {
		return "", "", fmt.Errorf("上传路径不能为空")
	}

	fullPath := filepath.Join(baseDir, relPath)
	relToBase, err := filepath.Rel(baseDir, fullPath)
	if err != nil {
		return "", "", fmt.Errorf("上传路径无效")
	}
	if relToBase == "." || strings.HasPrefix(relToBase, ".."+string(filepath.Separator)) || relToBase == ".." {
		return "", "", fmt.Errorf("上传路径不能超出共享目录")
	}

	return relPath, fullPath, nil
}

func hasHiddenSegment(relPath string) bool {
	for seg := range strings.SplitSeq(relPath, "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

func saveUpload(fullPath string, body io.Reader) error {
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".ufshare-upload-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, fullPath)
}

func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	realIP := r.Header.Get("X-Real-Ip")
	if realIP != "" {
		return realIP
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func serveFileList(w http.ResponseWriter, baseDir, relPath string, showHidden bool) {
	fullPath := filepath.Join(baseDir, relPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, "读取目录失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, htmlTemplate)

	fmt.Fprint(w, `<script>
const currentPath = "`+escapeJS(relPath)+`";
const dirs = [`)
	first := true
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !showHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !first {
			fmt.Fprint(w, ",")
		}
		first = false
		name := entry.Name()
		modTime := info.ModTime().Format("2006-01-02 15:04")
		fmt.Fprintf(w, `{"name":"%s","time":"%s"}`,
			escapeJS(name), modTime)
	}
	fmt.Fprint(w, `];
const files = [`)
	first = true
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !showHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !first {
			fmt.Fprint(w, ",")
		}
		first = false
		name := entry.Name()
		size := formatSize(info.Size())
		modTime := info.ModTime().Format("2006-01-02 15:04")
		fmt.Fprintf(w, `{"name":"%s","size":"%s","time":"%s"}`,
			escapeJS(name), size, modTime)
	}
	fmt.Fprint(w, `];
renderFiles(dirs, files);
</script>`)
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func escapeJS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}

func serveFile(w http.ResponseWriter, r *http.Request, baseDir, filename string) {
	fileServer := http.FileServer(http.Dir(baseDir))
	r.URL.Path = "/" + filename
	fileServer.ServeHTTP(w, r)
}

func servePreview(w http.ResponseWriter, r *http.Request, fullPath, relPath string) {
	ext := strings.ToLower(filepath.Ext(relPath))
	textExts := map[string]bool{
		".txt": true, ".log": true, ".csv": true,
		".json": true, ".yaml": true, ".yml": true, ".toml": true,
		".xml": true, ".html": true, ".htm": true,
		".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".py": true, ".go": true, ".rb": true, ".rs": true,
		".java": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".sh": true, ".bash": true, ".zsh": true,
		".css": true, ".scss": true, ".less": true,
		".sql": true, ".ini": true, ".conf": true, ".env": true,
		".md": true,
	}
	switch {
	case ext == ".pdf" ||
		ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" || ext == ".svg" ||
		ext == ".mp4" || ext == ".webm" ||
		ext == ".mp3" || ext == ".wav" || ext == ".flac" || ext == ".ogg":
		f, err := os.Open(fullPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			http.Error(w, "文件错误", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Disposition", "inline")
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	case textExts[ext]:
		f, err := os.Open(fullPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			http.Error(w, "文件错误", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", "inline")
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	default:
		http.Error(w, "不支持预览该文件类型", http.StatusBadRequest)
	}
}
