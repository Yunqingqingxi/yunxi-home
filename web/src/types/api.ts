export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

export interface ApiError extends Error {
  errorCode?: string
  serverMessage?: string
}

export interface PaginatedData<T> {
  list: T[]
  total: number
  page: number
  pageSize: number
}

export interface SessionListResponse {
  id: string
  title: string
  created_at: string
  updated_at: string
  message_count: number
}

export interface SessionMessagesResponse {
  messages: any[]
  session_id: string
}
