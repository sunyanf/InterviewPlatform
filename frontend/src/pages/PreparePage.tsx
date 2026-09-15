import { useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { interviewApi, jobApi, resumeApi } from '../api/endpoints'
import type { Resume } from '../api/types'
import { ApiError } from '../api/client'

type InterviewType = 'technical' | 'behavioral' | 'mixed'

export default function PreparePage() {
  const { jobId = '' } = useParams()
  const navigate = useNavigate()
  const fileRef = useRef<HTMLInputElement>(null)

  const { data: job } = useQuery({ queryKey: ['job', jobId], queryFn: () => jobApi.get(jobId) })
  const { data: resumes, refetch } = useQuery({ queryKey: ['resumes'], queryFn: resumeApi.list })

  const [resumeId, setResumeId] = useState('')
  const [uploading, setUploading] = useState(false)
  const [uploadMsg, setUploadMsg] = useState('')
  const [interviewType, setInterviewType] = useState<InterviewType>('technical')
  const [mode, setMode] = useState<'text' | 'voice'>('text')
  const [questionCount, setQuestionCount] = useState(5)
  const [duration, setDuration] = useState(30)
  const [starting, setStarting] = useState(false)
  const [error, setError] = useState('')

  const handleUpload = async (file: File) => {
    setUploading(true)
    setUploadMsg('')
    setError('')
    try {
      // 上传 → 解析（mock parser 即刻返回结构化简历）
      const rs: Resume = await resumeApi.upload(file)
      const parsed = await resumeApi.parse(rs.id)
      setResumeId(parsed.id)
      setUploadMsg(`「${parsed.file_name}」解析完成，识别 ${parsed.skills.length} 项技能`)
      await refetch()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '简历上传失败')
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  const start = async () => {
    if (!resumeId) {
      setError('请先上传或选择一份简历')
      return
    }
    setStarting(true)
    setError('')
    try {
      const session = await interviewApi.create({
        job_id: jobId,
        resume_id: resumeId,
        interview_type: interviewType,
        mode,
        question_count: questionCount,
        duration_minutes: duration,
      })
      await interviewApi.start(session.id)
      navigate(`/interview/${session.id}`)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '面试创建失败')
      setStarting(false)
    }
  }

  return (
    <main className="shell" style={{ paddingTop: 36, paddingBottom: 60, maxWidth: 880 }}>
      <nav className="dim rise" style={{ fontSize: 13, marginBottom: 18 }}>
        <Link to="/jobs">选择岗位</Link> <span> / </span> 面试准备
      </nav>

      <header className="rise rise-1" style={{ marginBottom: 26 }}>
        <span className="mono dim" style={{ fontSize: 11.5, letterSpacing: '0.3em' }}>
          BRIEFING
        </span>
        <h1 style={{ fontSize: 30, marginTop: 8 }}>{job?.title ?? '岗位加载中…'}</h1>
        {job && (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 12 }}>
            {job.skills.map((s) => (
              <span key={s} className="tag">
                {s}
              </span>
            ))}
          </div>
        )}
      </header>

      {/* 简历 */}
      <section className="card rise rise-2" style={{ padding: 26, marginBottom: 18 }}>
        <h2 style={{ fontSize: 18, marginBottom: 16 }}>① 选择简历</h2>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 16 }}>
          {(resumes ?? []).map((r) => (
            <label
              key={r.id}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 12,
                padding: '12px 16px',
                border: `1px solid ${
                  resumeId === r.id ? 'rgba(217,164,65,.5)' : 'var(--line-strong)'
                }`,
                borderRadius: 'var(--radius-s)',
                background: resumeId === r.id ? 'var(--accent-soft)' : 'transparent',
                cursor: 'pointer',
              }}
            >
              <input
                type="radio"
                name="resume"
                checked={resumeId === r.id}
                onChange={() => setResumeId(r.id)}
                style={{ accentColor: 'var(--accent)' }}
              />
              <span style={{ flex: 1 }}>{r.file_name}</span>
              <span className="dim mono" style={{ fontSize: 12 }}>
                {r.skills.length} 项技能
              </span>
            </label>
          ))}
          {(!resumes || resumes.length === 0) && (
            <p className="dim" style={{ fontSize: 14 }}>还没有简历，上传第一份吧。</p>
          )}
        </div>

        <input
          ref={fileRef}
          type="file"
          accept=".pdf,.doc,.docx,.txt"
          style={{ display: 'none' }}
          onChange={(e) => {
            const f = e.target.files?.[0]
            if (f) handleUpload(f)
          }}
        />
        <button className="btn btn-ghost" onClick={() => fileRef.current?.click()} disabled={uploading}>
          {uploading ? <span className="spinner" /> : '上传新简历（PDF / Word / TXT，≤10MB）'}
        </button>
        {uploadMsg && <p style={{ color: 'var(--jade)', fontSize: 13, marginTop: 10 }}>{uploadMsg}</p>}
      </section>

      {/* 面试配置 */}
      <section className="card rise rise-3" style={{ padding: 26, marginBottom: 22 }}>
        <h2 style={{ fontSize: 18, marginBottom: 16 }}>② 面试设置</h2>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 18 }} className="prepare-grid">
          <div className="field">
            <label>面试类型</label>
            <Segmented
              value={interviewType}
              onChange={(v) => setInterviewType(v as InterviewType)}
              options={[
                { value: 'technical', label: '技术面' },
                { value: 'behavioral', label: '行为面' },
                { value: 'mixed', label: '综合面' },
              ]}
            />
          </div>
          <div className="field">
            <label>答题方式</label>
            <Segmented
              value={mode}
              onChange={(v) => setMode(v as 'text' | 'voice')}
              options={[
                { value: 'text', label: '文字' },
                { value: 'voice', label: '语音' },
              ]}
            />
          </div>
          <div className="field">
            <label>题目数量：{questionCount} 题</label>
            <input
              type="range"
              min={3}
              max={10}
              value={questionCount}
              onChange={(e) => setQuestionCount(Number(e.target.value))}
              style={{ accentColor: 'var(--accent)' }}
            />
          </div>
          <div className="field">
            <label>预计时长：{duration} 分钟</label>
            <input
              type="range"
              min={15}
              max={90}
              step={5}
              value={duration}
              onChange={(e) => setDuration(Number(e.target.value))}
              style={{ accentColor: 'var(--accent)' }}
            />
          </div>
        </div>
      </section>

      {error && <div className="notice notice-error" style={{ marginBottom: 14 }}>{error}</div>}

      <div className="rise rise-4" style={{ display: 'flex', gap: 12 }}>
        <button className="btn btn-primary" onClick={start} disabled={starting} style={{ minWidth: 180 }}>
          {starting ? (
            <>
              <span className="spinner" /> 正在生成考题…
            </>
          ) : (
            '进入考场'
          )}
        </button>
        <Link to="/jobs" className="btn btn-ghost">
          返回
        </Link>
      </div>
    </main>
  )
}

function Segmented({
  value,
  onChange,
  options,
}: {
  value: string
  onChange: (v: string) => void
  options: { value: string; label: string }[]
}) {
  return (
    <div
      style={{
        display: 'flex',
        border: '1px solid var(--line-strong)',
        borderRadius: 'var(--radius-s)',
        overflow: 'hidden',
      }}
    >
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          onClick={() => onChange(o.value)}
          style={{
            flex: 1,
            padding: '10px 0',
            border: 'none',
            background: value === o.value ? 'var(--accent-soft)' : 'transparent',
            color: value === o.value ? 'var(--accent)' : 'var(--ink-2)',
            fontSize: 14,
          }}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}
