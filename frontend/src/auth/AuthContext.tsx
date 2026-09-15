import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { authApi } from '../api/endpoints'
import { saveTokens, tokenStore } from '../api/client'
import type { User } from '../api/types'

interface AuthState {
  user: User | null
  loading: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, nickname: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  // 启动时若有令牌，拉取 /me 恢复会话（access 过期时 client 会自动用 refresh 旋转重试）
  useEffect(() => {
    const token = tokenStore.get()
    if (!token) {
      setLoading(false)
      return
    }
    authApi
      .me()
      .then(setUser)
      .catch(() => tokenStore.clear())
      .finally(() => setLoading(false))
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    const res = await authApi.login(email, password)
    saveTokens(res)
    setUser(res.user)
  }, [])

  const register = useCallback(async (email: string, password: string, nickname: string) => {
    const res = await authApi.register({ email, password, nickname })
    saveTokens(res)
    setUser(res.user)
  }, [])

  const logout = useCallback(async () => {
    // 通知后端吊销 refresh token（网络失败也本地登出，忽略错误）
    const refreshToken = tokenStore.getRefresh()
    if (refreshToken) {
      try {
        await authApi.logout(refreshToken)
      } catch {
        // best effort
      }
    }
    tokenStore.clear()
    setUser(null)
  }, [])

  const value = useMemo(
    () => ({ user, loading, login, register, logout }),
    [user, loading, login, register, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
