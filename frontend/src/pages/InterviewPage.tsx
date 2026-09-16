import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { interviewApi } from '../api/endpoints'
import type { AnswerAnalysis, Question } from '../api/types'
import { useInterviewSocket } from '../ws/useInterviewSocket'
import { QTYPE_LABEL } from '../lib/display'
import { ChatPanel } from '../interview/ChatPanel'
import { VoiceRecorder } from '../interview/VoiceRecorder'
import { isAutoplayEnabled, speak, speechSupported, stopSpeaking } from '../interview/speech'

export default function InterviewPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const socket = useInterviewSocket(id)
  const { status: conn, session, answerEvents, error } = socket

  const [currentId, setCurrentId] = useState<string>('')
  const [finishing, setFinishing] = useState(false)
  const [openingText, setOpeningText] = useState('')

  const questions = useMemo(() => session?.questions ?? [], [session])

  // snapshot 到达后，默认选中第一道未回答的题
  useEffect(() => {
    if (!currentId && questions.length > 0) {
      const next = questions.find((q) => !q.answered) ?? questions[0]
      setCurrentId(next.id)
    }
  }, [questions, currentId])

  // 拉取开场白文本；用户开启“面试官语音”时自动朗读（失败静默，不影响文字面试）
  useEffect(() => {
    if (session?.status === 'RUNNING' && !openingText) {
      interviewApi
        .openingSpeech(id)
        .then((r) => {
          setOpeningText(r.text)
          if (isAutoplayEnabled()) speak(r.text)
        })
        .catch(() => undefined)
    }
  }, [session?.status, id, openingText])

  // 离开面试间时停止朗读
  useEffect(() => () => stopSpeaking(), [])

  const current = questions.find((q) => q.id === currentId) ?? null
  const currentAnalysis = answerEvents.find((e) => e.answer?.question_id === currentId)?.analysis

  const selectQuestion = (q: Question) => setCurrentId(q.id)

  const finish = async () => {
    if (!window.confirm('确认结束本场面试？结束后将进入评分与报告。')) return
    setFinishing(true)
    try {
      await interviewApi.finish(id)
      navigate(`/report/${id}`)
    } catch {
      setFinishing(false)
      window.alert('结束失败，请重试')
    }
  }

  if (!session && conn === 'connecting') {
    return (
      <div style={{ display: 'grid', placeItems: 'center', height: '100vh' }}>
        <div style={{ textAlign: 'center' }}>
          <div className="spinner" style={{ margin: '0 auto 16px' }} />
          <p className="muted">考官正在调阅你的简历与岗位要求…</p>
        </div>
      </div>
    )
  }

  return (
    <div className="iv-layout">
      {/* 左：题目时间轴 */}
      <aside className="iv-panel" style={{ padding: 18 }}>
        <div style={{ marginBottom: 16 }}>
          <div className="mono dim" style={{ fontSize: 10.5, letterSpacing: '0.28em' }}>
            QUESTIONS
          </div>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 17, marginTop: 4 }}>
            {session?.job_title ?? '面试'}
          </div>
          <div style={{ marginTop: 10, display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5 }}>
            <span className={conn === 'open' ? 'dot-live' : 'spinner'} />
            <span className="dim">
              {conn === 'open' ? '实时连接' : conn === 'connecting' ? '重连中…' : '连接已断开'}
            </span>
          </div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 10, overflowY: 'auto', flex: 1 }}>
          {questions.map((q) => {
            const active = q.id === currentId
            return (
              <button
                key={q.id}
                onClick={() => selectQuestion(q)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                  textAlign: 'left',
                  background: 'transparent',
                  border: 'none',
                  color: 'inherit',
                  cursor: 'pointer',
                  padding: 2,
                }}
              >
                <span className={`q-dot ${q.answered ? 'q-dot-done' : active ? 'q-dot-active' : ''}`}>
                  {q.answered ? '✓' : q.seq}
                </span>
                <span
                  style={{
                    fontSize: 13,
                    color: active ? 'var(--ink)' : 'var(--ink-2)',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {QTYPE_LABEL[q.question_type] ?? q.question_type}
                </span>
              </button>
            )
          })}
        </div>

        <button className="btn btn-jade" style={{ marginTop: 14 }} onClick={finish} disabled={finishing}>
          {finishing ? <span className="spinner" /> : '结束面试'}
        </button>
      </aside>

      {/* 中：题面与作答 */}
      <section className="iv-panel">
        <QuestionStage
          key={current?.id ?? 'none'}
          question={current}
          analysis={currentAnalysis}
          openingText={openingText}
          answered={!!current?.answered}
          onSubmit={(text, durationMs) => {
            if (current) socket.sendAnswer(current.id, text, durationMs)
          }}
          busy={answerEvents.some((e) => e.questionId === currentId && !e.answer)}
        />
      </section>

      {/* 右：自由对话 */}
      <aside className="iv-panel">
        <ChatPanel
          bubbles={socket.bubbles}
          streaming={socket.streaming}
          disabled={session?.status !== 'RUNNING'}
          error={error}
          onDismissError={socket.dismissError}
          onSend={socket.sendChat}
        />
      </aside>
    </div>
  )
}

