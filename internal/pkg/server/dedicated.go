// Package server 提供独立端口监听器的生命周期管理。
// 每个仓库模块（npm、file-store 等）可拥有一个独立的 HTTP 监听器，
// 该监听器与主端口并行运行，路由从根路径开始（无前缀）。
package server

import (
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/loveuer/ursa"
)

// SetupFunc 是注册路由的回调函数，接收一个全新的 ursa.App
type SetupFunc func(app *ursa.App)

// Dedicated 管理一个独立 HTTP 服务器的生命周期（启动/停止/重启）
type Dedicated struct {
	name      string
	setup     SetupFunc
	bodyLimit int64
	mu        sync.Mutex
	srv       *http.Server
	cancel    context.CancelFunc
	started   bool
}

// New 创建一个 Dedicated，name 仅用于日志标识，bodyLimit 透传给 ursa（-1 不限制），setup 负责注册路由
func New(name string, bodyLimit int64, setup SetupFunc) *Dedicated {
	return &Dedicated{name: name, bodyLimit: bodyLimit, setup: setup}
}

// Start 在 addr 上启动独立服务器（非阻塞）。
// 若服务器已在运行则直接返回（地址变更请使用 Restart）。
func (d *Dedicated) Start(addr string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.startLocked(addr)
}

// Stop 优雅关闭当前独立服务器（最多等待 5 秒）
func (d *Dedicated) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopLocked()
}

// Restart 先停止旧服务器，再在新地址上启动（线程安全，持有锁完成全过程）
func (d *Dedicated) Restart(addr string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopLocked()
	d.startLocked(addr)
}

// startLocked 假设调用方已持有 d.mu
func (d *Dedicated) startLocked(addr string) {
	if addr == "" || d.started {
		return
	}

	app := ursa.New(ursa.Config{BodyLimit: d.bodyLimit})
	d.setup(app)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[%s] listen %s failed: %v", d.name, addr, err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.srv = &http.Server{Handler: app}
	d.started = true

	go func() {
		log.Printf("[%s] dedicated server listening on %s", d.name, addr)
		if err := d.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[%s] dedicated server error: %v", d.name, err)
		}
		_ = ctx
	}()
}

// stopLocked 假设调用方已持有 d.mu
func (d *Dedicated) stopLocked() {
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	if d.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := d.srv.Shutdown(ctx); err != nil {
			log.Printf("[%s] dedicated server shutdown: %v", d.name, err)
		}
		d.srv = nil
		log.Printf("[%s] dedicated server stopped", d.name)
	}
	d.started = false // 重置，允许下次 Start/Restart 重新注册路由
}
