import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '../api/admin'
import type { Setting } from '../api/types'

type Tab = 'settings' | 'users' | 'jobs'

const TAB_META: Record<Tab, { label: string; desc: string }> = {
  settings: {
    label: '系统配置',
    desc: '配置 AI 模型接入、接口限流与后台任务。修改直接影响全站出题、分析与评估能力，普通用户看不到此页面。',
  },
  users: {
    label: '用户管理',
    desc: '查看全部注册用户，可对违规或异常账号执行禁用 / 启用。被禁用账号将无法登录。',
  },
  jobs: {
    label: '岗位管理',
    desc: '岗位由用户在「选择岗位」页自助创建（可粘贴招聘 JD）。此处仅作说明，暂不提供集中编辑。',
  },
}

export default function AdminPage() {
  const [tab, setTab] = useState<Tab>('settings')
  return (
    <div className="shell" style={{ paddingTop: 40, maxWidth: 960 }}>
      <h1 style={{ fontSize: 24, marginBottom: 6 }}>管理后台</h1>
      <p className="dim" style={{ fontSize: 13, marginBottom: 22 }}>
        仅管理员可见，用于维护平台运行参数、用户账号与岗位数据。
      </p>
      <div style={{ display: 'flex', gap: 8, marginBottom: 12 }}>
        {(['settings', 'users', 'jobs'] as Tab[]).map((t) => (
          <button
            key={t}
            className={`btn ${tab === t ? 'btn-primary' : 'btn-ghost'}`}
            style={{ fontSize: 14 }}
            onClick={() => setTab(t)}
          >
            {TAB_META[t].label}
          </button>
        ))}
      </div>
      <p className="dim" style={{ fontSize: 13, marginBottom: 22, lineHeight: 1.7 }}>
        {TAB_META[tab].desc}
      </p>
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

  const groups: { label: string; desc: string; prefix: string }[] = [
    {
      label: 'AI 模型（LLM）',
      desc: '面试出题、回答分析与评分总结所用的大模型。留空则使用服务端默认值，切换模型后即时生效。',
      prefix: 'llm.',
    },
    {
      label: '接口限流',
      desc: '限制单位时间内的请求次数，防止刷接口。修改后需重启服务生效。',
      prefix: 'rate_limit.',
    },
    {
      label: '后台任务（Worker）',
      desc: '简历解析、AI 评分等异步任务的执行参数。修改后需重启服务生效。',
      prefix: 'task.',
    },
  ]

  return (
    <div>
      {groups.map((g) => (
        <fieldset key={g.prefix} style={{ marginBottom: 20, border: '1px solid #e0e0e0', borderRadius: 8, padding: 16 }}>
          <legend style={{ fontWeight: 600, fontSize: 15 }}>{g.label}</legend>
          <p className="dim" style={{ fontSize: 12.5, margin: '0 0 14px' }}>{g.desc}</p>
          {(settings as Setting[]).filter((s) => s.key.startsWith(g.prefix)).map((s) => {
            const meta = SETTING_META[s.key]
            return (
              <div key={s.key} style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 10 }}>
                <div style={{ width: 240, flexShrink: 0 }}>
                  <div style={{ fontSize: 13, color: '#333' }}>
                    {meta?.label ?? s.key}
                  </div>
                  <div className="mono dim" style={{ fontSize: 11 }}>{s.key}</div>
                </div>
                <input
                  type={s.key.includes('api_key') ? 'password' : 'text'}
                  className="input"
                  style={{ flex: 1, fontSize: 13 }}
                  placeholder={meta?.hint ? `${meta.hint}（当前：${s.value || '空'}）` : s.value || '(空)'}
                  value={draft[s.key] ?? ''}
                  onChange={(e) => setDraft({ ...draft, [s.key]: e.target.value })}
                />
              </div>
            )
          })}
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
      <p className="dim" style={{ marginTop: 16, fontSize: 12 }}>
        只保存本次被修改过的输入框；AI 模型配置即时热切换，限流与 Worker 参数需重启生效。
      </p>
    </div>
  )
}

// 系统配置项的中文名称与填写说明
const SETTING_META: Record<string, { label: string; hint?: string }> = {
  'llm.provider': { label: '模型厂商', hint: '如 openai / deepseek / 自定义兼容厂商名' },
  'llm.api_key': { label: 'API 密钥', hint: '模型服务的访问密钥，已加密掩码显示' },
  'llm.base_url': { label: '接口地址', hint: 'OpenAI 兼容接口的 Base URL' },
  'llm.model': { label: '模型名称', hint: '如 gpt-4o、deepseek-chat' },
  'llm.price_input_per_1k': { label: '输入价格（元 / 千 Token）', hint: '用于成本统计，可为空' },
  'llm.price_output_per_1k': { label: '输出价格（元 / 千 Token）', hint: '用于成本统计，可为空' },
  'rate_limit.enabled': { label: '是否启用限流', hint: 'true 启用 / false 关闭' },
  'rate_limit.auth_per_min': { label: '登录接口每分钟上限', hint: '每个 IP 每分钟最多登录次数' },
  'rate_limit.api_per_min': { label: '业务接口每分钟上限', hint: '每个用户每分钟最多请求次数' },
  'task.enabled': { label: '是否启用后台任务', hint: 'true 启用 / false 关闭' },
  'task.workers': { label: '并发执行数', hint: '同时处理的异步任务数量' },
  'task.poll_interval': { label: '轮询间隔（秒）', hint: '拉取新任务的间隔' },
  'task.lease_timeout': { label: '任务租约超时（秒）', hint: '超时未完成则重新派发' },
  'task.shutdown_timeout': { label: '优雅退出等待（秒）', hint: '服务重启时等待在途任务的最长时间' },
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

  const roleText = (r: string) => (r === 'admin' ? '管理员' : '普通用户')
  const statusText = (s: string) => (s === 'active' ? '正常' : '已禁用')

  return (
    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
      <thead>
        <tr style={{ borderBottom: '2px solid #eee', textAlign: 'left' }}>
          <th style={{ padding: 8 }}>邮箱（登录账号）</th>
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
            <td style={{ padding: 8 }}>{roleText(u.role)}</td>
            <td style={{ padding: 8, color: u.status === 'active' ? '#2e7d32' : '#c62828' }}>
              {statusText(u.status)}
            </td>
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
                {u.status === 'active' ? '禁用账号' : '启用账号'}
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
    <div className="card" style={{ padding: 24, lineHeight: 1.9 }}>
      <p style={{ fontWeight: 600, marginBottom: 8 }}>岗位从哪里来？</p>
      <p className="dim" style={{ fontSize: 13.5 }}>
        本平台不做统一岗位维护。任意登录用户都可在
        <a href="/jobs" style={{ color: 'var(--accent)', margin: '0 4px' }}>选择岗位</a>
        页点击「＋ 自定义岗位（粘贴 JD）」，粘贴目标公司招聘要求创建专属练习岗位，随后即可围绕该岗位配置题量、时长并开始面试。
      </p>
      <p className="dim" style={{ fontSize: 12.5, marginTop: 12 }}>
        说明：岗位为练习用途的用户自定义数据，不与真实招聘系统（HC / 招聘进度）打通。
      </p>
    </div>
  )
}
