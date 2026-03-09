#!/usr/bin/env bash
# tests/run_tests.sh  —  运行全套 npm 测试并生成报告
# 用法: bash tests/run_tests.sh [--short]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REPORT_DIR="$ROOT/tests/report"
SHORT_FLAG=""

for arg in "$@"; do
  case "$arg" in
    --short) SHORT_FLAG="-short" ;;
  esac
done

# ─────────────────────────────────────────────────────────
# 颜色
GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
ok()   { echo -e "${GREEN}✓${NC} $*"; }
fail() { echo -e "${RED}✗${NC} $*"; }
info() { echo -e "${YELLOW}▶${NC} $*"; }

# ─────────────────────────────────────────────────────────
mkdir -p "$REPORT_DIR"

REPORT_MD="$REPORT_DIR/npm-test-report.md"
GO_JSON="$REPORT_DIR/go-results.json"
START_TIME=$(date +%s)

cat > "$REPORT_MD" <<EOF
# UFShare npm 测试报告

**生成时间:** $(date '+%Y-%m-%d %H:%M:%S')

---

EOF

# ─────────────────────────────────────────────────────────
# 1. 构建 Go 二进制
# ─────────────────────────────────────────────────────────
info "Step 1/4: 构建 ufshare 二进制..."
cd "$ROOT"
if go build -o ufshare ./cmd/ufshare; then
  ok "构建成功: $ROOT/ufshare"
  echo "## 构建" >> "$REPORT_MD"
  echo "" >> "$REPORT_MD"
  echo "- \`go build -o ufshare ./cmd/ufshare\` **成功**" >> "$REPORT_MD"
  echo "" >> "$REPORT_MD"
else
  fail "构建失败"
  echo "## 构建" >> "$REPORT_MD"
  echo "**FAILED** — 构建失败，后续测试跳过" >> "$REPORT_MD"
  exit 1
fi

# ─────────────────────────────────────────────────────────
# 2. Go 集成测试（API + npm install）
# ─────────────────────────────────────────────────────────
info "Step 2/4: 运行 Go 集成测试..."
echo "## Go 集成测试" >> "$REPORT_MD"
echo "" >> "$REPORT_MD"

GO_PASS=0; GO_FAIL=0; GO_SKIP=0

set +e
go test -v -timeout 300s $SHORT_FLAG \
  -json ./tests/ 2>&1 | tee "$GO_JSON"
GO_EXIT=$?
set -e

# 解析 JSON 结果
if command -v python3 &>/dev/null; then
  python3 - <<'PYEOF' >> "$REPORT_MD"
import json, sys

results = []
with open(sys.argv[1] if len(sys.argv) > 1 else 'tests/report/go-results.json') as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        try:
            obj = json.loads(line)
            if obj.get('Action') in ('pass', 'fail', 'skip') and obj.get('Test'):
                results.append(obj)
        except:
            pass

passed = [r for r in results if r['Action'] == 'pass']
failed = [r for r in results if r['Action'] == 'fail']
skipped = [r for r in results if r['Action'] == 'skip']

print(f"| 状态 | 数量 |")
print(f"|------|------|")
print(f"| ✅ Pass | {len(passed)} |")
print(f"| ❌ Fail | {len(failed)} |")
print(f"| ⏭ Skip | {len(skipped)} |")
print()

if failed:
    print("### 失败用例")
    print()
    for r in failed:
        elapsed = r.get('Elapsed', 0)
        print(f"- ❌ `{r['Test']}` ({elapsed:.2f}s)")
    print()

print("### 详细结果")
print()
print("| 测试名称 | 状态 | 耗时 |")
print("|----------|------|------|")
all_tests = sorted(results, key=lambda r: r.get('Test', ''))
for r in all_tests:
    icon = {'pass': '✅', 'fail': '❌', 'skip': '⏭'}.get(r['Action'], '?')
    elapsed = r.get('Elapsed', 0)
    print(f"| `{r['Test']}` | {icon} {r['Action']} | {elapsed:.2f}s |")
PYEOF
fi

echo "" >> "$REPORT_MD"

if [ $GO_EXIT -eq 0 ]; then
  ok "Go 集成测试全部通过"
else
  fail "Go 集成测试有失败项 (exit $GO_EXIT)"
fi

