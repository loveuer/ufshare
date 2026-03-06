package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// FS 返回 dist 目录的子文件系统，用于 http.FileServer
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
