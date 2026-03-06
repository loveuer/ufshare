# Agent Instructions

## 项目概述

UFShare v2 是一个轻量级制品仓库管理系统，目标是成为 JFrog Artifactory 的开源替代方案。

## 技术栈

- 语言: Go 1.25+
- Web 框架: [ursa](https://github.com/loveuer/ursa)
- ORM: GORM (支持 SQLite / MySQL / PostgreSQL)
- 前端: React + MUI

## 构建与运行

```bash
# 构建
go build -o ufshare ./cmd/ufshare

# 运行
./ufshare

# 测试
go test ./...

# 代码检查
go vet ./...
golangci-lint run
```

## 项目结构

```
.
├── cmd/ufshare/        # 程序入口
├── internal/
│   ├── api/            # HTTP API 路由和处理器
│   │   ├── handler/    # 请求处理器
│   │   └── middleware/ # 中间件
│   ├── model/          # 数据模型 (GORM)
│   ├── service/        # 业务逻辑层
│   └── pkg/            # 内部工具包
├── pkg/                # 公共包
└── web/                # 前端 (React + MUI)
```

## 代码风格

- 遵循 Go 官方代码规范
- 使用 gofmt 格式化代码
- 错误处理使用 error wrapping
- 注释使用中文或英文均可

## MVP 开发计划

### 第一步: 用户和权限管理架构

- 用户模型 (User)
- 模块模型 (Module): 定义模块类型和权限设置
- 权限模型 (Permission): 用户对模块的读/写权限
- JWT 认证
- API: 注册、登录、用户管理、权限管理

### 第二步: 普通文件模块

- 文件上传 (PUT)
- 文件下载 (GET)
- 文件列表
- 权限检查集成

### 第三步: npm 模块

- npm publish
- npm install
- package.json 解析
- 版本管理

## TODO

- [ ] go 模块
- [ ] pypi 模块
- [ ] mvn 模块
