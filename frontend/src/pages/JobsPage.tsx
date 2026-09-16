import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { jobApi } from '../api/endpoints'
import { ApiError } from '../api/client'

// 把逗号/顿号/换行/分号分隔的文本拆成非空条目
function splitItems(raw: string): string[] {
  return raw
    .split(/[\n,，、；;]+/)
    .map((s) => s.trim())
    .filter(Boolean)
}

export default function JobsPage() {
  const navigate = useNavigate()
  const [category, setCategory] = useState<string>('')
  const [showCreate, setShowCreate] = useState(false)

  const { data: categories } = useQuery({
    queryKey: ['job-categories'],
    queryFn: jobApi.categories,
  })
  const { data: jobList, isLoading } = useQuery({
    queryKey: ['jobs', category],
    queryFn: () => jobApi.list({ category_id: category || undefined, page_size: 50 }),
  })

  return (
    <main className="shell" style={{ paddingTop: 40, paddingBottom: 60 }}>
      <header
        className="rise"
        style={{ marginBottom: 26, display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap' }}
      >
        <div>
          <span className="mono dim" style={{ fontSize: 11.5, letterSpacing: '0.3em' }}>
            PICK YOUR BATTLE
          </span>
          <h1 style={{ fontSize: 32, marginTop: 8 }}>选择目标岗位</h1>
          <p className="muted" style={{ marginTop: 6 }}>
            考官将依据岗位要求与你的简历生成问题。共 {jobList?.total ?? '—'} 个在招岗位。
          </p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowCreate((v) => !v)}>
          {showCreate ? '收起' : '＋ 自定义岗位（粘贴 JD）'}
        </button>
      </header>

      {showCreate && (
        <CreateJobCard
          defaultCategory={category}
          categories={categories ?? []}
          onCancel={() => setShowCreate(false)}
          onCreated={(id) => navigate(`/prepare/${id}`)}
        />
      )}

      {/* 分类筛选 */}
      <div className="rise rise-1" style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 26 }}>
        <CategoryChip active={category === ''} onClick={() => setCategory('')}>
          全部
        </CategoryChip>
        {(categories ?? []).map((c) => (
          <CategoryChip key={c.id} active={category === c.id} onClick={() => setCategory(c.id)}>
            {c.name}
          </CategoryChip>
        ))}
      </div>

      {isLoading && <div className="spinner" />}

      <div
        className="rise rise-2"
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
          gap: 16,
        }}
      >
        {(jobList?.list ?? []).map((job) => (
          <article
            key={job.id}
            className="card"
            onClick={() => navigate(`/prepare/${job.id}`)}
            style={{
              padding: '22px 22px 20px',
              cursor: 'pointer',
              display: 'flex',
              flexDirection: 'column',
              gap: 12,
              minHeight: 188,
              transition: 'transform .2s var(--ease), border-color .2s var(--ease)',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.transform = 'translateY(-3px)'
              e.currentTarget.style.borderColor = 'rgba(217,164,65,.4)'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.transform = 'none'
              e.currentTarget.style.borderColor = 'var(--line)'
            }}
          >
            <h2 style={{ fontSize: 19 }}>{job.title}</h2>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {(job.skills ?? []).slice(0, 5).map((s) => (
                <span key={s} className="tag">
                  {s}
                </span>
              ))}
              {(job.skills?.length ?? 0) > 5 && <span className="tag">+{job.skills!.length - 5}</span>}
            </div>
            {job.description && (
              <p className="dim" style={{ fontSize: 13, lineHeight: 1.6, flex: 1 }}>
                {job.description.length > 72 ? `${job.description.slice(0, 72)}…` : job.description}
              </p>
            )}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span className="dim mono" style={{ fontSize: 11.5 }}>
                {job.requirements?.length ?? 0} 项要求
              </span>
              <span style={{ color: 'var(--accent)', fontSize: 13 }}>开始准备 →</span>
            </div>
          </article>
        ))}
      </div>

      {!isLoading && jobList && (jobList.list?.length ?? 0) === 0 && (
        <div className="card" style={{ padding: 40, textAlign: 'center' }}>
          <p className="muted" style={{ marginBottom: 16 }}>
            该分类下暂无在招岗位。可以粘贴招聘 JD，创建一个只属于你的练习岗位。
          </p>
          <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
            ＋ 自定义岗位（粘贴 JD）
          </button>
        </div>
      )}
    </main>
  )
}

