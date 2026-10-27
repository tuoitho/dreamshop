import axios, { AxiosError, type InternalAxiosRequestConfig } from "axios"
import { toast } from "sonner"
import type {
  AccountStatus,
  AdminInventoryPage,
  AdminProductDetail,
  AdminProductMutationPayload,
  AdminProductsPage,
  AdminUser,
  AdminUserPage,
  ApiError,
  ApiResponse,
  CurrentUser,
  ListAdminInventoryParams,
  ListAdminProductsParams,
  ListAdminUsersParams,
  LoginRequest,
  LoginResponse,
  MediaPresignData,
  MediaPresignRequest,
  RbacItem,
  RbacItemType,
  StockMutationResult,
} from "./types"
import {
  clearAccessToken,
  getAccessToken,
  getSessionEpoch,
  runSingleFlightRefresh,
  setAccessToken,
} from "./auth-session"

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8000/api/v1"

export const http = axios.create({
  baseURL: BASE_URL,
  headers: { "Content-Type": "application/json" },
  withCredentials: true,
})

export const refreshHttp = axios.create({
  baseURL: BASE_URL,
  headers: { "Content-Type": "application/json" },
  withCredentials: true,
})

function isMutation(config: { method?: string } | undefined): boolean {
  return Boolean(config?.method && config.method.toLowerCase() !== "get")
}

function shouldSilentMutationToast(url?: string): boolean {
  if (!url) return false
  return url.startsWith("/auth/") || url.endsWith("/admin/products/media/presign")
}

function responseMessage(data: unknown): string | undefined {
  return data && typeof data === "object" && "message" in data && typeof data.message === "string"
    ? data.message
    : undefined
}

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

http.interceptors.response.use(
  (response) => {
    if (isMutation(response.config) && !shouldSilentMutationToast(response.config.url)) {
      const message = responseMessage(response.data)
      if (message) toast.success(message)
    }
    return response
  },
  async (error: AxiosError) => {
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
      if (isMutation(config) && !shouldSilentMutationToast(url)) {
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
      if (isMutation(config) && !shouldSilentMutationToast(url)) {
        toast.error(responseMessage(error.response?.data) ?? error.message)
      }
      return Promise.reject(error)
    }
  }
)

export class ApiClientError extends Error {
  readonly status: number | null
  readonly code?: string
  readonly traceId?: string
  readonly errors?: Record<string, string>

  constructor(
    status: number | null,
    message: string,
    code?: string,
    traceId?: string,
    errors?: Record<string, string>
  ) {
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
      return new ApiClientError(
        error.response?.status ?? null,
        body.message,
        body.code,
        body.traceId,
        body.errors
      )
    }
    return new ApiClientError(error.response?.status ?? null, error.message)
  }
  return new ApiClientError(null, error instanceof Error ? error.message : "Đã xảy ra lỗi không xác định")
}

// --- Auth & Operator Session ---

