import { createContext } from "react"
import type { CartItem } from "@/lib/types"

export interface CartLoadError {
  message: string
  code?: string
}

export interface CartContextValue {
  items: CartItem[]
  addItem: (productId: string, skuId: string, quantity: number) => Promise<void>
  updateQuantity: (itemId: string, quantity: number) => Promise<void>
  removeItem: (itemId: string) => Promise<void>
  removeSelected: () => Promise<void>
  clearCart: () => Promise<void>
  toggleSelected: (itemId: string) => void
  selectOnly: (itemId: string) => void
  toggleAllSelected: () => void
  selectedItems: CartItem[]
  selectedCount: number
  totalQuantity: number
  selectedTotal: number
  allSelected: boolean
  loading: boolean
  degraded: boolean
  forbidden?: boolean
  loadError: CartLoadError | null
  retryLoad: () => Promise<void>
}

export const CartContext = createContext<CartContextValue | null>(null)
