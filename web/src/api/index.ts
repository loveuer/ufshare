import http from './http'
import type { ApiResponse, LoginResponse, PageData, User, Module, Permission } from '../types'

// 认证
export const authApi = {
  login: (username: string, password: string) =>
    http.post<LoginResponse>('/auth/login', { username, password }),
  register: (username: string, password: string, email: string) =>
    http.post<ApiResponse<User>>('/auth/register', { username, password, email }),
  me: () => http.get<ApiResponse<User>>('/auth/me'),
}

// 用户管理
export const userApi = {
  list: (page = 1, page_size = 20) =>
    http.get<ApiResponse<PageData<User>>>('/admin/users', { params: { page, page_size } }),
  get: (id: number) => http.get<ApiResponse<User>>(`/admin/users/${id}`),
  update: (id: number, data: Partial<User> & { password?: string }) =>
    http.put<ApiResponse<null>>(`/admin/users/${id}`, data),
  delete: (id: number) => http.delete<ApiResponse<null>>(`/admin/users/${id}`),
}

// 模块管理
export const moduleApi = {
  list: () => http.get<ApiResponse<Module[]>>('/admin/modules'),
  get: (id: number) => http.get<ApiResponse<Module>>(`/admin/modules/${id}`),
  create: (data: Partial<Module>) =>
    http.post<ApiResponse<Module>>('/admin/modules', data),
  update: (id: number, data: Partial<Module>) =>
    http.put<ApiResponse<null>>(`/admin/modules/${id}`, data),
  delete: (id: number) => http.delete<ApiResponse<null>>(`/admin/modules/${id}`),
}

// 权限管理
export const permissionApi = {
  getUserPermissions: (userId: number) =>
    http.get<ApiResponse<Permission[]>>(`/admin/permissions/user/${userId}`),
  grant: (user_id: number, module_id: number, can_read: boolean, can_write: boolean) =>
    http.post<ApiResponse<null>>('/admin/permissions/grant', { user_id, module_id, can_read, can_write }),
  revoke: (user_id: number, module_id: number) =>
    http.post<ApiResponse<null>>('/admin/permissions/revoke', { user_id, module_id }),
}
