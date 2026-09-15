import type { ReactNode } from 'react'

// AuthPanel 登录/注册双栏布局：左侧考场氛围叙事，右侧表单
export function AuthPanel({
  title,
  kicker,
  quote,
  children,
}: {
  title: ReactNode
  kicker: string
  quote: string
  children: ReactNode
}) {
  return (
    <div
      style={{
        minHeight: 'calc(100vh - 66px)',
        display: 'grid',
        gridTemplateColumns: '1.05fr 0.95fr',
      }}
      className="auth-grid"
    >
      {/* 左：叙事 */}
      <section
        className="rise"
        style={{
          padding: '7vh 5vw 7vh 7vw',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          gap: 28,
          borderRight: '1px solid var(--line)',
        }}
      >
        <span className="mono dim" style={{ fontSize: 11.5, letterSpacing: '0.38em' }}>
          {kicker}
        </span>
        <h1 style={{ fontSize: 'clamp(34px, 4.4vw, 58px)', maxWidth: 560 }}>{title}</h1>
        <p className="muted" style={{ maxWidth: 440, fontSize: 15.5 }}>
          基于你的简历与目标岗位，AI 考官会生成技术追问、记录每一次迟疑，并在终场后给出带证据的评分与训练计划。
        </p>
        <div
          style={{
            width: 56,
            height: 1,
            background: 'var(--accent)',
            opacity: 0.6,
          }}
        />
        <span className="muted" style={{ fontFamily: 'var(--font-display)', fontSize: 17 }}>
          {quote}
        </span>
      </section>

      {/* 右：表单 */}
      <section
        className="rise rise-1"
        style={{
          padding: '7vh 7vw',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <div className="card" style={{ width: '100%', maxWidth: 400, padding: 36 }}>
          {children}
        </div>
      </section>
    </div>
  )
}
