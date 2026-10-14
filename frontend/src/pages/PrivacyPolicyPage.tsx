import type { ReactNode } from 'react'

// 隐私政策：公开页面，无需登录；版式复用 .shell + .card 与既有排版变量
export default function PrivacyPolicyPage() {
  return (
    <main className="shell" style={{ paddingTop: 48, paddingBottom: 60, maxWidth: 820 }}>
      <header style={{ marginBottom: 26 }}>
        <span className="mono dim" style={{ fontSize: 11, letterSpacing: '0.36em' }}>
          LEGAL · PRIVACY
        </span>
        <h1 style={{ fontSize: 30, marginTop: 10 }}>隐私政策</h1>
        <p className="dim" style={{ fontSize: 13, marginTop: 8 }}>
          版本 v1.0 · 生效日期：【待填：生效日期】
        </p>
      </header>

      <section className="card" style={{ padding: '30px 34px' }}>
        <Section title="一、引言">
          本政策说明【待填：运营主体名称】（下称“我们”）在向你提供 AI 模拟面试训练服务（下称“本服务”）时，如何收集、使用、存储、共享与保护你的个人信息。请在使用本服务前仔细阅读并充分理解本政策。
        </Section>

        <Section title="二、我们收集的信息">
          为向你提供本服务，我们可能收集以下类别的信息：
          <Bullets
            items={[
              '账号信息：注册所需的邮箱、昵称，以及加密存储的密码凭据。',
              '简历及其解析结果：你上传的简历文件，以及系统解析得到的经历、技能等结构化信息。',
              '面试回答文本：你在模拟面试中提交的文字回答。',
              '录音与音频：你使用语音作答功能时录制的音频。',
              '转写文本：由语音识别（ASR）将音频转换得到的文本。',
              '日志信息：设备与浏览器类型、操作记录及必要的诊断日志，用于保障服务稳定与安全。',
            ]}
          />
        </Section>

        <Section title="三、处理目的">
          我们基于提供与改进本服务的目的处理上述信息，包括：生成面试题目与追问、分析并评价你的回答、给出训练建议，以及进行质量改进与故障排查。我们不会将你的信息用于与服务无关的用途。
        </Section>

        <Section title="四、第三方处理方">
          为实现上述功能，我们可能委托第三方处理方处理你的信息（以下均按类别描述，不披露具体厂商名称）：
          <Bullets
            items={[
              '通用大语言模型（LLM）服务：用于题目生成、回答分析与建议生成。',
              '语音识别（ASR）服务：用于将你的录音转写为文本。',
              '语音合成（TTS）服务：用于朗读面试官开场白等文本。',
              '对象存储服务：用于存储简历、录音等文件。',
            ]}
          />
          我们会与上述处理方约定数据处理与保密义务，并要求其仅在必要范围内处理相关信息。
        </Section>

        <Section title="五、存储与保留期限">
          你的信息存储于【待填：数据存储地点】。我们仅在实现处理目的所必需的期限内保留信息，超出期限后将予以删除或匿名化处理；具体保留期限为【待填：数据保留期限】。法律法规另有要求的，从其规定。
        </Section>

        <Section title="六、你的权利">
          你有权访问、更正你的个人信息，删除相关信息，以及撤回此前的同意。数据删除入口将在后续版本提供（【待填：数据删除入口】）；在入口开放前，你可通过【待填：联系方式】联系我们行使上述权利。
        </Section>

        <Section title="七、未成年人使用">
          本服务主要面向成年求职者。若你为未成年人，请在监护人同意与指导下使用本服务，并避免提交敏感个人信息。
        </Section>

        <Section title="八、安全措施">
          我们采取加密传输、访问控制、最小权限等合理的安全措施保护你的信息，防止未经授权的访问、泄露、篡改或丢失。但请注意，任何安全措施都无法保证绝对安全。
        </Section>

        <Section title="九、政策变更">
          本政策如有变更，我们将通过站内公告、【待填：通知方式】等方式通知你；涉及你权利的重大变更会以显著方式提示。变更后的政策自【待填：生效日期】起生效。
        </Section>

        <Section title="十、联系我们" last>
          如对本政策有任何疑问，请通过【待填：联系方式】与我们联系。运营主体：【待填：运营主体名称】。
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
    <ul style={{ margin: '8px 0 0 20px', lineHeight: 1.9 }}>
      {items.map((it, i) => (
        <li key={i}>{it}</li>
      ))}
    </ul>
  )
}
