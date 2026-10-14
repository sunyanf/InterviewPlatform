import { Link } from 'react-router-dom'

// Footer 极简页脚：运营主体占位 + 合规文本入口
export function Footer() {
  return (
    <footer style={{ borderTop: '1px solid var(--line)', marginTop: 48 }}>
      <div
        className="shell"
        style={{
          paddingTop: 22,
          paddingBottom: 22,
          display: 'flex',
          flexWrap: 'wrap',
          gap: 14,
          justifyContent: 'space-between',
          alignItems: 'center',
        }}
      >
        <span className="dim" style={{ fontSize: 12.5 }}>
          【待填：运营主体名称】
        </span>
        <nav style={{ display: 'flex', gap: 18, fontSize: 13 }}>
          <Link to="/legal/terms" className="muted">
            用户协议
          </Link>
          <Link to="/legal/privacy" className="muted">
            隐私政策
          </Link>
        </nav>
      </div>
    </footer>
  )
}
