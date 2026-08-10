import axios, { AxiosError, type InternalAxiosRequestConfig } from "axios"
import type { ApiError, CurrentUser, LoginRequest, LoginResponse, RegisterRequest, RegisterResponse, UpdateProfileRequest } from "./types"
import { clearAccessToken, getAccessToken, runSingleFlightRefresh, setAccessToken } from "./auth-session"

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8000/api/v1"

export const http = axios.create({
  baseURL: BASE_URL,
  headers: { "Content-Type": "application/json" },
  withCredentials: true,
})

const refreshHttp = axios.create({
  baseURL: BASE_URL,
  headers: { "Content-Type": "application/json" },
  withCredentials: true,
})

export async function refreshAccessToken(): Promise<string> {
  return runSingleFlightRefresh(async () => {
    const { data } = await refreshHttp.post<LoginResponse>("/auth/refresh")
    setAccessToken(data.access_token)
    return data.access_token
  })
}

http.interceptors.request.use((config) => {
  const token = getAccessToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use(undefined, async (error: AxiosError) => {
  const config = error.config as (InternalAxiosRequestConfig & { _retried?: boolean }) | undefined
  const url = config?.url ?? ""
  if (error.response?.status !== 401 || url.startsWith("/auth/") || !config || config._retried) {
    return Promise.reject(error)
  }
  config._retried = true
  try {
    const token = await refreshAccessToken()
    config.headers.Authorization = `Bearer ${token}`
    return http(config)
  } catch {
    clearAccessToken()
    return Promise.reject(error)
  }
})

export class ApiClientError extends Error {
  readonly status: number | null
  readonly code?: string
  readonly traceId?: string

  constructor(status: number | null, message: string, code?: string, traceId?: string) {
    super(message)
    this.name = "ApiClientError"
    this.status = status
    this.code = code
    this.traceId = traceId
  }
}

function toClientError(error: unknown): ApiClientError {
  if (error instanceof AxiosError) {
    const body = error.response?.data as Partial<ApiError> | undefined
    if (body && typeof body.message === "string") {
      return new ApiClientError(error.response?.status ?? null, body.message, body.code, body.trace_id)
    }
    return new ApiClientError(error.response?.status ?? null, error.message)
  }
  return new ApiClientError(null, error instanceof Error ? error.message : "Unexpected error")
}

export async function login(payload: LoginRequest): Promise<LoginResponse> {
  try {
    const { data } = await http.post<LoginResponse>("/auth/login", payload)
    return data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function register(payload: RegisterRequest): Promise<RegisterResponse> {
  try {
    const { data } = await http.post<RegisterResponse>("/auth/register", payload)
    return data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function fetchHealth(): Promise<{ status: string }> {
  try {
    const { data } = await http.get<{ status: string }>("/auth/actuator/health")
    return data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function fetchCurrentUser(): Promise<CurrentUser> {
  try {
    const { data } = await http.get<CurrentUser>("/users/me")
    return data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function updateCurrentUser(payload: UpdateProfileRequest): Promise<CurrentUser> {
  try {
    const { data } = await http.patch<CurrentUser>("/users/me", payload)
    return data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function logoutRequest(): Promise<void> {
  try {
    await http.post("/auth/logout")
  } catch (error) {
    throw toClientError(error)
  }
}
