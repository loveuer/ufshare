import { defineConfig } from '@playwright/test'

export const E2E_PORT = parseInt(process.env.E2E_PORT ?? '18082')
export const E2E_JWT_SECRET = 'e2e-playwright-test-jwt-secret-xyz'
export const E2E_ADMIN_USER = 'admin'
export const E2E_ADMIN_PASS = 'admin123'

export default defineConfig({
  testDir: '.',
  globalSetup: './global-setup.ts',
  globalTeardown: './global-teardown.ts',
  reporter: [
    ['html', { outputFolder: 'playwright-report', open: 'never' }],
    ['list'],
  ],
  use: {
    baseURL: `http://127.0.0.1:${E2E_PORT}`,
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    locale: 'en-US',
  },
  timeout: 30000,
  retries: 1,
})
