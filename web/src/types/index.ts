export interface User {
  id: number
  created_at: string
  updated_at: string
  username: string
  email: string
  is_admin: boolean
  status: number
}

export interface Module {
  id: number
  created_at: string
  updated_at: string
  name: string
  type: string
  description: string
  public_read: boolean
  public_write: boolean
}

export interface Permission {
  id: number
  user_id: number
  module_id: number
  can_read: boolean
  can_write: boolean
  module?: Module
}

export interface FileEntry {
  id: number
  created_at: string
  updated_at: string
  module_id: number
  path: string
  size: number
  mime_type: string
  sha256: string
  uploader_id: number
  uploader: string
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
