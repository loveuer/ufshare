export interface User {
  id: number
  created_at: string
  updated_at: string
  username: string
  email: string
  is_admin: boolean
  status: number
}

export interface FileEntry {
  id: number
  created_at: string
  updated_at: string
  path: string
  size: number
  mime_type: string
  sha256: string
  uploader_id: number
  uploader: string
}

export interface NpmPackage {
  name: string
  description: string
  dist_tags: Record<string, string>
  version_count: number
  cached_count: number
}

export interface NpmVersion {
  version: string
  tarball_name: string
  size: number
  shasum: string
  cached: boolean
  uploader: string
  created_at: string
}

export interface LoginResponse {
  code: number
  message: string
  data: {
    token: string
    user: User
  }
}

export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

export interface PageData<T> {
  items: T[]
  total: number
  page: number
}

export interface GoCacheStats {
  cache_dir: string
  size_bytes: number
  file_count: number
  upstream: string
  goprivate: string
}
