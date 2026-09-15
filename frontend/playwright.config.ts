import { defineConfig, devices } from '@playwright/test'

// E2E 假设后端（mock provider + PostgreSQL + MinIO）已在 :8080 运行，
// 前端经 Vite 代理同源访问；未显式指定 E2E_BASE_URL 时自动拉起/复用 dev server。
// CI 中由 workflow 负责启动后端，再执行 npx playwright test。
export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  timeout: 120_000,
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: process.env.E2E_BASE_URL
    ? undefined
    : {
        command: 'npm run dev',
        port: 5173,
        reuseExistingServer: true,
        timeout: 60_000,
      },
})
