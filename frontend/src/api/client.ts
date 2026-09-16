import type { ApiEnvelope, LoginResponse } from './types'

// token 存取：localStorage 键集中管理（access / refresh 分离）
const TOKEN_KEY = 'aip_token'
const REFRESH_KEY = 'aip_refresh_token'

export const tokenStore = {
  get: () => localStorage.getItem(TOKEN_KEY),
  set: (t: string) => localStorage.setItem(TOKEN_KEY, t),
  getRefresh: () => localStorage.getItem(REFRESH_KEY),
  setRefresh: (t: string) => localStorage.setItem(REFRESH_KEY, t),
  clear: () => {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REFRESH_KEY)
  },
}

// 保存登录/刷新响应中的令牌对
export function saveTokens(data: LoginResponse): void {
  tokenStore.set(data.token)
  tokenStore.setRefresh(data.refresh_token)
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
  // 内部标记：401 后已重试过，防止无限循环
  retried?: boolean
}

// 刷新请求单飞行（single-flight）：并发 401 只触发一次 /auth/refresh
let refreshInFlight: Promise<string | null> | null = null

async function refreshAccessToken(): Promise<string | null> {
  const refreshToken = tokenStore.getRefresh()
  if (!refreshToken) return null
  if (!refreshInFlight) {
    refreshInFlight = (async () => {
      try {
        const resp = await fetch('/api/v1/auth/refresh', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_token: refreshToken }),
        })
        if (!resp.ok) return null
        const payload = (await resp.json()) as ApiEnvelope<LoginResponse>
        if (!payload.data?.token) return null
        saveTokens(payload.data)
        return payload.data.token
      } catch {
        return null
      } finally {
        refreshInFlight = null
      }
    })()
  }
  return refreshInFlight
}

// request 统一请求：注入 Bearer、解包 {code,message,data}、归一化错误；
// 401 时用 refresh token 静默旋转 access token 并重试一次
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

  // 非认证类接口的 401：尝试一次静默刷新后重放原请求
  const isAuthPath = path.startsWith('/auth/')
  if (resp.status === 401 && !isAuthPath && !opts.retried && tokenStore.getRefresh()) {
    const newToken = await refreshAccessToken()
    if (newToken) {
      return request<T>(path, { ...opts, retried: true })
    }
    tokenStore.clear()
  }

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
// extra 用于附加 duration_ms 等普通表单字段
export async function uploadFile<T>(
  path: string,
  field: string,
  file: File,
  extra: Record<string, string> = {},
): Promise<T> {
  const form = new FormData()
  form.append(field, file)
  Object.entries(extra).forEach(([k, v]) => form.append(k, v))
  const doUpload = (accessToken: string | null) =>
    fetch(`/api/v1${path}`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${accessToken ?? ''}` },
      body: form,
    })

  let resp = await doUpload(tokenStore.get())
  if (resp.status === 401 && tokenStore.getRefresh()) {
    const newToken = await refreshAccessToken()
    if (newToken) resp = await doUpload(newToken)
    else tokenStore.clear()
  }

  const payload = (await resp.json().catch(() => null)) as ApiEnvelope<T> | null
  if (!resp.ok || !payload) {
    throw new ApiError(payload?.code ?? 'NETWORK_ERROR', payload?.message ?? '上传失败', resp.status)
  }
  return payload.data
}

export const api = {
  get: <T>(path: string, signal?: AbortSignal) => request<T>(path, { signal }),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: 'POST', body }),
  patch: <T>(path: string, body?: unknown) => request<T>(path, { method: 'PATCH', body }),
}
