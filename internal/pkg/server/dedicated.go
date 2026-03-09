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
	name  string
	setup SetupFunc

	mu     sync.Mutex
	srv    *http.Server
	cancel context.CancelFunc
	started bool  // 是否已启动过
}

// New 创建一个 Dedicated，name 仅用于日志标识，setup 负责注册路由
func New(name string, setup SetupFunc) *Dedicated {
	return &Dedicated{name: name, setup: setup}
}

// Start 在 addr 上启动独立服务器（非阻塞）。
// 若已有服务器在运行，先停止旧的再启动新的。
// 注意：每个 Dedicated 实例只能成功启动一次路由注册，后续调用 Start 会直接返回。
func (d *Dedicated) Start(addr string) {
	log.Printf("[DEDICATED] %s.Start(%q) called, started=%v", d.name, addr, d.started) // DEBUG
	if addr == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	// 防止重复注册路由（ursa 不允许多次注册相同路由）
	if d.started {
		log.Printf("[%s] dedicated server already started, ignoring Start(%s)", d.name, addr)
		return
	}

	d.stopLocked()

	app := ursa.New(ursa.Config{BodyLimit: -1})
	d.setup(app)

	// ursa.App.Run 是阻塞的；用 net/http.Server 包装以支持优雅关闭
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[%s] dedicated server: listen %s failed: %v", d.name, addr, err)
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

// Stop 优雅关闭当前独立服务器（最多等待 5 秒）
func (d *Dedicated) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopLocked()
}

// Restart 先停止旧服务器，再在新地址上启动（线程安全）
func (d *Dedicated) Restart(addr string) {
	d.mu.Lock()
	d.stopLocked()
	d.mu.Unlock()
	d.Start(addr)
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
}
