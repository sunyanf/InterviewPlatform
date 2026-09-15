import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import type { Page } from '@playwright/test'

// 通过 UI 注册一个全新账号（每个 worker/用例独立账号，避免数据互相污染）
export async function registerViaUI(page: Page): Promise<{ email: string; password: string }> {
  const stamp = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
  const email = `e2e-${stamp}@test.com`
  const password = 'Test1234'

  await page.goto('/register')
  await page.getByPlaceholder('考官将这样称呼你').fill('E2E 考生')
  await page.getByPlaceholder('you@example.com').fill(email)
  await page.getByPlaceholder('至少 6 位').fill(password)
  await page.getByRole('button', { name: '开始第一场模拟' }).click()
  await page.waitForURL('/')
  return { email, password }
}

// 生成一份临时简历文件（mock 解析不依赖真实内容，技能仅用于页面提示断言）
export function writeTempResume(tag: string): string {
  const path = join(tmpdir(), `e2e-resume-${tag}.txt`)
  writeFileSync(
    path,
    [
      'E2E Candidate Resume',
      '',
      'Skills: Go, goroutine, channel, context, pprof, PostgreSQL, Redis',
      '',
      'Experience: backend engineer on an AI interview platform,',
      'async task queues, RAG retrieval pipelines and LLM provider abstraction.',
    ].join('\n'),
    'utf8',
  )
  return path
}
