import { expect, test } from '@playwright/test'
import { registerViaUI } from './utils'

// 失败态 + 重试：不存在的会话无法评估，报告页应进入错误态；
// 点击「重新生成」会重新发起评估请求（mock 环境下仍然失败，但不应白屏/卡死）
test('report generation failure shows retry and re-requests on click', async ({ page }) => {
  await registerViaUI(page)

  await page.goto('/report/00000000-0000-0000-0000-000000000000')

  await expect(page.getByRole('heading', { name: '评卷遇到问题' })).toBeVisible({
    timeout: 30_000,
  })
  const retryBtn = page.getByRole('button', { name: '重新生成' })
  await expect(retryBtn).toBeVisible()

  // 点击重试时前端必须重新 POST 评估任务
  const evalRequest = page.waitForRequest((req) =>
    req.method() === 'POST' && req.url().includes('/evaluations/sessions/'),
  )
  await retryBtn.click()
  await evalRequest

  // 回到错误态（不存在的会话重试无副作用），重试按钮仍在，可再次操作
  await expect(page.getByRole('heading', { name: '评卷遇到问题' })).toBeVisible({
    timeout: 30_000,
  })
  await expect(retryBtn).toBeVisible()
})
