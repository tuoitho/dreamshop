export interface ApiResponse<T> {
  data: T
  message: string
}

export interface ApiError {
  code: string
  message: string
  errors?: Record<string, string>
  traceId?: string
}

export interface LoginRequest {
  email: string
  password: string
}

export interface LoginResponse {
  accessToken: string
  tokenType: string
  expiresIn: number
}

export interface CurrentUser {
  userId: string
  email: string
  fullName: string
  phone?: string | null
  avatarUrl?: string | null
  roles: string[]
}

export interface AdminStockSnapshot {
  quantity: number
  reserved: number
  availableQuantity: number
  updatedAt?: string
}

export interface ProductVariant {
  skuId?: string
  label: string
  colorHex?: string
  price: number
  originalPrice?: number
  stock?: AdminStockSnapshot | null
}

export interface AdminProductVariantPayload {
  skuId?: string
  label: string
  colorHex?: string
  price: number
  originalPrice?: number
}

export interface AdminCategoryMetric {
  categoryId: string
  categoryName: string
  activeProductCount: number
}

export interface AdminProductSummary {
  id: string
  name: string
  brand: string
  categoryId: string
  categoryName: string
  price: number
  originalPrice: number
  maxPrice?: number
  images: string[]
  rating: number
  reviewCount: number
  soldCount: number
  isNew: boolean
  badge: string
  featured: boolean
  emoji: string
  imageHue: number[]
  deleted: boolean
  variantCount: number
  videoCount: number
  createdAt: string
  updatedAt: string
  stock: AdminStockSnapshot | null
}

export interface AdminProductsPage {
  items: AdminProductSummary[]
  page: number
  pageSize: number
  totalItems: number
  totalPages: number
  activeCount: number
  deletedCount: number
  categories: AdminCategoryMetric[]
  stockDegraded: boolean
}

export interface AdminProductDetail {
  id: string
  name: string
  shortDescription: string
  description: string
  brand: string
  categoryId: string
  categoryName: string
  price: number
  originalPrice: number
  maxPrice?: number
  images: string[]
  videos: string[]
  rating: number
  reviewCount: number
  soldCount: number
  isNew: boolean
  badge: string
  featured: boolean
  emoji: string
  imageHue: number[]
  variants: ProductVariant[]
  details: string[]
  deleted?: boolean
  stock: AdminStockSnapshot | null
}

export interface AdminProductMutationPayload {
  id?: string
  name: string
  shortDescription: string
  description: string
  brand: string
  categoryId: string
  categoryName: string
  images: string[]
  videos: string[]
  isNew: boolean
  badge: string
  featured: boolean
  emoji: string
  imageHue: number[]
  variants: AdminProductVariantPayload[]
  details: string[]
}

export interface ListAdminProductsParams {
  q?: string
  categoryId?: string
  status?: "all" | "active" | "deleted"
  sort?: string
  page?: number
  pageSize?: number
}

export type MediaAssetType = "IMAGE" | "VIDEO"

export interface MediaPresignRequest {
  filename: string
  contentType: string
  sizeBytes: number
}

export interface MediaPresignData {
  uploadId: string
  mediaType: MediaAssetType
  uploadUrl: string
  publicUrl: string
  objectKey: string
  expiresInSeconds: number
}

export type InventoryStockBucket = "IN_STOCK" | "LOW_STOCK" | "OUT_OF_STOCK"

export interface AdminInventoryItem {
  skuId: string
  productId: string
  name: string
  variantLabel: string
  variantColorHex?: string
  brand: string
  categoryId: string
  categoryName: string
  price: number
  originalPrice?: number
  imageUrl: string
  emoji: string
  imageHue: number[]
  deleted: boolean
  quantity: number
  reserved: number
  availableQuantity: number
  stockStatus: InventoryStockBucket
  updatedAt?: string
}

export interface AdminInventorySummary {
  totalTrackedProducts: number
  totalTrackedSkus: number
  inStockCount: number
  lowStockCount: number
  outOfStockCount: number
  totalQuantity: number
  totalReserved: number
  totalAvailable: number
}

export interface AdminInventoryPage {
  items: AdminInventoryItem[]
  page: number
  pageSize: number
  totalItems: number
  totalPages: number
  summary: AdminInventorySummary
}

export interface ListAdminInventoryParams {
  q?: string
  categoryId?: string
  stockStatus?: "all" | "in_stock" | "low_stock" | "out_of_stock"
  includeDeleted?: boolean
  page?: number
  pageSize?: number
}

export interface StockMutationResult {
  skuId: string
  productId: string
  quantity: number
  reserved: number
  availableQuantity: number
  updatedAt?: string
}

export type AccountStatus = "ACTIVE" | "DISABLED"

export interface AdminUser {
  userId: string
  email: string
  fullName: string
  status: AccountStatus
  roles: string[]
  createdAt: string
  updatedAt: string
}

export interface AdminUserPage {
  items: AdminUser[]
  page: number
  pageSize: number
  totalItems: number
  totalPages: number
  activeCount: number
  disabledCount: number
}

export interface ListAdminUsersParams {
  q?: string
  status?: AccountStatus
  page?: number
  pageSize?: number
}

export type RbacItemType = "ROLE" | "PERMISSION"

export interface RbacItem {
  name: string
  type: RbacItemType
  description: string | null
  systemItem: boolean
  directChildren: string[]
  createdAt: string
  updatedAt: string
}
