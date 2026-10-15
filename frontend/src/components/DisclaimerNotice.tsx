import { Link } from 'react-router-dom'

// 核心免责文案：所有变体共用，措辞严谨、不夸大平台能力
const CORE_TEXT =
  '本平台为第三方 AI 面试训练工具，与任何用人单位、考试机构、政府或官方组织无关。AI 生成的题目、评分、分析与建议仅供参考，不构成真实面试结果、考试成绩或任何官方评价，不应作为录用、报考或其它决策的依据。'

// DisclaimerNotice 免责声明提示
// variant=compact（默认）：页面内嵌的轻量提示；variant=full：独立完整展示
export function DisclaimerNotice({ variant = 'compact' }: { variant?: 'compact' | 'full' }) {
  const links = (
    <>
      详见 <Link to="/legal/terms">《用户协议》</Link> 与 <Link to="/legal/privacy">《隐私政策》</Link>。
    </>
  )

  if (variant === 'full') {
    return (
      <section className="card" style={{ padding: '22px 26px' }}>
        <div className="mono dim" style={{ fontSize: 11, letterSpacing: '0.28em', marginBottom: 8 }}>
          DISCLAIMER
        </div>
        <h2 style={{ fontSize: 17, marginBottom: 8 }}>免责声明</h2>
        <p className="muted" style={{ fontSize: 14, lineHeight: 1.9 }}>{CORE_TEXT}</p>
        <p className="dim" style={{ fontSize: 13, marginTop: 10 }}>{links}</p>
      </section>
    )
  }

  return (
    <div
      className="dim"
      style={{
        fontSize: 12.5,
        lineHeight: 1.7,
        padding: '10px 14px',
        background: 'var(--bg-3)',
        border: '1px solid var(--line)',
        borderRadius: 'var(--radius-s)',
      }}
    >
      {CORE_TEXT}
      <br />
      {links}
    </div>
  )
}
