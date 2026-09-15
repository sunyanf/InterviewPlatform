import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { evaluationApi, reportApi } from '../api/endpoints'
import { ApiError } from '../api/client'
import { TaskFailedError, waitForTask } from '../api/taskPolling'
import type { Evaluation, Report } from '../api/types'

type Phase = 'evaluating' | 'generating' | 'done' | 'error'

const STAGES: Record<Phase, string> = {
  evaluating: '考官正在逐题复核你的回答…',
  generating: '正在生成能力画像与训练计划…',
  done: '完成',
  error: '生成失败，请稍后重试',
}

export default function ReportPage() {
  const { id = '' } = useParams()
  const [phase, setPhase] = useState<Phase>('evaluating')
  const [evaluation, setEvaluation] = useState<Evaluation | null>(null)
  const [report, setReport] = useState<Report | null>(null)
  const [errorMsg, setErrorMsg] = useState('')
  // 失败后点击「重新生成」递增 nonce，重新跑一遍评估/报告流水线
  const [nonce, setNonce] = useState(0)

  useEffect(() => {
    let cancelled = false

    const fail = (err: unknown) => {
      if (err instanceof TaskFailedError) {
        setErrorMsg(err.message || '评估任务执行失败')
      } else {
        setErrorMsg(err instanceof ApiError ? err.message : '评估服务暂不可用')
      }
      setPhase('error')
    }

    // GET 复用已有产物；404 → POST 提交异步任务（202）→ 轮询至成功 → GET 拉取产物
    async function run() {
      try {
        let evaluation: Evaluation
        try {
          evaluation = await evaluationApi.get(id)
        } catch (err) {
          if (err instanceof ApiError && err.status === 404) {
            const accepted = await evaluationApi.run(id)
            await waitForTask(accepted)
            evaluation = await evaluationApi.get(id)
          } else {
            throw err
          }
        }
        if (cancelled) return
        setEvaluation(evaluation)
        setPhase('generating')

        let report: Report
        try {
          report = await reportApi.get(id)
        } catch (err) {
          if (err instanceof ApiError && err.status === 404) {
            const accepted = await reportApi.generate(id)
            await waitForTask(accepted)
            report = await reportApi.get(id)
          } else {
            throw err
          }
        }
        if (cancelled) return
        setReport(report)
        setPhase('done')
      } catch (err) {
        if (!cancelled) fail(err)
      }
    }

    run()
    return () => {
      cancelled = true
    }
  }, [id, nonce])

  const retry = () => {
    setErrorMsg('')
    setEvaluation(null)
    setReport(null)
    setPhase('evaluating')
    setNonce((n) => n + 1)
  }

  if (phase === 'error') {
    return (
      <main className="shell" style={{ paddingTop: 80, textAlign: 'center' }}>
        <h1 style={{ fontSize: 26, marginBottom: 12 }}>评卷遇到问题</h1>
        <p className="muted" style={{ marginBottom: 24 }}>{errorMsg}</p>
        <div style={{ display: 'flex', gap: 12, justifyContent: 'center' }}>
          <button className="btn btn-primary" onClick={retry}>
            重新生成
          </button>
          <Link to="/" className="btn btn-ghost">
            返回我的面试
          </Link>
        </div>
      </main>
    )
  }

  if (phase !== 'done' || !report || !evaluation) {
    return (
      <div style={{ display: 'grid', placeItems: 'center', height: '80vh' }}>
        <div style={{ textAlign: 'center' }}>
          <div className="spinner" style={{ margin: '0 auto 18px' }} />
          <p className="muted">{STAGES[phase]}</p>
        </div>
      </div>
    )
  }

  return <ReportView report={report} evaluation={evaluation} />
}

function scoreColor(score: number): string {
  if (score >= 80) return 'var(--jade)'
  if (score >= 60) return 'var(--accent)'
  return 'var(--clay)'
}

function scoreGrade(score: number): string {
  if (score >= 90) return '卓越'
  if (score >= 80) return '优秀'
  if (score >= 70) return '良好'
  if (score >= 60) return '及格'
  return '待提升'
}

