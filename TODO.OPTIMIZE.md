# 代码优化待办清单

> 生成日期：2026-03-09
> 来源：代码审查报告

---

## 优先级说明

- **P0** - 高优先级：影响正确性、安全性或数据一致性，应尽快修复
- **P1** - 中优先级：影响代码质量或可维护性，建议近期修复
- **P2** - 低优先级：代码风格或重构优化，可择机进行

---

## P0 - 高优先级

### 1. 事务保护缺失

**问题：** `npm/service.go:Publish` 多步操作没有事务保护，可能导致脏数据。

**位置：** `internal/service/npm/publish.go:17-89`

**建议：**
```go
func (s *Service) Publish(body *PublishBody, ...) (err error) {
    tx := s.db.Begin()
    defer func() {
        if err != nil {
            tx.Rollback()
        }
    }()

    // 所有 DB 操作使用 tx
    if err := tx.Create(&pkg).Error; err != nil {
        return err
    }
    // ...

    return tx.Commit().Error
}
```

---

### 2. 并发安全问题

**问题：**
- `setting.go:Set` 中回调使用 `go fn(value)` 异步执行，无错误处理
- `Dedicated.Start` 的 `started` 标志检查存在竞态条件

**位置：**
- `internal/service/setting.go:77-80`
- `internal/pkg/server/dedicated.go:40-68`

**建议：**
```go
// 添加 panic 恢复和错误日志
for _, fn := range fns {
    go func(cb func(string)) {
        defer func() {
            if r := recover(); r != nil {
                log.Printf("callback panic: %v", r)
            }
        }()
        cb(value)
    }(fn)
}
```

---

### 3. 文件上传并发控制

**问题：** 同一文件并发上传时会相互覆盖，缺少防重机制。

**位置：** `internal/service/file.go:35-94`

**建议：** 使用 `singleflight.Group` 防止并发重复上传：
```go
var sf singleflight.Group

func (s *FileService) Upload(filePath string, ...) (*model.FileEntry, error) {
    v, err, _ := sf.Do(filePath, func() (interface{}, error) {
        // 上传逻辑
    })
    return v.(*model.FileEntry), err
}
```

---

## P1 - 中优先级

### 4. 配置管理混乱

**问题：**
- `Config` 默认值和验证逻辑分散在 `main.go` 和 `config.go`
- 数据库 DSN 默认逻辑在 `main.go:72-74`
- 生产环境 JWT 默认值不安全

**位置：** `internal/pkg/config/config.go`, `cmd/ufshare/main.go:72-74`

**建议：** 统一在 `Load()` 中处理所有默认值和验证：
```go
func Load() *Config {
    cfg := &Config{
        Address:  getEnv("UFSHARE_ADDRESS", "0.0.0.0:8000"),
        Data:     getEnv("UFSHARE_DATA", "./data"),
        Database: DatabaseConfig{
            Driver: getEnv("DB_DRIVER", "sqlite"),
            DSN:    getEnv("DB_DSN", ""),
        },
        JWT: JWTConfig{
            Secret: getEnv("JWT_SECRET", ""),
            Expire: 24 * time.Hour,
        },
    }
    if cfg.Database.Driver == "sqlite" && cfg.Database.DSN == "" {
        cfg.Database.DSN = filepath.Join(cfg.Data, "ufshare.db")
    }
    return cfg
}
```

---

### 5. main.go 职责过重

**问题：** `run()` 函数 190 行，包含数据库连接、服务初始化、服务器启动、回调注册等多种职责。

**位置：** `cmd/ufshare/main.go:57-191`

**建议：** 抽取为 `App` 结构体：
```go
type App struct {
    config  *config.Config
    db      *gorm.DB
    services *Services
    servers  *Servers
}

type Services struct {
    Auth    *service.AuthService
    User    *service.UserService
    File    *service.FileService
    Setting *service.SettingService
    Npm     *npmsvc.Service
}
```

---

### 6. 错误定义分散

**问题：** 各 service 包各自定义错误，没有统一管理。

**位置：**
- `internal/service/auth.go:14-22`
- `internal/service/npm/service.go:20-25`

**建议：** 创建 `internal/errors` 包统一管理：
```go
package errors

var (
    // User errors
    ErrUserNotFound    = errors.New("user not found")
    ErrUserExists      = errors.New("user already exists")

    // Package errors
    ErrPackageNotFound = errors.New("package not found")
    ErrVersionNotFound = errors.New("version not found")
)
```

