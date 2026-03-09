import { test, expect, Page } from '@playwright/test'
import { E2E_ADMIN_USER, E2E_ADMIN_PASS, E2E_PORT } from './playwright.config'

const REGISTRY_URL = `http://127.0.0.1:${E2E_PORT}/npm`

async function login(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Username').fill(E2E_ADMIN_USER)
  await page.getByLabel('Password').fill(E2E_ADMIN_PASS)
  await page.getByRole('button', { name: 'Sign In' }).click()
  await page.waitForURL(/\/(files|npm|users|settings)/)
}

test.describe('npm page', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.getByRole('link', { name: 'npm' }).click()
    await page.waitForURL(/\/npm/)
  })

  test('shows npm registry URL', async ({ page }) => {
    // registry URL 精确显示在页面顶部（install 命令行也包含 URL，需用 exact）
    await expect(page.getByText(REGISTRY_URL, { exact: true }).first()).toBeVisible()
  })

  test('shows package count chip', async ({ page }) => {
    // 等待加载完毕（loading spinner 消失）
    await expect(page.getByRole('progressbar')).toHaveCount(0)
    // 页面顶部显示包数量 chip
    await expect(page.getByText(/^\d+ packages$/)).toBeVisible()
  })

  test('lists seeded packages (lodash, axios)', async ({ page }) => {
    await expect(page.getByRole('progressbar')).toHaveCount(0)

    // globalSetup 预热了 lodash 和 axios；包名显示在 monospace Typography 中
    // 用 getByText 定位包名文字节点（exact 匹配纯文本节点）
    await expect(page.getByText('lodash', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('axios', { exact: true }).first()).toBeVisible()
  })

  test('shows latest dist-tag chip for each package', async ({ page }) => {
    await expect(page.getByRole('progressbar')).toHaveCount(0)

    // 每个包行应有 "latest: x.y.z" chip
    const chips = page.getByText(/^latest:/)
    await expect(chips.first()).toBeVisible()
  })

  test('expands lodash row and shows version list', async ({ page }) => {
    await expect(page.getByRole('progressbar')).toHaveCount(0)

    // 找到 lodash 所在行并点击展开
    const lodashRow = page.getByRole('row').filter({ hasText: 'lodash' }).first()
    await lodashRow.click()

    // 展开后出现版本子表头（<th> = columnheader；exact 区分 "Version" vs 主表的 "Versions"）
    await expect(page.getByRole('columnheader', { name: 'Version', exact: true }).first()).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Status', exact: true }).first()).toBeVisible()

    // 至少有一条版本行（Cached 或 Proxy only chip）
    const statusChip = page.getByText(/Cached|Proxy only/).first()
    await expect(statusChip).toBeVisible()
  })

  test('shows install command for each package', async ({ page }) => {
    await expect(page.getByRole('progressbar')).toHaveCount(0)

    // 每个包行都有 "npm install <name> --registry ..." 文字
    await expect(page.getByText(/npm install lodash --registry/)).toBeVisible()
    await expect(page.getByText(/npm install axios --registry/)).toBeVisible()
  })

  test('copy registry button copies correct npm set command', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write'])
    await expect(page.getByRole('progressbar')).toHaveCount(0)

    // registry URL 旁边的 IconButton：找到显示 URL 的文字节点，再定位其父容器内的 button
    const copyBtn = page.getByText(REGISTRY_URL, { exact: true }).locator('..').locator('button')
    await copyBtn.click()

    const clipboard = await page.evaluate(() => navigator.clipboard.readText())
    expect(clipboard).toBe(`npm set registry ${REGISTRY_URL}`)
  })

  test('collapse expanded package row hides version list', async ({ page }) => {
    await expect(page.getByRole('progressbar')).toHaveCount(0)

    const lodashRow = page.getByRole('row').filter({ hasText: 'lodash' }).first()

    // 展开
    await lodashRow.click()
    await expect(page.getByRole('columnheader', { name: 'Version', exact: true }).first()).toBeVisible()

    // 再次点击折叠；unmountOnExit 会在动画结束后移除子表，exact 避免匹配主表 "Versions"
    await lodashRow.click()
    await expect(page.getByRole('columnheader', { name: 'Version', exact: true })).toHaveCount(0)
  })
})
