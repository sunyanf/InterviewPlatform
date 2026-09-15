import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { ApiError } from '../api/client'
import { AuthPanel } from '../components/AuthPanel'

export default function RegisterPage() {
  const { register } = useAuth()
  const navigate = useNavigate()

  const [nickname, setNickname] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (password.length < 6) {
      setError('密码至少 6 位')
      return
    }
    setBusy(true)
    try {
      await register(email.trim(), password, nickname.trim())
      navigate('/')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '注册失败，请稍后再试')
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthPanel
      kicker="FIRST SESSION"
      title={<>建立你的<br />第一份考场档案</>}
      quote="“每一次模拟，都是正式 offer 的彩排。”"
    >
      <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
        <h2 style={{ fontSize: 24 }}>创建账号</h2>
        {error && <div className="notice notice-error">{error}</div>}
        <div className="field">
          <label>昵称</label>
          <input
            className="input"
            required
            maxLength={32}
            placeholder="考官将这样称呼你"
            value={nickname}
            onChange={(e) => setNickname(e.target.value)}
          />
        </div>
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
            minLength={6}
            placeholder="至少 6 位"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        <button className="btn btn-primary" disabled={busy} style={{ marginTop: 4 }}>
          {busy ? <span className="spinner" /> : '开始第一场模拟'}
        </button>
        <p className="dim" style={{ fontSize: 13.5, textAlign: 'center' }}>
          已有账号？<Link to="/login">直接登录</Link>
        </p>
      </form>
    </AuthPanel>
  )
}
