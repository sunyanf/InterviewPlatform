import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { jobApi } from '../api/endpoints'

export default function JobsPage() {
  const navigate = useNavigate()
  const [category, setCategory] = useState<string>('')

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
      <header className="rise" style={{ marginBottom: 26 }}>
        <span className="mono dim" style={{ fontSize: 11.5, letterSpacing: '0.3em' }}>
          PICK YOUR BATTLE
        </span>
        <h1 style={{ fontSize: 32, marginTop: 8 }}>选择目标岗位</h1>
        <p className="muted" style={{ marginTop: 6 }}>
          考官将依据岗位要求与你的简历生成问题。共 {jobList?.total ?? '—'} 个在招岗位。
        </p>
      </header>

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
              {job.skills.slice(0, 5).map((s) => (
                <span key={s} className="tag">
                  {s}
                </span>
              ))}
              {job.skills.length > 5 && <span className="tag">+{job.skills.length - 5}</span>}
            </div>
            {job.description && (
              <p className="dim" style={{ fontSize: 13, lineHeight: 1.6, flex: 1 }}>
                {job.description.length > 72 ? `${job.description.slice(0, 72)}…` : job.description}
              </p>
            )}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span className="dim mono" style={{ fontSize: 11.5 }}>
                {job.requirements.length} 项要求
              </span>
              <span style={{ color: 'var(--accent)', fontSize: 13 }}>开始准备 →</span>
            </div>
          </article>
        ))}
      </div>

      {!isLoading && jobList && jobList.list.length === 0 && (
        <div className="card" style={{ padding: 40, textAlign: 'center' }}>
          <p className="muted">该分类下暂无岗位，试试其他分类。</p>
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
