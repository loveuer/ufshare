import { test, expect } from '@playwright/test'

test.describe('Maven Repository', () => {
  test.beforeEach(async ({ page }) => {
    // Login first
    await page.goto('http://localhost:9817/login')
    await page.fill('input[type="text"]', 'admin')
    await page.fill('input[type="password"]', 'admin123')
    await page.click('button[type="submit"]')
    await page.waitForURL('http://localhost:9817/files')
  })

  test('should navigate to Maven page', async ({ page }) => {
    await page.click('text=Maven')
    await page.waitForURL('http://localhost:9817/maven')
    await expect(page.locator('h4')).toContainText('Maven Repository')
  })

  test('should display artifacts list', async ({ page }) => {
    await page.click('text=Maven')
    await page.waitForURL('http://localhost:9817/maven')
    // Should show search fields and table
    await expect(page.locator('input[placeholder="Search artifacts..."]')).toBeVisible()
    await expect(page.locator('button:has-text("Search")')).toBeVisible()
    await expect(page.locator('table')).toBeVisible()
  })

  test('should search for artifacts', async ({ page }) => {
    await page.click('text=Maven')
    await page.waitForURL('http://localhost:9817/maven')
    await page.fill('input[placeholder="Search artifacts..."]', 'commons-lang3')
    await page.click('button:has-text("Search")')
    // Wait for search results or no results message
    await page.waitForTimeout(1000)
    // Should show table or "No artifacts found" message
    const hasTable = await page.locator('table').isVisible().catch(() => false)
    const hasNoResults = await page.locator('text=No artifacts found').isVisible().catch(() => false)
    expect(hasTable || hasNoResults).toBeTruthy()
  })

  test('should display Maven settings in settings page', async ({ page }) => {
    await page.click('text=Settings')
    await page.waitForURL('http://localhost:9817/settings')
    // Click on Maven tab (icon button)
    await page.locator('button[role="tab"]:has-text("Maven")').click()
    // Check Maven settings are displayed
    await expect(page.locator('text=Maven Module')).toBeVisible()
    await expect(page.locator('label:has-text("Upstream Repository")')).toBeVisible()
    await expect(page.locator('input[placeholder="https://repo.maven.apache.org/maven2"]')).toBeVisible()
    // Enable dedicated port to see the address field
    await page.click('text=Enable dedicated port')
    await expect(page.locator('label:has-text("Dedicated Listen Address")')).toBeVisible()
    await expect(page.locator('input[placeholder="0.0.0.0:8082"]')).toBeVisible()
  })
})
