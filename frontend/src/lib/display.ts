// 后端枚举到中文展示的映射

export const STATUS_LABEL: Record<string, string> = {
  INIT: '待开始',
  READY: '就绪',
  RUNNING: '进行中',
  PAUSED: '已暂停',
  FINISHING: '交卷中',
  EVALUATING: '评分中',
  COMPLETED: '已完成',
  FAILED: '异常',
}

// 状态对应 tag 配色（clay/jade/accent 之外默认中性）
export function statusTagClass(status: string): string {
  if (status === 'RUNNING') return 'tag tag-jade'
  if (status === 'COMPLETED') return 'tag tag-accent'
  if (status === 'FAILED') return 'tag tag-clay'
  return 'tag'
}

export const TYPE_LABEL: Record<string, string> = {
  technical: '技术面',
  behavioral: '行为面',
  mixed: '综合面',
}

export const MODE_LABEL: Record<string, string> = {
  text: '文字',
  voice: '语音',
}

export const QTYPE_LABEL: Record<string, string> = {
  technical: '技术题',
  behavioral: '行为题',
  project: '项目题',
  follow_up: '追问',
}

export function formatTime(iso?: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}