function CategoryChip({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className="btn"
      style={{
        padding: '7px 16px',
        fontSize: 13.5,
        borderRadius: 999,
        border: `1px solid ${active ? 'rgba(217,164,65,.5)' : 'var(--line-strong)'}`,
        background: active ? 'var(--accent-soft)' : 'transparent',
        color: active ? 'var(--accent)' : 'var(--ink-2)',
      }}
    >
      {children}
    </button>
  )
}

// 自定义岗位：粘贴招聘 JD/HC 要求，创建后直接进入该岗位的面试准备页
function CreateJobCard({
  defaultCategory,
  categories,
  onCancel,
  onCreated,
}: {
  defaultCategory: string
  categories: { id: string; name: string }[]
  onCancel: () => void
  onCreated: (jobId: string) => void
}) {
  const [categoryId, setCategoryId] = useState(defaultCategory || categories[0]?.id || '')
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [requirements, setRequirements] = useState('')
  const [skills, setSkills] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const submit = async () => {
    if (!title.trim()) {
      setError('请填写岗位名称，例如「高级 Go 后端工程师」')
      return
    }
    if (!categoryId) {
      setError('请选择岗位分类')
      return
    }
    setSaving(true)
    setError('')
    try {
      const job = await jobApi.create({
        category_id: categoryId,
        title: title.trim(),
        description: description.trim(),
        requirements: splitItems(requirements),
        skills: splitItems(skills),
        source: 'custom',
      })
      onCreated(job.id)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '创建失败，请重试')
      setSaving(false)
    }
  }

  return (
    <section className="card" style={{ padding: 24, marginBottom: 24, border: '1px solid rgba(217,164,65,.4)' }}>
      <h2 style={{ fontSize: 18, marginBottom: 4 }}>自定义岗位</h2>
      <p className="dim" style={{ fontSize: 13, marginBottom: 18 }}>
        粘贴目标公司的招聘 JD 或 HC 要求，AI 考官会严格围绕该岗位的职责与要求出题。
      </p>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }} className="prepare-grid">
        <div className="field">
          <label>岗位名称 *</label>
          <input
            className="input"
            placeholder="例如：高级 Go 后端工程师"
            value={title}
            maxLength={100}
            onChange={(e) => setTitle(e.target.value)}
          />
        </div>
        <div className="field">
          <label>岗位分类 *</label>
          <select className="input" value={categoryId} onChange={(e) => setCategoryId(e.target.value)}>
            {categories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="field" style={{ marginTop: 16 }}>
        <label>岗位职责 / JD 原文（粘贴整段招聘描述即可）</label>
        <textarea
          className="input"
          style={{ minHeight: 120, fontSize: 13.5, lineHeight: 1.7 }}
          placeholder={'粘贴岗位描述，例如：\n1. 负责核心交易系统的设计与开发；\n2. 保障高并发服务的稳定性与性能；\n3. 参与系统容量规划与技术选型……'}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginTop: 16 }} className="prepare-grid">
        <div className="field">
          <label>任职要求（每行一条）</label>
          <textarea
            className="input"
            style={{ minHeight: 96, fontSize: 13.5, lineHeight: 1.7 }}
            placeholder={'5 年以上 Go 开发经验\n熟悉 MySQL、Redis\n有高并发系统设计经验'}
            value={requirements}
            onChange={(e) => setRequirements(e.target.value)}
          />
        </div>
        <div className="field">
          <label>关键技能（逗号或换行分隔）</label>
          <textarea
            className="input"
            style={{ minHeight: 96, fontSize: 13.5, lineHeight: 1.7 }}
            placeholder={'Go, MySQL, Redis, Kafka, 微服务'}
            value={skills}
            onChange={(e) => setSkills(e.target.value)}
          />
        </div>
      </div>

      {error && (
        <div className="notice notice-error" style={{ marginTop: 14 }}>
          {error}
        </div>
      )}

      <div style={{ display: 'flex', gap: 12, marginTop: 18 }}>
        <button className="btn btn-primary" onClick={submit} disabled={saving} style={{ minWidth: 160 }}>
          {saving ? (
            <>
              <span className="spinner" /> 创建中…
            </>
          ) : (
            '创建并去准备面试'
          )}
        </button>
        <button className="btn btn-ghost" onClick={onCancel} disabled={saving}>
          取消
        </button>
      </div>
    </section>
  )
}