function ReportView({ report, evaluation }: { report: Report; evaluation: Evaluation }) {
  const dims = evaluation.rubric.dimensions
  const hist = report.capability_profile.history

  return (
    <main className="shell" style={{ paddingTop: 40, paddingBottom: 70, maxWidth: 940 }}>
      {/* 抬头 */}
      <header className="rise" style={{ textAlign: 'center', marginBottom: 34 }}>
        <span className="mono dim" style={{ fontSize: 11, letterSpacing: '0.36em' }}>
          EXAMINER'S REPORT
        </span>
        <h1 style={{ fontSize: 34, marginTop: 10 }}>面试评估报告</h1>
        <div className="dim mono" style={{ fontSize: 12, marginTop: 6 }}>
          {evaluation.model} · prompt {evaluation.prompt_version}
        </div>
      </header>

      {/* 总分卡 */}
      <section
        className="card rise rise-1"
        style={{ padding: '34px 40px', display: 'flex', alignItems: 'center', gap: 40, marginBottom: 18, flexWrap: 'wrap' }}
      >
        <div style={{ textAlign: 'center', minWidth: 150 }}>
          <div
            style={{
              fontFamily: 'var(--font-display)',
              fontSize: 78,
              lineHeight: 1,
              color: scoreColor(report.total_score),
            }}
          >
            {report.total_score.toFixed(1)}
          </div>
          <div
            style={{
              marginTop: 8,
              display: 'inline-block',
              padding: '3px 16px',
              border: `1px solid ${scoreColor(report.total_score)}`,
              borderRadius: 999,
              color: scoreColor(report.total_score),
              fontSize: 13,
              letterSpacing: '0.2em',
            }}
          >
            {scoreGrade(report.total_score)}
          </div>
        </div>

        <div style={{ flex: 1, minWidth: 280, display: 'flex', flexDirection: 'column', gap: 14 }}>
          {dims.map((d) => {
            const v = evaluation.dimensions[d.name] ?? 0
            return (
              <div key={d.name}>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 13.5, marginBottom: 5 }}>
                  <span>
                    {d.label}
                    <span className="dim mono" style={{ fontSize: 11, marginLeft: 8 }}>
                      权重 {Math.round(d.weight * 100)}%
                    </span>
                  </span>
                  <span className="mono" style={{ color: scoreColor(v) }}>
                    {v.toFixed(1)}
                    {hist && (
                      <span className="dim" style={{ fontSize: 11, marginLeft: 8 }}>
                        历史 {hist.avg_dimensions[d.name]?.toFixed(1) ?? '—'}
                      </span>
                    )}
                  </span>
                </div>
                <div style={{ height: 6, background: '#0e1319', borderRadius: 3, overflow: 'hidden' }}>
                  <div
                    style={{
                      width: `${Math.max(0, Math.min(100, v))}%`,
                      height: '100%',
                      background: scoreColor(v),
                      borderRadius: 3,
                      transition: 'width .9s var(--ease)',
                    }}
                  />
                </div>
              </div>
            )
          })}
        </div>

        {hist && (
          <div style={{ minWidth: 130, textAlign: 'center' }}>
            <div className="dim" style={{ fontSize: 12, marginBottom: 4 }}>较历史 {hist.compared_count} 场均值</div>
            <div
              className="mono"
              style={{ fontSize: 26, color: hist.delta_total_score >= 0 ? 'var(--jade)' : 'var(--clay)' }}
            >
              {hist.delta_total_score >= 0 ? '+' : ''}
              {hist.delta_total_score.toFixed(1)}
            </div>
          </div>
        )}
      </section>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 18 }} className="report-grid">
        <ListCard title="✓ 表现亮点" items={report.strengths} tone="jade" className="rise rise-2" />
        <ListCard title="! 主要不足" items={report.weaknesses} tone="clay" className="rise rise-2" />
      </div>

      {/* 证据 */}
      <section className="card rise rise-3" style={{ padding: 26, margin: '18px 0' }}>
        <h2 style={{ fontSize: 18, marginBottom: 16 }}>评分依据（Evidence）</h2>
        {evaluation.evidence.length === 0 && <p className="dim" style={{ fontSize: 14 }}>本场未记录到显著扣分项。</p>}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          {evaluation.evidence.map((ev, i) => (
            <div
              key={i}
              style={{
                borderLeft: '2px solid var(--accent)',
                padding: '4px 0 4px 16px',
                fontSize: 13.5,
                lineHeight: 1.7,
              }}
            >
              <div style={{ marginBottom: 2 }}>
                <span className="tag tag-accent" style={{ marginRight: 8 }}>
                  第 {ev.question_seq} 题
                </span>
                <strong>{ev.issue}</strong>
              </div>
              <div className="muted">依据：{ev.evidence}</div>
              {ev.reference && <div className="dim">参考：{ev.reference}</div>}
            </div>
          ))}
        </div>
      </section>

      {/* 建议 */}
      {evaluation.recommendations.length > 0 && (
        <section className="card rise rise-3" style={{ padding: 26, marginBottom: 18 }}>
          <h2 style={{ fontSize: 18, marginBottom: 14 }}>考官建议</h2>
          <ul style={{ marginLeft: 20, lineHeight: 2, fontSize: 14.5 }}>
            {evaluation.recommendations.map((r, i) => (
              <li key={i}>{r}</li>
            ))}
          </ul>
        </section>
      )}

      {/* 学习计划 */}
      <section className="card rise rise-4" style={{ padding: 26, marginBottom: 26 }}>
        <h2 style={{ fontSize: 18, marginBottom: 18 }}>下一阶段训练计划</h2>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {report.learning_plan.focus_areas.map((f, i) => (
            <div key={i} style={{ paddingBottom: 16, borderBottom: i < report.learning_plan.focus_areas.length - 1 ? '1px solid var(--line)' : 'none' }}>
              <div style={{ fontFamily: 'var(--font-display)', fontSize: 16, marginBottom: 4 }}>{f.topic}</div>
              <p className="dim" style={{ fontSize: 13.5, marginBottom: 6 }}>{f.reason}</p>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                {f.suggestions.map((s, j) => (
                  <span key={j} className="tag">{s}</span>
                ))}
              </div>
            </div>
          ))}
        </div>

        <div
          style={{
            marginTop: 20,
            padding: '18px 20px',
            background: 'var(--accent-soft)',
            border: '1px solid rgba(217,164,65,.3)',
            borderRadius: 'var(--radius-s)',
          }}
        >
          <div className="mono dim" style={{ fontSize: 11, letterSpacing: '0.24em', marginBottom: 6 }}>
            NEXT TRAINING
          </div>
          <div style={{ fontSize: 15, marginBottom: 6 }}>
            建议聚焦：<strong style={{ color: 'var(--accent)' }}>{report.learning_plan.next_training.focus}</strong>
          </div>
          <div className="muted" style={{ fontSize: 13.5 }}>
            下次可练 {report.learning_plan.next_training.suggested_question_type} ·{' '}
            {report.learning_plan.next_training.suggested_difficulty} 难度
            {report.learning_plan.next_training.suggested_topics.length > 0 && (
              <> · 主题：{report.learning_plan.next_training.suggested_topics.join('、')}</>
            )}
          </div>
        </div>
      </section>

      <div style={{ display: 'flex', gap: 12, justifyContent: 'center' }}>
        <Link to="/jobs" className="btn btn-primary">
          再练一场
        </Link>
        <Link to="/" className="btn btn-ghost">
          返回我的面试
        </Link>
      </div>
    </main>
  )
}

function ListCard({
  title,
  items,
  tone,
  className,
}: {
  title: string
  items: string[]
  tone: 'jade' | 'clay'
  className?: string
}) {
  return (
    <section className={`card ${className ?? ''}`} style={{ padding: 24 }}>
      <h2 style={{ fontSize: 17, marginBottom: 14 }}>{title}</h2>
      {items.length === 0 ? (
        <p className="dim" style={{ fontSize: 13.5 }}>暂无记录</p>
      ) : (
        <ul style={{ marginLeft: 18, lineHeight: 1.9, fontSize: 14 }}>
          {items.map((it, i) => (
            <li key={i} style={{ color: tone === 'jade' ? 'var(--ink)' : 'var(--ink)' }}>
              {it}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
