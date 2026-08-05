export interface LoginRequest {
  email: string
  password: string
}

export interface RegisterRequest {
  email: string
  password: string
  full_name: string
}

export interface LoginResponse {
  access_token: string
  token_type: string
  expires_in: number
}

export interface RegisterResponse {
  user_id: string
  email: string
  full_name: string
  roles: string[]
}

export interface ApiError {
  code: string
  message: string
  trace_id: string
}

export interface HealthResponse {
  status: string
  components?: Record<string, unknown>
}
