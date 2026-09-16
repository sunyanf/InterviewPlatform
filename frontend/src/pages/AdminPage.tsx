import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '../api/admin'
import type { Setting } from '../api/types'

type Tab = 'settings' | 'users' | 'jobs'

export default function AdminPage() {
  const [tab, setTab] = useState<Tab>('settings')
  return (
    <div className="shell" style={{ paddingTop: 40, maxWidth: 960 }}>
      <h1 style={{ fontSize: 24, marginBottom: 24 }}>管理后台</h1>
      <div style={{ display: 'flex', gap: 8, marginBottom: 24 }}>
        {(['settings', 'users', 'jobs'] as Tab[]).map((t) => (
          <button
            key={t}
            className={`btn ${tab === t ? 'btn-primary' : 'btn-ghost'}`}
            style={{ fontSize: 14 }}
            onClick={() => setTab(t)}
          >
            {t === 'settings' ? '系统配置' : t === 'users' ? '用户管理' : '岗位管理'}
          </button>
        ))}
      </div>
      {tab === 'settings' && <SettingsTab />}
      {tab === 'users' && <UsersTab />}
      {tab === 'jobs' && <JobsTab />}
    </div>
  )
}

function SettingsTab() {
  const qc = useQueryClient()
  const { data: settings, isLoading } = useQuery({
    queryKey: ['admin-settings'],
    queryFn: adminApi.getSettings,
  })
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [saved, setSaved] = useState(false)

  const mutation = useMutation({
    mutationFn: adminApi.updateSettings,
    onSuccess: () => {
      setSaved(true)
      setDraft({})
      qc.invalidateQueries({ queryKey: ['admin-settings'] })
      setTimeout(() => setSaved(false), 2000)
    },
  })

  if (isLoading || !settings) return <div className="spinner" />

  const groups: { label: string; prefix: string }[] = [
    { label: 'LLM 模型', prefix: 'llm.' },
    { label: '限流', prefix: 'rate_limit.' },
    { label: '任务 Worker', prefix: 'task.' },
  ]

  return (
    <div>
      {groups.map((g) => (
        <fieldset key={g.prefix} style={{ marginBottom: 20, border: '1px solid #e0e0e0', borderRadius: 8, padding: 16 }}>
          <legend style={{ fontWeight: 600, fontSize: 15 }}>{g.label}</legend>
          {(settings as Setting[]).filter((s) => s.key.startsWith(g.prefix)).map((s) => (
            <div key={s.key} style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 10 }}>
              <label style={{ width: 200, fontSize: 13, color: '#666' }}>{s.key}</label>
              <input
                type={s.key.includes('api_key') ? 'password' : 'text'}
                className="input"
                style={{ flex: 1, fontSize: 13 }}
                placeholder={s.value || '(空)'}
                value={draft[s.key] ?? ''}
                onChange={(e) => setDraft({ ...draft, [s.key]: e.target.value })}
              />
            </div>
          ))}
        </fieldset>
      ))}
      <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
        <button
          className="btn btn-primary"
          disabled={Object.keys(draft).length === 0 || mutation.isPending}
          onClick={() => mutation.mutate(draft)}
        >
          {mutation.isPending ? '保存中…' : '保存配置'}
        </button>
        {saved && <span style={{ color: 'green', fontSize: 13 }}>已保存</span>}
        {mutation.isError && (
          <span style={{ color: 'red', fontSize: 13 }}>
            保存失败：{(mutation.error as Error)?.message}
          </span>
        )}
      </div>
      <p style={{ marginTop: 16, fontSize: 12, color: '#999' }}>
        LLM 配置保存后即时热切换；限流与 Worker 参数需重启生效。
      </p>
    </div>
  )
}

function UsersTab() {
  const [page] = useState(1)
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['admin-users', page],
    queryFn: () => adminApi.listUsers(page, 50),
  })

  const mutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) =>
      adminApi.updateUser(id, { status }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin-users'] }),
  })

  if (isLoading || !data) return <div className="spinner" />

  return (
    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
      <thead>
        <tr style={{ borderBottom: '2px solid #eee', textAlign: 'left' }}>
          <th style={{ padding: 8 }}>邮箱</th>
          <th style={{ padding: 8 }}>昵称</th>
          <th style={{ padding: 8 }}>角色</th>
          <th style={{ padding: 8 }}>状态</th>
          <th style={{ padding: 8 }}>注册时间</th>
          <th style={{ padding: 8 }}>操作</th>
        </tr>
      </thead>
      <tbody>
        {data.list.map((u) => (
          <tr key={u.id} style={{ borderBottom: '1px solid #f5f5f5' }}>
            <td style={{ padding: 8 }}>{u.email}</td>
            <td style={{ padding: 8 }}>{u.nickname}</td>
            <td style={{ padding: 8 }}>{u.role}</td>
            <td style={{ padding: 8 }}>{u.status}</td>
            <td style={{ padding: 8, color: '#999' }}>
              {new Date(u.created_at).toLocaleString('zh-CN')}
            </td>
            <td style={{ padding: 8 }}>
              <button
                className="btn btn-ghost"
                style={{ fontSize: 12, padding: '4px 10px' }}
                disabled={mutation.isPending}
                onClick={() =>
                  mutation.mutate({
                    id: u.id,
                    status: u.status === 'active' ? 'disabled' : 'active',
                  })
                }
              >
                {u.status === 'active' ? '禁用' : '启用'}
              </button>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function JobsTab() {
  return (
    <div style={{ padding: 20, color: '#666', textAlign: 'center' }}>
      <p>岗位管理复用 <a href="/jobs" style={{ color: '#4f8cf0' }}>选择岗位</a> 页面的创建功能。</p>
      <p style={{ fontSize: 12, color: '#999' }}>登录后任意用户可创建岗位；admin 可在岗位列表页直接操作。</p>
    </div>
  )
}
