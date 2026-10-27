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

export interface ProductVariant {
  label: string
  colorHex?: string
}

export type CartItemStatus = "AVAILABLE" | "LIMITED" | "OUT_OF_STOCK" | "UNAVAILABLE"

export interface CartItem {
  itemId: string
  productId: string
  variantLabel: string
  quantity: number
  name: string
  brand: string
  categoryName: string
  unitPrice: number
  imageEmoji: string
  imageHue: [number, number] | number[]
  imageUrl: string
  variantColorHex?: string
  status: CartItemStatus
  availableQuantity: number
}

export interface CartData {
  items: CartItem[]
  degraded: boolean
}

export type FavoriteItemStatus =
  | "AVAILABLE"
  | "LIMITED"
  | "OUT_OF_STOCK"
  | "UNAVAILABLE"
  | "UNKNOWN"

export interface FavoriteItem {
  itemId: string
  productId: string
  name: string
  brand: string
  categoryId: string
  categoryName: string
  price: number
  originalPrice: number
  rating: number
  reviewCount: number
  soldCount: number
  imageUrl: string
  imageEmoji: string
  imageHue: [number, number] | number[]
  badge: string
  variants: ProductVariant[]
  status: FavoriteItemStatus
  availableQuantity: number
}

export interface FavoriteData {
  items: FavoriteItem[]
  degraded: boolean
}

export type ProductAvailabilityStatus =
  | "AVAILABLE"
  | "LIMITED"
  | "OUT_OF_STOCK"
  | "UNAVAILABLE"
  | "UNKNOWN"

export interface ProductSummary {
  id: string
  name: string
  brand: string
  categoryId: string
  categoryName: string
  price: number
  originalPrice: number
  rating: number
  reviewCount: number
  soldCount: number
  images?: string[]
  emoji?: string
  imageHue?: [number, number]
  isNew?: boolean
  badge?: "sale" | "hot" | "new"
  featured?: boolean
  status?: ProductAvailabilityStatus
  availableQuantity?: number
  inventoryDegraded?: boolean
}

export interface ProductDetail extends ProductSummary {
  shortDescription?: string
  description: string
  details: string[]
  variants: ProductVariant[]
  deleted?: boolean
}

export type ProductSort = "pho-bien" | "gia-thap" | "gia-cao" | "moi-nhat" | "danh-gia"

export interface SearchProductsParams {
  q?: string
  categoryId?: string
  priceMin?: number
  priceMax?: number
  onSale?: boolean
  ratingMin?: number
  sort?: ProductSort
  page?: number
  pageSize?: number
}

export interface SearchProductsResult {
  items: ProductSummary[]
  page: number
  pageSize: number
  totalItems: number
  totalPages: number
}