# ─────────────────────────────────────────────────────────
# 3. Playwright E2E 测试
# ─────────────────────────────────────────────────────────
info "Step 3/4: 运行 Playwright E2E 测试..."
echo "## Playwright Web UI 测试" >> "$REPORT_MD"
echo "" >> "$REPORT_MD"

PW_DIR="$ROOT/tests/e2e"
PW_EXIT=0

# 安装依赖
if [ ! -d "$PW_DIR/node_modules" ]; then
  info "安装 Playwright 依赖..."
  cd "$PW_DIR" && npm install --silent
fi

# 安装 Chromium（如果缺失）
info "确认 Playwright 浏览器..."
cd "$PW_DIR" && npx playwright install chromium --with-deps 2>/dev/null || true

set +e
cd "$PW_DIR" && npx playwright test \
  --reporter=list \
  2>&1 | tee "$REPORT_DIR/playwright-stdout.txt"
PW_EXIT=$?
set -e

# 解析 Playwright 输出摘要
if grep -qE "passed|failed|skipped" "$REPORT_DIR/playwright-stdout.txt" 2>/dev/null; then
  SUMMARY=$(grep -E "\d+ (passed|failed|skipped)" "$REPORT_DIR/playwright-stdout.txt" | tail -1)
  echo "**结果:** $SUMMARY" >> "$REPORT_MD"
  echo "" >> "$REPORT_MD"
fi

# 列出测试用例结果
echo "### 测试用例" >> "$REPORT_MD"
echo "" >> "$REPORT_MD"
grep -E "^\s+(✓|×|−)\s+" "$REPORT_DIR/playwright-stdout.txt" 2>/dev/null \
  | sed 's/✓/✅/g; s/×/❌/g; s/−/⏭/g' \
  | sed 's/^/- /' >> "$REPORT_MD" || true
echo "" >> "$REPORT_MD"

if [ -d "$PW_DIR/playwright-report" ]; then
  # 复制到统一报告目录
  cp -r "$PW_DIR/playwright-report" "$REPORT_DIR/playwright-html"
  echo "HTML 报告: \`tests/report/playwright-html/index.html\`" >> "$REPORT_MD"
  echo "" >> "$REPORT_MD"
fi

if [ $PW_EXIT -eq 0 ]; then
  ok "Playwright 测试全部通过"
else
  fail "Playwright 测试有失败项 (exit $PW_EXIT)"
fi

# ─────────────────────────────────────────────────────────
# 4. 清理临时文件
# ─────────────────────────────────────────────────────────
info "Step 4/4: 清理临时文件..."
echo "## 清理" >> "$REPORT_MD"
echo "" >> "$REPORT_MD"

# Playwright 会在 globalTeardown 里删掉 ufshare data dir 和 state file
# 清理 e2e 目录下的 Playwright 内部临时文件
rm -rf "$PW_DIR/test-results" 2>/dev/null && echo "- 删除 e2e/test-results/" >> "$REPORT_MD" || true
ok "临时文件已清理"
echo "" >> "$REPORT_MD"

# ─────────────────────────────────────────────────────────
# 最终汇总
# ─────────────────────────────────────────────────────────
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

FINAL_STATUS="PASSED"
[ $GO_EXIT -ne 0 ] && FINAL_STATUS="FAILED"
[ $PW_EXIT -ne 0 ] && FINAL_STATUS="FAILED"

cat >> "$REPORT_MD" <<EOF
---

## 总结

| 项目 | 结果 |
|------|------|
| Go 集成测试 | $([ $GO_EXIT -eq 0 ] && echo '✅ PASSED' || echo '❌ FAILED') |
| Playwright E2E | $([ $PW_EXIT -eq 0 ] && echo '✅ PASSED' || echo '❌ FAILED') |
| 总耗时 | ${ELAPSED}s |

**最终结果: $FINAL_STATUS**
EOF

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ "$FINAL_STATUS" = "PASSED" ]; then
  ok "全部测试通过 (${ELAPSED}s)"
else
  fail "测试存在失败项 (${ELAPSED}s)"
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "报告文件:"
echo "  Markdown : tests/report/npm-test-report.md"
echo "  Go JSON  : tests/report/go-results.json"
[ -d "$REPORT_DIR/playwright-html" ] && echo "  HTML     : tests/report/playwright-html/index.html"
echo ""

exit $([ "$FINAL_STATUS" = "PASSED" ] && echo 0 || echo 1)
