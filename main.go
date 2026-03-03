package main

import (
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

func main() {
	host := flag.String("host", "0.0.0.0", "监听主机")
	port := flag.String("port", "8000", "监听端口")
	dir := flag.String("dir", ".", "暴露的目录")
	flag.Parse()

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("获取目录绝对路径失败: %v", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(w, r, absDir)
	})

	addr := fmt.Sprintf("%s:%s", *host, *port)
	log.Printf("在 %s 启动服务，暴露目录: %s", addr, absDir)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleRequest(w http.ResponseWriter, r *http.Request, baseDir string) {
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

	info, err := os.Stat(fullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if info.IsDir() {
		serveFileList(w, baseDir, relPath)
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

func serveFileList(w http.ResponseWriter, baseDir, relPath string) {
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

const htmlTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>文件分享</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: linear-gradient(135deg, #a8e6cf 0%, #7fcdbb 50%, #41b6c4 100%);
            min-height: 100vh;
            padding: 40px 20px;
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
        }
        .header {
            text-align: center;
            margin-bottom: 40px;
        }
        .header h1 {
            color: #fff;
            font-size: 2.5rem;
            font-weight: 600;
            text-shadow: 0 2px 10px rgba(0,0,0,0.2);
        }
        .header p {
            color: rgba(255,255,255,0.8);
            margin-top: 10px;
            font-size: 1.1rem;
        }
        .card {
            background: rgba(255,255,255,0.95);
            border-radius: 20px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            overflow: hidden;
            backdrop-filter: blur(10px);
        }
        .toolbar {
            padding: 20px 30px;
            background: #f8f9fa;
            border-bottom: 1px solid #e9ecef;
            display: flex;
            gap: 15px;
            align-items: center;
        }
        .search-box {
            flex: 1;
            position: relative;
        }
        .search-box input {
            width: 100%;
            padding: 12px 20px 12px 45px;
            border: 2px solid #e9ecef;
            border-radius: 12px;
            font-size: 15px;
            transition: all 0.3s ease;
            background: #fff;
        }
        .search-box input:focus {
            outline: none;
            border-color: #41b6c4;
        }
        .search-box::before {
            content: "🔍";
            position: absolute;
            left: 15px;
            top: 50%;
            transform: translateY(-50%);
            font-size: 16px;
        }
        .file-list {
            padding: 10px;
        }
        .file-item {
            display: flex;
            align-items: center;
            padding: 16px 20px;
            margin: 8px;
            border-radius: 12px;
            transition: all 0.3s ease;
            cursor: pointer;
            text-decoration: none;
            color: inherit;
        }
        .file-item:hover {
            background: #e6f7f5;
            transform: translateX(5px);
        }
        .file-icon {
            width: 48px;
            height: 48px;
            border-radius: 12px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 24px;
            margin-right: 18px;
            background: linear-gradient(135deg, #7fcdbb 0%, #41b6c4 100%);
            color: white;
            flex-shrink: 0;
        }
        .file-info {
            flex: 1;
            min-width: 0;
        }
        .file-name {
            font-weight: 600;
            font-size: 16px;
            color: #2d3748;
            margin-bottom: 4px;
            word-break: break-all;
        }
        .file-meta {
            font-size: 13px;
            color: #718096;
        }
        .file-size {
            font-weight: 500;
            color: #2a9d8f;
            font-size: 14px;
            padding: 6px 12px;
            background: #e6f7f5;
            border-radius: 20px;
            white-space: nowrap;
            flex-shrink: 0;
        }
        .file-actions {
            display: flex;
            align-items: center;
            gap: 8px;
            flex-shrink: 0;
        }
        .btn-preview, .btn-download {
            display: inline-block;
            padding: 6px 14px;
            border-radius: 20px;
            font-size: 13px;
            font-weight: 500;
            text-decoration: none;
            transition: all 0.2s ease;
            white-space: nowrap;
        }
        .btn-preview {
            background: #ebf4ff;
            color: #2b6cb0;
            border: 1px solid #bee3f8;
        }
        .btn-preview:hover {
            background: #2b6cb0;
            color: white;
        }
        .btn-download {
            background: #e6f7f5;
            color: #2a9d8f;
            border: 1px solid #b2e0da;
        }
        .btn-download:hover {
            background: #2a9d8f;
            color: white;
        }
        .breadcrumb {
            padding: 15px 30px;
            background: #f8f9fa;
            border-bottom: 1px solid #e9ecef;
            font-size: 14px;
            color: #718096;
        }
        .breadcrumb a {
            color: #41b6c4;
            text-decoration: none;
            padding: 4px 8px;
            border-radius: 6px;
            transition: background 0.2s;
        }
        .breadcrumb a:hover {
            background: #e6f7f5;
        }
        .breadcrumb span {
            color: #2d3748;
            font-weight: 500;
        }
        .empty-state {
            text-align: center;
            padding: 80px 40px;
            color: #718096;
        }
        .empty-state-icon {
            font-size: 64px;
            margin-bottom: 20px;
            opacity: 0.5;
        }
        .empty-state h3 {
            font-size: 1.5rem;
            color: #2d3748;
            margin-bottom: 10px;
        }
        .footer {
            text-align: center;
            padding: 20px;
            background: #f8f9fa;
            border-top: 1px solid #e9ecef;
            color: #718096;
            font-size: 13px;
        }
        @media (max-width: 600px) {
            body {
                padding: 20px 10px;
            }
            .header h1 {
                font-size: 1.8rem;
            }
            .file-item {
                padding: 12px 15px;
            }
            .file-icon {
                width: 40px;
                height: 40px;
                font-size: 20px;
                margin-right: 12px;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📁 文件分享</h1>
            <p>快速访问和下载文件</p>
        </div>
        <div class="card">
            <div class="breadcrumb" id="breadcrumb"></div>
            <div class="toolbar">
                <div class="search-box">
                    <input type="text" id="searchInput" placeholder="搜索文件..." autocomplete="off">
                </div>
            </div>
            <div id="fileList" class="file-list"></div>
            <div class="footer">
                <span id="fileCount">0 个文件</span>
            </div>
        </div>
    </div>
    <script>
        const PREVIEWABLE = new Set(['pdf', 'md']);

        function getFileIcon(filename, isDir) {
            if (isDir) return '📁';
            const ext = filename.split('.').pop().toLowerCase();
            const icons = {
                pdf: '📄', doc: '📝', docx: '📝', txt: '📃', md: '📝',
                jpg: '🖼️', jpeg: '🖼️', png: '🖼️', gif: '🖼️', webp: '🖼️',
                mp4: '🎬', avi: '🎬', mkv: '🎬', mov: '🎬',
                mp3: '🎵', wav: '🎵', flac: '🎵',
                zip: '📦', rar: '📦', '7z': '📦', tar: '📦', gz: '📦',
                js: '💻', ts: '💻', html: '🌐', css: '🎨', py: '🐍', go: '🔵',
                json: '📋', xml: '📋', yaml: '📋', yml: '📋',
                exe: '⚙️', dmg: '💿', iso: '💿'
            };
            return icons[ext] || '📎';
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        function buildPath(basePath, name) {
            const parts = basePath ? basePath.split('/').filter(Boolean) : [];
            parts.push(name);
            return parts.map(encodeURIComponent).join('/');
        }

        function buildBreadcrumb(path) {
            const container = document.getElementById('breadcrumb');
            if (!path) {
                container.innerHTML = '<span>📁 根目录</span>';
                return;
            }
            const parts = path.split('/').filter(p => p);
            let html = '<a href="/">📁 根目录</a>';
            let currentPath = '';
            for (let i = 0; i < parts.length; i++) {
                currentPath += '/' + encodeURIComponent(parts[i]);
                if (i === parts.length - 1) {
                    html += ' / <span>' + escapeHtml(parts[i]) + '</span>';
                } else {
                    html += ' / <a href="' + currentPath + '">' + escapeHtml(parts[i]) + '</a>';
                }
            }
            container.innerHTML = html;
        }

        let allDirs = [];
        let allFiles = [];

        function renderFiles(dirs, files) {
            allDirs = dirs || [];
            allFiles = files || [];
            const container = document.getElementById('fileList');
            const countEl = document.getElementById('fileCount');
            const query = document.getElementById('searchInput').value.toLowerCase();

            buildBreadcrumb(currentPath);

            const displayDirs = query ? allDirs.filter(d => d.name.toLowerCase().includes(query)) : allDirs;
            const displayFiles = query ? allFiles.filter(f => f.name.toLowerCase().includes(query)) : allFiles;

            if (displayDirs.length === 0 && displayFiles.length === 0) {
                container.innerHTML = '<div class="empty-state"><div class="empty-state-icon">📂</div><h3>暂无内容</h3><p>该目录下还没有任何文件或文件夹</p></div>';
                countEl.textContent = '0 个项目';
                return;
            }

            let html = '';

            if (displayDirs.length > 0) {
                html += displayDirs.map(d => {
                    const href = '/' + buildPath(currentPath, d.name);
                    return '<a href="' + href + '" class="file-item" title="打开文件夹">' +
                        '<div class="file-icon">' + getFileIcon(d.name, true) + '</div>' +
                        '<div class="file-info">' +
                            '<div class="file-name">' + escapeHtml(d.name) + '</div>' +
                            '<div class="file-meta">修改时间: ' + d.time + '</div>' +
                        '</div>' +
                        '<div class="file-size">文件夹</div>' +
                    '</a>';
                }).join('');
            }

            if (displayFiles.length > 0) {
                html += displayFiles.map(f => {
                    const href = '/' + buildPath(currentPath, f.name);
                    const ext = f.name.split('.').pop().toLowerCase();
                    if (PREVIEWABLE.has(ext)) {
                        return '<div class="file-item">' +
                            '<div class="file-icon">' + getFileIcon(f.name, false) + '</div>' +
                            '<div class="file-info">' +
                                '<div class="file-name">' + escapeHtml(f.name) + '</div>' +
                                '<div class="file-meta">修改时间: ' + f.time + '</div>' +
                            '</div>' +
                            '<div class="file-actions">' +
                                '<span class="file-size">' + f.size + '</span>' +
                                '<a href="' + href + '?preview=1" target="_blank" class="btn-preview">预览</a>' +
                                '<a href="' + href + '" download class="btn-download">下载</a>' +
                            '</div>' +
                        '</div>';
                    }
                    return '<a href="' + href + '" class="file-item" download title="点击下载 ' + escapeHtml(f.name) + '">' +
                        '<div class="file-icon">' + getFileIcon(f.name, false) + '</div>' +
                        '<div class="file-info">' +
                            '<div class="file-name">' + escapeHtml(f.name) + '</div>' +
                            '<div class="file-meta">修改时间: ' + f.time + '</div>' +
                        '</div>' +
                        '<div class="file-size">' + f.size + '</div>' +
                    '</a>';
                }).join('');
            }

            container.innerHTML = html;
            const total = displayDirs.length + displayFiles.length;
            countEl.textContent = total + ' 个项目 (' + displayDirs.length + ' 个文件夹, ' + displayFiles.length + ' 个文件)';
        }

        document.getElementById('searchInput').addEventListener('input', function(e) {
            renderFiles(allDirs, allFiles);
        });
    </script>
`


