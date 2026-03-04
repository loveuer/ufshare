package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed templates/index.html
var htmlTemplate string

func main() {
	host := flag.String("host", "0.0.0.0", "监听主机")
	port := flag.String("port", "8000", "监听端口")
	dir := flag.String("dir", ".", "暴露的目录")
	hidden := flag.Bool("hidden", false, "显示以 . 开头的隐藏文件")
	flag.Parse()

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("获取目录绝对路径失败: %v", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(w, r, absDir, *hidden)
	})

	addr := fmt.Sprintf("%s:%s", *host, *port)
	log.Printf("在 %s 启动服务，暴露目录: %s", addr, absDir)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleRequest(w http.ResponseWriter, r *http.Request, baseDir string, showHidden bool) {
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

	if r.Method != "GET" {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	path := filepath.Clean(r.URL.Path)
	relPath := strings.TrimPrefix(path, "/")
	fullPath := filepath.Join(baseDir, relPath)

	// 隐藏文件保护：路径中任意分段以 . 开头时，未开启 -hidden 则返回 404
	if !showHidden {
		for seg := range strings.SplitSeq(relPath, "/") {
			if strings.HasPrefix(seg, ".") {
				http.NotFound(w, r)
				return
			}
		}
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
	switch ext {
	case ".pdf", ".md":
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
		if ext == ".md" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		w.Header().Set("Content-Disposition", "inline")
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	default:
		http.Error(w, "不支持预览该文件类型", http.StatusBadRequest)
	}
}
