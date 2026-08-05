import axios, { AxiosError } from "axios"
import type { ApiError, LoginRequest, LoginResponse, RegisterRequest, RegisterResponse } from "./types"
import { getAccessToken } from "./token-storage"

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8000/api/v1"

export const http = axios.create({
  baseURL: BASE_URL,
  headers: { "Content-Type": "application/json" },
})

http.interceptors.request.use((config) => {
  const token = getAccessToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
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
