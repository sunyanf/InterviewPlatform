import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'

export function Topbar() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  return (
    <header className="topbar">
      <div className="shell" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
        <Link to="/" className="brand">
          <span className="mark">❖</span>
          模拟面试室
          <small>INTERVIEW · ROOM</small>
        </Link>
        {user && (
          <nav style={{ display: 'flex', alignItems: 'center', gap: 22 }}>
            <Link to="/" className="muted" style={{ fontSize: 14 }}>
              我的面试
            </Link>
            <Link to="/jobs" className="muted" style={{ fontSize: 14 }}>
              选择岗位
            </Link>
            {user.role === 'admin' && (
              <Link to="/admin" className="muted" style={{ fontSize: 14 }}>
                管理后台
              </Link>
            )}
            <span className="dim" style={{ fontSize: 13 }}>
              {user.nickname || user.email}
            </span>
            <button
              className="btn btn-ghost"
              style={{ padding: '6px 14px', fontSize: 13 }}
              onClick={() => {
                logout()
                navigate('/login')
              }}
            >
              退出
            </button>
          </nav>
        )}
      </div>
    </header>
  )
}
