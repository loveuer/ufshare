# UFShare npm 测试报告

**生成时间:** 2026-03-08 19:39:38

---

## 构建

- `go build -o ufshare ./cmd/ufshare` **成功**

## Go 集成测试

| 状态 | 数量 |
|------|------|
| ✅ Pass | 8 |
| ❌ Fail | 0 |
| ⏭ Skip | 0 |

### 详细结果

| 测试名称 | 状态 | 耗时 |
|----------|------|------|
| `TestNpm_GetPackument_Cache` | ✅ pass | 0.23s |
| `TestNpm_GetPackument_Proxy` | ✅ pass | 0.22s |
| `TestNpm_GetTarball_Proxy` | ✅ pass | 0.58s |
| `TestNpm_Install_PopularFrontend` | ✅ pass | 23.58s |
| `TestNpm_ListPackages_API` | ✅ pass | 0.18s |
| `TestNpm_PackageNotFound` | ✅ pass | 0.21s |
| `TestNpm_Ping` | ✅ pass | 0.09s |
| `TestNpm_TarballURL_Rewritten` | ✅ pass | 0.18s |

## Playwright Web UI 测试

