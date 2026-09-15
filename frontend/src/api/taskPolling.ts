import { taskApi } from './endpoints'
import type { TaskAccepted, TaskInfo } from './types'

// TaskFailedError 任务达到 failed 终态（含后端 last_error）
export class TaskFailedError extends Error {
  task: TaskInfo
  constructor(task: TaskInfo) {
    super(task.last_error || '任务执行失败')
    this.name = 'TaskFailedError'
    this.task = task
  }
}

interface WaitOptions {
  intervalMs?: number // 轮询间隔，默认 1.2s
  timeoutMs?: number // 总超时，默认 5 分钟（真实 LLM 评估可能较慢）
  signal?: AbortSignal
}

// waitForTask 轮询任务直到 succeeded/failed/超时。
// POST 异步接口拿到 202 TaskAccepted 后调用；失败时抛 TaskFailedError（可读取 last_error）。
export async function waitForTask(accepted: TaskAccepted, opts: WaitOptions = {}): Promise<TaskInfo> {
  const interval = opts.intervalMs ?? 1200
  const deadline = Date.now() + (opts.timeoutMs ?? 5 * 60 * 1000)
  let latest: TaskInfo | null = null

  while (Date.now() < deadline) {
    if (opts.signal?.aborted) {
      throw new DOMException('aborted', 'AbortError')
    }
    latest = await taskApi.get(accepted.task_id)
    if (latest.status === 'succeeded') return latest
    if (latest.status === 'failed') throw new TaskFailedError(latest)
    await new Promise((resolve) => setTimeout(resolve, interval))
  }
  throw new Error(latest ? `任务超时（当前状态：${latest.status}）` : '任务轮询超时')
}