// ---------- 中栏：题面 + 作答 + 分析反馈 ----------
function QuestionStage({
  question,
  analysis,
  openingText,
  answered,
  onSubmit,
  busy,
}: {
  question: Question | null
  analysis: AnswerAnalysis | undefined
  openingText: string
  answered: boolean
  onSubmit: (text: string, durationMs: number) => void
  busy: boolean
}) {
  const [text, setText] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const [slow, setSlow] = useState(false)
  const startedAt = useRef(Date.now())

  useEffect(() => {
    setText('')
    setSubmitted(false)
    setSlow(false)
    startedAt.current = Date.now()
  }, [question?.id])

  useEffect(() => {
    if (answered) setSubmitted(true)
  }, [answered])

  // 客户端兜底：提交后 40s 仍未收到落库确认，提示可先继续（后端分析有 30s 超时降级）
  useEffect(() => {
    if (!submitted || !busy || analysis) return
    const timer = window.setTimeout(() => setSlow(true), 40_000)
    return () => window.clearTimeout(timer)
  }, [submitted, busy, analysis])

  if (!question) {
    return (
      <div style={{ margin: 'auto', textAlign: 'center', padding: 40 }}>
        <p className="muted">等待题目生成…</p>
      </div>
    )
  }

  const submit = () => {
    if (!text.trim()) return
    onSubmit(text, Date.now() - startedAt.current)
    setSubmitted(true)
  }

  return (
    <div style={{ flex: 1, overflowY: 'auto', padding: '30px 34px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 18 }}>
        <span className="tag tag-accent">第 {question.seq} 题</span>
        <span className="tag">{QTYPE_LABEL[question.question_type] ?? question.question_type}</span>
        {question.difficulty && <span className="tag">难度 {question.difficulty}</span>}
      </div>

      <h1 style={{ fontSize: 25, lineHeight: 1.5, marginBottom: 18 }}>{question.question}</h1>

      {openingText && question.seq === 1 && speechSupported() && (
        <button
          type="button"
          className="btn"
          onClick={() => speak(openingText)}
          style={{ marginBottom: 18, padding: '6px 14px', fontSize: 13 }}
          title="再次播放面试官开场白"
        >
          🔊 播放开场白
        </button>
      )}

      {!submitted ? (
        <>
          <textarea
            className="input"
            style={{ minHeight: 200, fontSize: 15, lineHeight: 1.8 }}
            placeholder="在这里输入你的回答，尽量结合项目实例与具体数据…"
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
          <div style={{ marginTop: 12 }}>
            <VoiceRecorder
              questionId={question.id}
              disabled={busy}
              onTranscript={(t) =>
                setText((prev) => (prev.trim() ? `${prev.trimEnd()}\n${t}` : t))
              }
            />
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 14 }}>
            <span className="dim mono" style={{ fontSize: 12 }}>
              {text.trim().length} 字
            </span>
            <button className="btn btn-primary" onClick={submit} disabled={busy || !text.trim()}>
              {busy ? (
                <>
                  <span className="spinner" /> 考官分析中…
                </>
              ) : (
                '提交回答'
              )}
            </button>
          </div>
        </>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div className="iv-panel" style={{ padding: '16px 18px' }}>

            <div className="dim" style={{ fontSize: 12.5, marginBottom: 6 }}>你的回答{question.answer?.text_content ? '' : '（提交中…）'}</div>
            <p style={{ whiteSpace: 'pre-wrap', lineHeight: 1.8, fontSize: 14.5 }}>
              {question.answer?.text_content || text}
            </p>
          </div>

          {busy && !analysis && !slow && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }} className="muted">
              <span className="spinner" /> 正在拆解你的论点与知识缺口…
            </div>
          )}

          {busy && !analysis && slow && (
            <div className="dim" style={{ fontSize: 13 }}>
              分析服务响应较慢，你的回答已提交，可以先从左侧选择其他题目继续作答，分析结果稍后自动出现。
            </div>
          )}

          {!busy && !analysis && question.answer && (
            <div className="dim" style={{ fontSize: 13 }}>
              本轮分析服务繁忙或超时，回答已记录，可继续作答其他题目。
            </div>
          )}

          {analysis && <AnalysisView analysis={analysis} />}
        </div>
      )}
    </div>
  )
}

function AnalysisView({ analysis }: { analysis: AnswerAnalysis }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {analysis.correct_points.length > 0 && (
        <div className="analysis-box analysis-good">
          <strong style={{ color: 'var(--jade)' }}>✓ 答到的点</strong>
          <ul style={{ margin: '6px 0 0 18px' }}>
            {analysis.correct_points.map((p, i) => (
              <li key={i}>{p}</li>
            ))}
          </ul>
        </div>
      )}
      {(analysis.missing_points.length > 0 || analysis.wrong_points.length > 0) && (
        <div className="analysis-box analysis-gap">
          <strong style={{ color: 'var(--accent)' }}>! 待补强</strong>
          <ul style={{ margin: '6px 0 0 18px' }}>
            {analysis.missing_points.map((p, i) => (
              <li key={`m${i}`}>遗漏：{p}</li>
            ))}
            {analysis.wrong_points.map((p, i) => (
              <li key={`w${i}`}>偏差：{p}</li>
            ))}
          </ul>
        </div>
      )}
      {analysis.knowledge_gaps.length > 0 && (
        <p className="dim" style={{ fontSize: 13 }}>
          知识缺口：{analysis.knowledge_gaps.join('；')}
        </p>
      )}
    </div>
  )
}
