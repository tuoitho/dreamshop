import axios, { AxiosError, type InternalAxiosRequestConfig } from "axios"
import { toast } from "sonner"
import type { ApiError, ApiResponse, CartData, CurrentUser, LoginRequest, LoginResponse, ProductDetail, RegisterRequest, RegisterResponse, SearchProductsParams, SearchProductsResult, UpdateProfileRequest } from "./types"
import { clearAccessToken, getAccessToken, getSessionEpoch, runSingleFlightRefresh, setAccessToken } from "./auth-session"

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8000/api/v1"

export const http = axios.create({
  baseURL: BASE_URL,
  headers: { "Content-Type": "application/json" },
  withCredentials: true,
})

function isMutation(config: { method?: string } | undefined): boolean {
  return Boolean(config?.method && config.method.toLowerCase() !== "get")
}

function responseMessage(data: unknown): string | undefined {
  return data && typeof data === "object" && "message" in data && typeof data.message === "string"
    ? data.message
    : undefined
}

function shouldSuppressMutationSuccessToast(response: {
  config?: { method?: string; url?: string; data?: unknown }
  data?: unknown
}): boolean {
  const method = response.config?.method?.toLowerCase()
  const url = response.config?.url ?? ""
  if (method !== "post" || !url.endsWith("/cart/items")) {
    return false
  }
  const envelope = response.data as Partial<ApiResponse<CartData>> | undefined
  const cart = envelope?.data
  if (!cart || cart.degraded !== false || !Array.isArray(cart.items)) {
    return false
  }
  let reqBody: { productId?: string; variantLabel?: string } | undefined
  const rawData = response.config?.data
  if (typeof rawData === "string") {
    try {
      reqBody = JSON.parse(rawData) as { productId?: string; variantLabel?: string }
    } catch {
      return false
    }
  } else if (rawData && typeof rawData === "object") {
    reqBody = rawData as { productId?: string; variantLabel?: string }
  }
  if (!reqBody?.productId) {
    return false
  }
  const variantLabel = reqBody.variantLabel ?? ""
  const target = cart.items.find(
    (item) => item.productId === reqBody.productId && item.variantLabel === variantLabel
  )
  return target?.status === "OUT_OF_STOCK" || target?.status === "UNAVAILABLE"
}

export const refreshHttp = axios.create({
  baseURL: BASE_URL,
  headers: { "Content-Type": "application/json" },
  withCredentials: true,
})

export async function refreshAccessToken(): Promise<string> {
  return runSingleFlightRefresh(async () => {
    const startEpoch = getSessionEpoch()
    const { data } = await refreshHttp.post<ApiResponse<LoginResponse>>("/auth/refresh")
    if (getSessionEpoch() !== startEpoch) {
      throw new Error("Session changed during token refresh")
    }
    setAccessToken(data.data.accessToken)
    return data.data.accessToken
  })
}

type SessionAwareRequestConfig = InternalAxiosRequestConfig & {
  _retried?: boolean
  _sessionEpoch?: number
}

