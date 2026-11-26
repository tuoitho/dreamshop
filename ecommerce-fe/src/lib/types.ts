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

export type ProductAvailabilityStatus =
  | "AVAILABLE"
  | "LIMITED"
  | "OUT_OF_STOCK"
  | "UNAVAILABLE"
  | "UNKNOWN"

export interface FavoriteVariant {
  skuId: string
  label: string
  colorHex?: string
  price: number
  originalPrice?: number
}

export interface ProductVariant extends FavoriteVariant {
  status: ProductAvailabilityStatus
  availableQuantity: number
}

export type CartItemStatus = "AVAILABLE" | "LIMITED" | "OUT_OF_STOCK" | "UNAVAILABLE"

export interface CartItem {
  itemId: string
  productId: string
  skuId: string
  variantLabel: string
  quantity: number
  name: string
  brand: string
  categoryName: string
  unitPrice: number
  originalPrice?: number
  imageEmoji: string
  imageHue: [number, number] | number[]
  imageUrl: string
  variantColorHex?: string
  status: CartItemStatus
  availableQuantity: number
  productVersion?: number
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
  maxPrice?: number
  rating: number
  reviewCount: number
  soldCount: number
  imageUrl: string
  imageEmoji: string
  imageHue: [number, number] | number[]
  badge: string
  variants: FavoriteVariant[]
  status: FavoriteItemStatus
  availableQuantity: number
}

export interface FavoriteData {
  items: FavoriteItem[]
  degraded: boolean
}

export interface ProductSummary {
  id: string
  name: string
  brand: string
  categoryId: string
  categoryName: string
  price: number
  originalPrice: number
  maxPrice?: number
  rating: number
  reviewCount: number
  soldCount: number
  images?: string[]
  emoji?: string
  imageHue?: [number, number] | number[]
  isNew?: boolean
  badge?: "sale" | "hot" | "new" | ""
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
  videos?: string[]
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

export interface ShippingAddress {
  id: string
  recipientName: string
  phone: string
  provinceCode: string
  wardCode: string
  addressLine: string
  label: string | null
  isDefault: boolean
  createdAt: string
  updatedAt: string
}

export interface CreateShippingAddressPayload {
  recipientName: string
  phone: string
  provinceCode: string
  wardCode: string
  addressLine: string
  label?: string | null
  isDefault?: boolean
}

export interface ShippingAddressSnapshot {
  recipientName: string
  phone: string
  provinceCode: string
  provinceName: string
  wardCode: string
  wardName: string
  addressLine: string
  label?: string
}

export type PaymentMethod = "COD" | "MOMO"

export type OrderStatus =
  | "PENDING_STOCK"
  | "PENDING_PAYMENT"
  | "PENDING_CONFIRMATION"
  | "CONFIRMED"
  | "PREPARING"
  | "SHIPPING"
  | "DELIVERED"
  | "DELIVERY_FAILED"
  | "CANCELLED"

export interface OrderStatusChange {
  fromStatus?: OrderStatus | string
  toStatus: OrderStatus | string
  actorType: "SYSTEM" | "CUSTOMER" | "ADMIN" | string
  reason?: string
  shippingCarrier?: string
  trackingNumber?: string
  createdAt: string
}

export type PaymentStatus =
  | "UNPAID"
  | "PENDING"
  | "PAID"
  | "FAILED"
  | "REFUND_REQUIRED"
  | "REFUNDED"
  | "REFUND_FAILED"

export interface OrderItemSnapshot {
  cartItemId?: string
  productId: string
  skuId: string
  quantity: number
  unitPrice: number
  lineTotal: number
  expectedName: string
  brand: string
  categoryName: string
  variantLabel: string
  variantColorHex?: string
  imageUrl: string
  imageEmoji: string
  imageHue: [number, number] | number[]
  expectedProductVersion: number
}

export interface OrderDetail {
  orderId: string
  orderCode: string
  userId: string
  status: OrderStatus | string
  paymentMethod: PaymentMethod | string
  paymentStatus: PaymentStatus | string
  shippingAddress: ShippingAddressSnapshot
  items: OrderItemSnapshot[]
  subtotal: number
  shippingFee: number
  totalAmount: number
  currency: string
  note?: string
  cancelReason?: string
  payUrl?: string
  paymentExpiresAt?: string
  autoConfirmAt?: string
  shippingCarrier?: string
  trackingNumber?: string
  refundTransId?: string
  refundedAt?: string
  createdAt: string
  updatedAt: string
  /** Present on single-order reads; list responses return an empty array. */
  statusHistory?: OrderStatusChange[]
}

export interface CreateOrderItemPayload {
  cartItemId?: string
  productId: string
  skuId: string
  quantity: number
  unitPrice: number
  expectedName: string
  brand?: string
  categoryName?: string
  variantLabel: string
  variantColorHex?: string
  imageUrl?: string
  imageEmoji?: string
  imageHue?: [number, number] | number[]
  expectedProductVersion: number
}

export interface CreateOrderPayload {
  paymentMethod: PaymentMethod
  note?: string
  shippingAddress: ShippingAddressSnapshot
  items: CreateOrderItemPayload[]
}

export interface CreateOrderResult {
  order: OrderDetail
  payUrl?: string
}

export interface OrderListResult {
  items: OrderDetail[]
  page: number
  pageSize: number
  totalItems: number
  totalPages: number
}
