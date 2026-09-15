import { expect, test } from '@playwright/test'
import { registerViaUI, writeTempResume } from './utils'

// Happy path：注册 → 选岗 → 上传简历并解析 → 建面试 → 逐题作答 → 结束 → 评估报告
test('register to report: full mock interview happy path', async ({ page }) => {
  await registerViaUI(page)

  // 选择岗位（种子数据 job-go-senior）
  await page.goto('/jobs')
  await expect(page.getByText('开始准备 →').first()).toBeVisible()
  await page.locator('article').first().click()
  await page.waitForURL(/\/prepare\//)

  // 上传简历，等待 mock 解析完成（异步任务 + 轮询）
  const resumePath = writeTempResume('happy')
  await page.setInputFiles('input[type="file"]', resumePath)
  await expect(page.getByText(/解析完成/)).toBeVisible({ timeout: 60_000 })

  // 固定 3 题，缩短用例时长
  await page.locator('input[type="range"]').first().fill('3')

  // 进入考场
  await page.getByRole('button', { name: '进入考场' }).click()
  await page.waitForURL(/\/interview\//)

  // 逐题作答：文本框在中栏（自由对话栏也有 textarea，按 placeholder 区分）
  const answerTextarea = page.getByPlaceholder('在这里输入你的回答，尽量结合项目实例与具体数据…')
  const submitBtn = page.getByRole('button', { name: '提交回答' })
  const doneDots = page.locator('.q-dot-done')

  for (let i = 0; i < 3; i++) {
    await answerTextarea.waitFor({ state: 'visible' })
    await answerTextarea.fill(
      `第 ${i + 1} 题：我会先定位瓶颈（pprof / 指标），再用 context 超时控制、` +
        '带缓冲 channel 和 worker pool 控制并发，并补充单测与可观测性。',
    )
    await submitBtn.click()
    await expect(doneDots).toHaveCount(i + 1, { timeout: 60_000 })
    if (i < 2) {
      await page.locator('.q-dot').nth(i + 1).click()
    }
  }

  // 结束面试（确认弹窗）→ 跳转报告页
  page.once('dialog', (dialog) => void dialog.accept())
  await page.getByRole('button', { name: '结束面试' }).click()
  await page.waitForURL(/\/report\//)

  // 评估 + 报告为异步任务，等待最终报告渲染
  await expect(page.getByRole('heading', { name: '面试评估报告' })).toBeVisible({
    timeout: 90_000,
  })

  // 报告关键结构：总分卡与 Evidence
  const score = page.locator('main').filter({ hasText: '面试评估报告' })
  await expect(score.getByText(/EXAMINER'S REPORT/)).toBeVisible()
  await expect(page.getByRole('heading', { name: '评分依据（Evidence）' })).toBeVisible()
})
