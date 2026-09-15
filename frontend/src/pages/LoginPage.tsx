import { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { ApiError } from '../api/client'
import { AuthPanel } from '../components/AuthPanel'

export default function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as { from?: { pathname: string } } | null)?.from?.pathname ?? '/'

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      await login(email.trim(), password)
      navigate(from, { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '登录失败，请稍后再试')
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthPanel
      kicker="NIGHT BEFORE THE OFFER"
      title={<>在深夜，<br />提前走进那间考场</>}
      quote="“准备得越充分，运气就越好。”"
    >
      <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
        <h2 style={{ fontSize: 24 }}>欢迎回来</h2>
        {error && <div className="notice notice-error">{error}</div>}
        <div className="field">
          <label>邮箱</label>
          <input
            className="input"
            type="email"
            required
            placeholder="you@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </div>
        <div className="field">
          <label>密码</label>
          <input
            className="input"
            type="password"
            required
            placeholder="至少 6 位"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        <button className="btn btn-primary" disabled={busy} style={{ marginTop: 4 }}>
          {busy ? <span className="spinner" /> : '进入面试室'}
        </button>
        <p className="dim" style={{ fontSize: 13.5, textAlign: 'center' }}>
          还没有账号？<Link to="/register">注册一个</Link>
        </p>
      </form>
    </AuthPanel>
  )
}
