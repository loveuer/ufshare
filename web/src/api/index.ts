import http from './http'
import type { ApiResponse, LoginResponse, PageData, User, NpmPackage, NpmVersion, GoCacheStats, OciRepository, OciTagInfo, OciCacheStats, MavenArtifact, MavenRepository, MavenRepositoryConfig } from '../types'

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

// Go 模块代理
export const goApi = {
  getStats: () => http.get<ApiResponse<GoCacheStats>>('/go/stats'),
  cleanCache: () => http.delete<ApiResponse<null>>('/go/cache'),
}

// OCI/Docker 镜像代理
export const ociApi = {
  listRepos: (page = 1, pageSize = 20, search = '') =>
    http.get<ApiResponse<OciRepository[]>>('/oci/repositories', { params: { page, page_size: pageSize, search: search || undefined } }),
  listTags: (name: string) =>
    http.get<ApiResponse<OciTagInfo[]>>('/oci/repositories/tags', { params: { name } }),
  deleteRepo: (id: number) => http.delete<ApiResponse<null>>(`/oci/repositories/${id}`),
  getStats: () => http.get<ApiResponse<OciCacheStats>>('/oci/stats'),
  cleanCache: () => http.delete<ApiResponse<null>>('/oci/cache'),
}

// Maven 仓库
export const mavenApi = {
  listArtifacts: (page = 1, pageSize = 20, groupId = '', artifactId = '') =>
    http.get<ApiResponse<MavenArtifact[]>>('/maven/artifacts', { params: { page, page_size: pageSize, group_id: groupId || undefined, artifact_id: artifactId || undefined } }),
  searchArtifacts: (q = '', page = 1, pageSize = 20) =>
    http.get<ApiResponse<MavenArtifact[]>>('/maven/artifacts/search', { params: { q, page, page_size: pageSize } }),
  getVersions: (groupId: string, artifactId: string) =>
    http.get<ApiResponse<string[]>>('/maven/artifacts/versions', { params: { group_id: groupId, artifact_id: artifactId } }),
  getArtifactDetail: (groupId: string, artifactId: string, version: string) =>
    http.get<ApiResponse<MavenArtifact>>('/maven/artifacts/detail', { params: { group_id: groupId, artifact_id: artifactId, version } }),
  listRepositories: () =>
    http.get<ApiResponse<MavenRepository[]>>('/maven/repositories'),
  addRepository: (data: MavenRepositoryConfig) =>
    http.post<ApiResponse<MavenRepository>>('/maven/repositories', data),
  updateRepository: (id: number, data: MavenRepositoryConfig) =>
    http.put<ApiResponse<null>>(`/maven/repositories/${id}`, data),
  deleteRepository: (id: number) =>
    http.delete<ApiResponse<null>>(`/maven/repositories/${id}`),
}
