import http from './http'
import type { ApiResponse, LoginResponse, PageData, User, NpmPackage, NpmVersion } from '../types'

// 认证
export const authApi = {
  login: (username: string, password: string) =>
    http.post<LoginResponse>('/auth/login', { username, password }),
  me: () => http.get<ApiResponse<User>>('/auth/me'),
  changePassword: (oldPassword: string, newPassword: string) =>
    http.put<ApiResponse<null>>('/auth/password', { old_password: oldPassword, new_password: newPassword }),
}

// 用户管理
export const userApi = {
  list: (page = 1, page_size = 20) =>
    http.get<ApiResponse<PageData<User>>>('/admin/users', { params: { page, page_size } }),
  get: (id: number) => http.get<ApiResponse<User>>(`/admin/users/${id}`),
  create: (data: { username: string; password: string; email?: string; is_admin?: boolean }) =>
    http.post<ApiResponse<User>>('/admin/users', data),
  update: (id: number, data: Partial<User> & { password?: string }) =>
    http.put<ApiResponse<null>>(`/admin/users/${id}`, data),
  resetPassword: (id: number, password: string) =>
    http.put<ApiResponse<null>>(`/admin/users/${id}/password`, { password }),
  delete: (id: number) => http.delete<ApiResponse<null>>(`/admin/users/${id}`),
}

// 系统设置
export const settingApi = {
  getAll: () => http.get<ApiResponse<Record<string, string>>>('/admin/settings'),
  update: (data: Record<string, string>) => http.put<ApiResponse<null>>('/admin/settings', data),
}

// npm 仓库
export const npmApi = {
  listPackages: (page = 1, pageSize = 20, search = '') =>
    http.get<ApiResponse<NpmPackage[]>>('/npm/packages', { params: { page, page_size: pageSize, search: search || undefined } }),
  listVersions: (name: string) => {
    if (name.startsWith('@')) {
      const [scope, pkg] = name.slice(1).split('/')
      return http.get<ApiResponse<NpmVersion[]>>(`/npm/packages/${pkg}`, { params: { scope } })
    }
    return http.get<ApiResponse<NpmVersion[]>>(`/npm/packages/${name}`)
  },
}
