import { api } from './client'
import type { AdminUserList, Setting } from './types'

export const adminApi = {
  getSettings: () => api.get<Setting[]>('/admin/settings'),
  updateSettings: (body: Record<string, string>) =>
    api.patch<Setting[]>('/admin/settings', body),
  listUsers: (page = 1, pageSize = 20) =>
    api.get<AdminUserList>(`/admin/users?page=${page}&page_size=${pageSize}`),
  updateUser: (id: string, body: { status: string }) =>
    api.patch<{ id: string; status: string }>(`/admin/users/${encodeURIComponent(id)}`, body),
}