---

### 7. 请求验证不统一

**问题：** 各 handler 手动验证必填字段，没有统一机制。

**位置：**
- `internal/api/handler/user.go:37-42`
- `internal/api/handler/auth.go:39-44`

**建议：** 引入验证接口：
```go
type Validator interface {
    Validate() error
}

func validate(req interface{}) error {
    if v, ok := req.(Validator); ok {
        return v.Validate()
    }
    return nil
}
```

---

### 8. 调试日志未清理

**问题：** 生产代码包含 DEBUG 日志。

**位置：** `internal/service/setting.go:38-50, 64, 77`

```go
log.Printf("[SETTING] OnChange registered for key %s", key) // DEBUG
log.Printf("[SETTING] Current listeners: %v", ...)          // DEBUG
log.Printf("[SETTING] Set %s = %q", key, value)             // DEBUG
log.Printf("[SETTING] Notifying %d listeners", len(fns))    // DEBUG
```

**建议：** 移除或使用可配置的日志级别。

---

## P2 - 低优先级

### 9. npm/service.go 过于臃肿

**问题：** 单文件 240 行，包含多种职责（包管理、tarball 存储、代理缓存、元数据转换）。

**位置：** `internal/service/npm/service.go`

**建议：** 按职责拆分：
```
internal/service/npm/
├── service.go      # 主体 Service 结构
├── publish.go      # 发布逻辑
├── proxy.go        # 代理缓存逻辑
├── fetch.go        # 获取逻辑
├── storage.go      # tarball 存储逻辑
├── metadata.go     # 元数据转换逻辑
└── types.go        # 类型定义
```

---

### 10. 软删除处理不当

**问题：** 模型定义了 `DeletedAt` 但查询未正确处理软删除。

**位置：** `internal/model/*.go`

**建议：** 所有查询明确排除已删除记录：
```go
db.Where("name = ? AND deleted_at IS NULL", name).First(&pkg)
```

---

### 11. 错误忽略未处理

**问题：** 代码中存在 `//nolint:errcheck` 或 `err` 未处理。

**位置：**
- `internal/api/router.go:106` - `//nolint:errcheck`
- `internal/service/npm/service.go:179-182` - `abbreviateVersionMeta` JSON 错误静默忽略

**建议：** 明确处理或记录日志。

---

### 12. 中间件返回值语义模糊

**问题：** `resolveAuth` 返回 `(ok bool, err error)` 语义不清晰。

**位置：** `internal/api/middleware/auth.go:20-59`

**建议：** 使用枚举状态：
```go
type AuthResult int

const (
    AuthNone AuthResult = iota
    AuthSuccess
    AuthFailure
)
```

---

### 13. 不必要的 sleep

**问题：** `main.go:81` 使用 `time.Sleep(500ms)` 等待 goroutine。

**位置：** `cmd/ufshare/main.go:81`

**建议：** 使用 channel 或 `sync.WaitGroup` 等同步原语。

---

## 检查清单

### 代码质量
- [ ] P0-1: 事务保护缺失
- [ ] P0-2: 并发安全问题
- [ ] P0-3: 文件上传并发控制
- [ ] P1-4: 配置管理混乱
- [ ] P1-5: main.go 职责过重
- [ ] P1-6: 错误定义分散
- [ ] P1-7: 请求验证不统一
- [ ] P1-8: 调试日志未清理
- [ ] P2-9: npm/service.go 过于臃肿
- [ ] P2-10: 软删除处理不当
- [ ] P2-11: 错误忽略未处理
- [ ] P2-12: 中间件返回值语义模糊
- [ ] P2-13: 不必要的 sleep

---

## 附录：相关文件列表

| 文件 | 问题数 | 优先级 |
|------|--------|--------|
| `internal/service/npm/publish.go` | 1 | P0 |
| `internal/service/setting.go` | 2 | P0, P1 |
| `internal/pkg/server/dedicated.go` | 1 | P0 |
| `internal/service/file.go` | 1 | P0 |
| `internal/pkg/config/config.go` | 1 | P1 |
| `cmd/ufshare/main.go` | 3 | P1, P2 |
| `internal/service/npm/service.go` | 2 | P1, P2 |
| `internal/api/handler/user.go` | 1 | P1 |
| `internal/api/handler/auth.go` | 1 | P1 |
| `internal/api/middleware/auth.go` | 1 | P2 |
| `internal/model/*.go` | 1 | P2 |
