export interface ApiResponse<T> {
  data: T
  message: string
}

export interface LoginRequest {
  email: string
  password: string
}

export interface RegisterRequest {
  email: string
  password: string
  fullName: string
}

export interface LoginResponse {
  accessToken: string
  tokenType: string
  expiresIn: number
}

export interface RegisterResponse {
  userId: string
  email: string
  fullName: string
  roles: string[]
}

export type Gender = "MALE" | "FEMALE" | "OTHER"

export interface LocationOption {
  code: string
  name: string
}

export interface CurrentUser {
  userId: string
  email: string
  fullName: string
  roles: string[]
  phone: string | null
  dateOfBirth: string | null
  gender: Gender | null
  provinceCode: string | null
  wardCode: string | null
  addressLine: string | null
}

export interface UpdateProfileRequest {
  fullName?: string
  phone?: string
  dateOfBirth?: string
  gender?: Gender | ""
  provinceCode?: string
  wardCode?: string
  addressLine?: string
}

export interface ApiError {
  code: string
  message: string
  errors?: Record<string, string>
  traceId: string
}

export interface HealthResponse {
  status: string
  components?: Record<string, unknown>
}
