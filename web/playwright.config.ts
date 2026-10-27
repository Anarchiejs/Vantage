import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig, devices } from '@playwright/test'

const web = path.dirname(fileURLToPath(import.meta.url))
const racine = path.resolve(web, '..')

export default defineConfig({
  testDir: path.join(web, 'e2e'),
  testMatch: '*.spec.ts',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: 'http://127.0.0.1:7879',
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'node web/e2e/preparer.mjs',
    cwd: racine,
    url: 'http://127.0.0.1:7879',
    timeout: 600_000,
    reuseExistingServer: !process.env.CI,
  },
})
