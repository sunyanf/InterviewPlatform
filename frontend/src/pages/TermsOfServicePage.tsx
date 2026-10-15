import type { ReactNode } from 'react'

// 用户协议：公开页面，无需登录；版式复用 .shell + .card 与既有排版变量
export default function TermsOfServicePage() {
  return (
    <main className="shell" style={{ paddingTop: 48, paddingBottom: 60, maxWidth: 820 }}>
      <header style={{ marginBottom: 26 }}>
        <span className="mono dim" style={{ fontSize: 11, letterSpacing: '0.36em' }}>
          LEGAL · TERMS
        </span>
        <h1 style={{ fontSize: 30, marginTop: 10 }}>用户协议</h1>
        <p className="dim" style={{ fontSize: 13, marginTop: 8 }}>
          版本 v1.0 · 生效日期：【待填：生效日期】
        </p>
      </header>

      <section className="card" style={{ padding: '30px 34px' }}>
        <Section title="一、服务说明">
          本服务是第三方 AI 面试训练工具，向你提供模拟面试练习、回答分析与训练建议。本平台不是官方机构，与任何用人单位、考试机构、政府或官方组织无关；AI 生成的内容仅供参考，不构成真实面试结果、考试成绩或任何官方评价。
        </Section>

        <Section title="二、账号规则">
          你应提供真实、准确、合法的注册信息，并妥善保管账号与密码。因你保管不当导致的损失由你自行承担。你不得将账号转让、出租或出借给他人使用。
        </Section>

        <Section title="三、用户行为规范">
          你承诺不上传、不提交、不传播任何违反法律法规或公序良俗的内容，
          <Bullets
            items={[
              '违法违规、涉政敏感、暴力恐怖、色情低俗内容；',
              '侮辱、诽谤、虚假、欺诈及侵犯他人合法权益的内容；',
              '其他违反本协议或平台规则的内容。',
            ]}
          />
          你提交的内容可能经过内容安全审核；对违规内容，我们有权拒绝处理或采取必要措施。
        </Section>

        <Section title="四、内容安全条款">
          你理解并同意，为履行法律义务与保障平台安全，我们可能对相关内容进行审核、过滤或留存记录。若发现违法或不良信息，我们有权依法处理并向有关部门报告。
        </Section>

        <Section title="五、知识产权">
          本服务的软件、界面、文档及 AI 生成的训练内容的知识产权归【待填：运营主体名称】或相关权利人所有。你对你上传、提交的内容享有相应权利，并授权我们在提供本服务所必需的范围内使用。
        </Section>

        <Section title="六、责任限制">
          在法律允许的最大范围内，我们不对因使用或无法使用本服务而产生的间接损失、预期利益损失，以及你依赖 AI 内容作出的决策后果承担责任。本服务按“现状”提供，我们不对服务的绝对准确、及时或不中断作出保证。
        </Section>

        <Section title="七、免责声明">
          AI 生成的题目、评分、分析与建议仅供参考，不构成真实面试结果、考试成绩或任何官方评价，不应作为录用、报考或其它决策的依据。你应自行判断并对相关决策负责。
        </Section>

        <Section title="八、协议变更">
          我们可能不时更新本协议。更新后我们将通过站内公告、【待填：通知方式】等方式通知你；若你继续使用本服务，即视为接受更新后的协议。
        </Section>

        <Section title="九、适用与管辖" last>
          本协议适用中华人民共和国法律。因本协议引起的争议，双方应友好协商解决；协商不成的，提交【待填：管辖法院】诉讼解决。运营主体：【待填：运营主体名称】，联系方式：【待填：联系方式】。
        </Section>
      </section>
    </main>
  )
}

function Section({ title, children, last }: { title: string; children: ReactNode; last?: boolean }) {
  return (
    <section style={{ marginBottom: last ? 0 : 24 }}>
      <h2 style={{ fontSize: 18, marginBottom: 8 }}>{title}</h2>
      <div className="muted" style={{ fontSize: 14.5, lineHeight: 1.9 }}>
        {children}
      </div>
    </section>
  )
}

function Bullets({ items }: { items: string[] }) {
  return (
    <ul style={{ margin: '8px 0 8px 20px', lineHeight: 1.9 }}>
      {items.map((it, i) => (
        <li key={i}>{it}</li>
      ))}
    </ul>
  )
}
