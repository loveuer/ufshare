# UFShare v2

轻量级制品仓库管理系统，类似 JFrog Artifactory 的开源替代方案。

## 技术栈

- **后端**: Go 1.25+ / [ursa](https://github.com/loveuer/ursa) / GORM
- **前端**: React + MUI
- **数据库**: SQLite / MySQL / PostgreSQL

## 特性

### MVP (开发中)

- **用户和权限管理**
  - 用户管理（注册、登录、Token）
  - 模块管理（用户模块、普通文件模块、npm 模块）
  - 权限控制（读/写权限，支持公有读/写设置）

- **普通文件模块**
  - 支持 curl 上传/下载
  - 适用于 CI/CD 产物存储

- **npm 模块**
  - npm publish / install
  - 私有 npm registry

### Roadmap

- [ ] go 模块
- [ ] pypi 模块
- [ ] mvn 模块

## 快速开始

```bash
# 构建
go build -o ufshare .

# 运行
./ufshare
```

## API

### 文件上传

```bash
curl -X PUT -T /path/to/file http://localhost:8000/files/your-file
```

### 文件下载

```bash
curl -O http://localhost:8000/files/your-file
```

## 许可证

MIT