export async function login(payload: LoginRequest): Promise<LoginResponse> {
  try {
    const { data } = await http.post<ApiResponse<LoginResponse>>("/auth/login", payload)
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

export async function fetchCurrentUser(): Promise<CurrentUser> {
  try {
    const { data } = await http.get<CurrentUser>("/users/me")
    return data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function fetchMyPermissions(): Promise<string[]> {
  try {
    const { data } = await http.get<ApiResponse<string[]>>("/users/me/permissions")
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

// --- Admin Products & Media ---

export async function listAdminProducts(
  params: ListAdminProductsParams = {},
  signal?: AbortSignal
): Promise<AdminProductsPage> {
  try {
    const { data } = await http.get<ApiResponse<AdminProductsPage>>("/admin/products", {
      params,
      signal,
    })
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function getAdminProduct(id: string, signal?: AbortSignal): Promise<AdminProductDetail> {
  try {
    const { data } = await http.get<ApiResponse<AdminProductDetail>>(
      `/admin/products/${encodeURIComponent(id)}`,
      { signal }
    )
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function createAdminProduct(
  payload: AdminProductMutationPayload
): Promise<AdminProductDetail> {
  try {
    const { data } = await http.post<ApiResponse<AdminProductDetail>>("/admin/products", payload)
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function updateAdminProduct(
  id: string,
  payload: AdminProductMutationPayload
): Promise<AdminProductDetail> {
  try {
    const { data } = await http.put<ApiResponse<AdminProductDetail>>(
      `/admin/products/${encodeURIComponent(id)}`,
      payload
    )
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function toggleAdminProductDeleted(
  id: string,
  deleted: boolean
): Promise<{ id: string; deleted: boolean }> {
  try {
    const { data } = await http.post<ApiResponse<{ id: string; deleted: boolean }>>(
      `/admin/products/${encodeURIComponent(id)}/delete`,
      { deleted }
    )
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function presignProductMediaUpload(
  payload: MediaPresignRequest
): Promise<MediaPresignData> {
  try {
    const { data } = await http.post<ApiResponse<MediaPresignData>>(
      "/admin/products/media/presign",
      payload
    )
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

// --- Admin Inventory ---

export async function listAdminInventory(
  params: ListAdminInventoryParams = {},
  signal?: AbortSignal
): Promise<AdminInventoryPage> {
  try {
    const { data } = await http.get<ApiResponse<AdminInventoryPage>>("/admin/inventory", {
      params,
      signal,
    })
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function setProductStock(
  productId: string,
  quantity: number
): Promise<StockMutationResult> {
  try {
    const { data } = await http.put<ApiResponse<StockMutationResult>>(
      `/admin/inventory/${encodeURIComponent(productId)}`,
      { quantity }
    )
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function adjustProductStock(
  productId: string,
  delta: number
): Promise<StockMutationResult> {
  try {
    const { data } = await http.post<ApiResponse<StockMutationResult>>(
      `/admin/inventory/${encodeURIComponent(productId)}/adjust`,
      { delta }
    )
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

// --- Admin Users ---

export async function listAdminUsers(
  params: ListAdminUsersParams = {},
  signal?: AbortSignal
): Promise<AdminUserPage> {
  try {
    const { data } = await http.get<ApiResponse<AdminUserPage>>("/admin/users", {
      params,
      signal,
    })
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function getAdminUser(userId: string, signal?: AbortSignal): Promise<AdminUser> {
  try {
    const { data } = await http.get<ApiResponse<AdminUser>>(
      `/admin/users/${encodeURIComponent(userId)}`,
      { signal }
    )
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function updateAdminUserStatus(
  userId: string,
  status: AccountStatus
): Promise<AdminUser> {
  try {
    const { data } = await http.patch<ApiResponse<AdminUser>>(
      `/admin/users/${encodeURIComponent(userId)}/status`,
      { status }
    )
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function assignAdminUserRole(
  userId: string,
  roleName: string
): Promise<AdminUser> {
  try {
    const { data } = await http.put<ApiResponse<AdminUser>>(
      `/admin/users/${encodeURIComponent(userId)}/roles/${encodeURIComponent(roleName)}`
    )
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function revokeAdminUserRole(
  userId: string,
  roleName: string
): Promise<AdminUser> {
  try {
    const { data } = await http.delete<ApiResponse<AdminUser>>(
      `/admin/users/${encodeURIComponent(userId)}/roles/${encodeURIComponent(roleName)}`
    )
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

// --- Admin RBAC ---

export async function listRbacItems(
  type?: RbacItemType,
  signal?: AbortSignal
): Promise<RbacItem[]> {
  try {
    const { data } = await http.get<ApiResponse<RbacItem[]>>("/admin/rbac/items", {
      params: type ? { type } : undefined,
      signal,
    })
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function createRbacItem(payload: {
  name: string
  type: RbacItemType
  description?: string
}): Promise<RbacItem> {
  try {
    const { data } = await http.post<ApiResponse<RbacItem>>("/admin/rbac/items", payload)
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function updateRbacItem(
  name: string,
  payload: { name: string; description?: string }
): Promise<RbacItem> {
  try {
    const { data } = await http.patch<ApiResponse<RbacItem>>(
      `/admin/rbac/items/${encodeURIComponent(name)}`,
      payload
    )
    return data.data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function deleteRbacItem(name: string): Promise<void> {
  try {
    await http.delete(`/admin/rbac/items/${encodeURIComponent(name)}`)
  } catch (error) {
    throw toClientError(error)
  }
}

export async function addRbacChild(parent: string, child: string): Promise<void> {
  try {
    await http.put(
      `/admin/rbac/items/${encodeURIComponent(parent)}/children/${encodeURIComponent(child)}`
    )
  } catch (error) {
    throw toClientError(error)
  }
}

export async function removeRbacChild(parent: string, child: string): Promise<void> {
  try {
    await http.delete(
      `/admin/rbac/items/${encodeURIComponent(parent)}/children/${encodeURIComponent(child)}`
    )
  } catch (error) {
    throw toClientError(error)
  }
}
