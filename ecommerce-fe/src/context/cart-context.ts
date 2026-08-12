import { createContext } from "react"
import type { CartItem } from "@/lib/types"

export interface CartContextValue {
  items: CartItem[]
  addItem: (productId: string, variantLabel: string, quantity: number) => Promise<void>
  updateQuantity: (itemId: string, quantity: number) => Promise<void>
  removeItem: (itemId: string) => Promise<void>
  removeSelected: () => Promise<void>
  clearCart: () => Promise<void>
  toggleSelected: (itemId: string) => void
  toggleAllSelected: () => void
  selectedItems: CartItem[]
  selectedCount: number
  totalQuantity: number
  selectedTotal: number
  allSelected: boolean
  loading: boolean
  degraded: boolean
}

export const CartContext = createContext<CartContextValue | null>(null)