http.interceptors.request.use((config) => {
  ;(config as SessionAwareRequestConfig)._sessionEpoch = getSessionEpoch()
  const token = getAccessToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use((response) => {
  if (isMutation(response.config) && !shouldSuppressMutationSuccessToast(response)) {
    const message = responseMessage(response.data)
    if (message) toast.success(message)
  }
  return response
}, async (error: AxiosError) => {
  const config = error.config as SessionAwareRequestConfig | undefined
  const url = config?.url ?? ""
  const requestEpoch = config?._sessionEpoch ?? getSessionEpoch()
  if (
    error.response?.status !== 401 ||
    url.startsWith("/auth/") ||
    !config ||
    config._retried ||
    requestEpoch !== getSessionEpoch()
  ) {
    if (isMutation(config)) {
      toast.error(responseMessage(error.response?.data) ?? error.message)
    }
    return Promise.reject(error)
  }
  config._retried = true
  try {
    const token = await refreshAccessToken()
    if (getSessionEpoch() !== requestEpoch) {
      return Promise.reject(error)
    }
    config.headers.Authorization = `Bearer ${token}`
    return http(config)
  } catch {
    if (getSessionEpoch() === requestEpoch) {
      clearAccessToken()
    }
    if (isMutation(config)) {
      toast.error(responseMessage(error.response?.data) ?? error.message)
    }
    return Promise.reject(error)
  }
})

export class ApiClientError extends Error {
  readonly status: number | null
  readonly code?: string
  readonly traceId?: string
  readonly errors?: Record<string, string>

  constructor(status: number | null, message: string, code?: string, traceId?: string, errors?: Record<string, string>) {
    super(message)
    this.name = "ApiClientError"
    this.status = status
    this.code = code
    this.traceId = traceId
    this.errors = errors
  }
}

export function toClientError(error: unknown): ApiClientError {
  if (error instanceof ApiClientError) return error
  if (error instanceof AxiosError) {
    const body = error.response?.data as Partial<ApiError> | undefined
    if (body && typeof body.message === "string") {
      return new ApiClientError(error.response?.status ?? null, body.message, body.code, body.traceId, body.errors)
    }
    return new ApiClientError(error.response?.status ?? null, error.message)
  }
  return new ApiClientError(null, error instanceof Error ? error.message : "Unexpected error")
}

export function toErrorInfo(error: unknown): { message: string; code?: string } {
  const clientError = toClientError(error)
  return { message: clientError.message, code: clientError.code }
}

function toStorefrontQuery(params: SearchProductsParams): Record<string, string | number> {
  const query: Record<string, string | number> = {}
  if (params.q) query.q = params.q
  if (params.categoryId) query["danh-muc"] = params.categoryId
  if (params.priceMin !== undefined || params.priceMax !== undefined) {
    query.gia = `${params.priceMin ?? 0}-${params.priceMax ?? 0}`
  }
  if (params.onSale) query["giam-gia"] = 1
  if (params.ratingMin) query["danh-gia"] = params.ratingMin
  if (params.sort) query["sap-xep"] = params.sort
  if (params.page !== undefined) query.page = params.page
  if (params.pageSize !== undefined) query.pageSize = params.pageSize
  return query
}

export async function searchProducts(
  params: SearchProductsParams,
  signal?: AbortSignal
): Promise<SearchProductsResult> {
  try {
    const { data } = await http.get<ApiResponse<SearchProductsResult>>("/storefront/products", {
      params: toStorefrontQuery(params),
      signal,
    })
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function getProduct(id: string, signal?: AbortSignal): Promise<ProductDetail> {
  try {
    const { data } = await http.get<ApiResponse<ProductDetail>>(`/storefront/products/${id}`, { signal })
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function login(payload: LoginRequest): Promise<LoginResponse> {
  try {
    const { data } = await http.post<ApiResponse<LoginResponse>>("/auth/login", payload)
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function register(payload: RegisterRequest): Promise<RegisterResponse> {
  try {
    const { data } = await http.post<ApiResponse<RegisterResponse>>("/auth/register", payload)
    return data.data
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
    const { data } = await http.patch<ApiResponse<CurrentUser>>("/users/me", payload)
    return data.data
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

export async function getCart(): Promise<CartData> {
  try {
    const { data } = await http.get<ApiResponse<CartData>>("/cart")
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function addCartItem(payload: { productId: string; variantLabel: string; quantity: number }): Promise<CartData> {
  try {
    const { data } = await http.post<ApiResponse<CartData>>("/cart/items", payload)
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function updateCartItemQuantity(itemId: string, quantity: number): Promise<CartData> {
  try {
    const { data } = await http.patch<ApiResponse<CartData>>(`/cart/items/${itemId}`, { quantity })
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function removeCartItem(itemId: string): Promise<CartData> {
  try {
    const { data } = await http.delete<ApiResponse<CartData>>(`/cart/items/${itemId}`)
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function clearCart(): Promise<CartData> {
  try {
    const { data } = await http.delete<ApiResponse<CartData>>("/cart")
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function softDeleteProduct(id: string, deleted: boolean): Promise<{ id: string; deleted: boolean }> {
  try {
    const { data } = await http.post<ApiResponse<{ id: string; deleted: boolean }>>(`/admin/products/${id}/delete`, { deleted })
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}
