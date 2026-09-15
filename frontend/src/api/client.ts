import type { ApiEnvelope } from './types'

// token 存取：localStorage 键集中管理
const TOKEN_KEY = 'aip_token'

export const tokenStore = {
  get: () => localStorage.getItem(TOKEN_KEY),
  set: (t: string) => localStorage.setItem(TOKEN_KEY, t),
  clear: () => localStorage.removeItem(TOKEN_KEY),
}

export class ApiError extends Error {
  code: string
  status: number
  constructor(code: string, message: string, status: number) {
    super(message)
    this.code = code
    this.status = status
  }
}

interface RequestOptions {
  method?: string
  body?: unknown
  // 返回原始响应（如下载二进制）；默认解包 envelope.data
  raw?: boolean
  signal?: AbortSignal
}

// request 统一请求：注入 Bearer、解包 {code,message,data}、归一化错误
export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {}
  const token = tokenStore.get()
  if (token) headers.Authorization = `Bearer ${token}`

  let body: BodyInit | undefined
  if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }

  const resp = await fetch(`/api/v1${path}`, {
    method: opts.method ?? 'GET',
    headers,
    body,
    signal: opts.signal,
  })

  if (opts.raw) {
    if (!resp.ok) throw new ApiError('HTTP_ERROR', `请求失败 ${resp.status}`, resp.status)
    return resp as unknown as T
  }

  const payload = (await resp.json().catch(() => null)) as ApiEnvelope<T> | null
  if (!resp.ok || !payload) {
    const code = payload?.code ?? 'NETWORK_ERROR'
    const message = payload?.message ?? `服务异常（${resp.status}）`
    throw new ApiError(code, message, resp.status)
  }
  return payload.data
}

// uploadFile 处理 multipart/form-data（不带 JSON Content-Type，浏览器自动加 boundary）
export async function uploadFile<T>(path: string, field: string, file: File): Promise<T> {
  const form = new FormData()
  form.append(field, file)
  const resp = await fetch(`/api/v1${path}`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${tokenStore.get() ?? ''}` },
    body: form,
  })
  const payload = (await resp.json().catch(() => null)) as ApiEnvelope<T> | null
  if (!resp.ok || !payload) {
    throw new ApiError(payload?.code ?? 'NETWORK_ERROR', payload?.message ?? '上传失败', resp.status)
  }
  return payload.data
}

export const api = {
  get: <T>(path: string, signal?: AbortSignal) => request<T>(path, { signal }),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: 'POST', body }),
}
