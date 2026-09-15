import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { interviewApi } from '../api/endpoints'
import type { InterviewSession } from '../api/types'
import { MODE_LABEL, STATUS_LABEL, TYPE_LABEL, formatTime, statusTagClass } from '../lib/display'

export default function DashboardPage() {
  const navigate = useNavigate()
  const { data: sessions, isLoading } = useQuery({
    queryKey: ['interviews'],
    queryFn: interviewApi.list,
  })

  const enter = (s: InterviewSession) => {
    if (s.status === 'COMPLETED') navigate(`/report/${s.id}`)
    else navigate(`/interview/${s.id}`)
  }

  const running = (sessions ?? []).filter((s) => s.status === 'RUNNING').length

  return (
    <main className="shell" style={{ paddingTop: 44, paddingBottom: 60 }}>
      <header
        className="rise"
        style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', marginBottom: 30 }}
      >
        <div>
          <span className="mono dim" style={{ fontSize: 11.5, letterSpacing: '0.3em' }}>
            MY SESSIONS
          </span>
          <h1 style={{ fontSize: 34, marginTop: 8 }}>我的面试</h1>
          {running > 0 && <p className="muted" style={{ marginTop: 6 }}>有 {running} 场进行中的面试在等你</p>}
        </div>
        <Link to="/jobs" className="btn btn-primary">
          开启新的模拟
        </Link>
      </header>

      {isLoading && <div className="spinner" />}

      {!isLoading && sessions && sessions.length === 0 && (
        <div className="card rise rise-1" style={{ padding: '56px 32px', textAlign: 'center' }}>
          <div style={{ fontSize: 40, marginBottom: 12 }}>🪑</div>
          <h2 style={{ fontSize: 22, marginBottom: 8 }}>考场还是空的</h2>
          <p className="muted" style={{ marginBottom: 24 }}>
            选择一个目标岗位、上传简历，AI 考官会为你量身出题。
          </p>
          <Link to="/jobs" className="btn btn-primary" style={{ margin: '0 auto' }}>
            去选岗位
          </Link>
        </div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {(sessions ?? []).map((s, i) => (
          <button
            key={s.id}
            className="card rise"
            style={{
              animationDelay: `${0.05 + i * 0.05}s`,
              padding: '20px 24px',
              textAlign: 'left',
              background: 'transparent',
              color: 'inherit',
              border: '1px solid var(--line)',
              borderRadius: 'var(--radius-m)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 18,
            }}
            onClick={() => enter(s)}
          >
            <div style={{ flex: 1 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 6 }}>
                <span className={statusTagClass(s.status)}>{STATUS_LABEL[s.status] ?? s.status}</span>
                <span className="tag">{TYPE_LABEL[s.interview_type] ?? s.interview_type}</span>
                <span className="tag">{MODE_LABEL[s.mode] ?? s.mode}</span>
              </div>
              <div style={{ fontFamily: 'var(--font-display)', fontSize: 19 }}>
                {s.job_title || '目标岗位'}
              </div>
              <div className="dim mono" style={{ fontSize: 12, marginTop: 4 }}>
                {formatTime(s.created_at)} · 已答 {s.answered_count}/{s.config.question_count} 题
              </div>
            </div>
            <span className="dim" style={{ fontSize: 18 }}>→</span>
          </button>
        ))}
      </div>
    </main>
  )
}
