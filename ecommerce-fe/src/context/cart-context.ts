import { createContext } from "react"

export interface CartItem {
  productId: string
  variantIndex: number
  quantity: number
  selected: boolean
}

export interface CartContextValue {
  items: CartItem[]
  addItem: (productId: string, variantIndex: number, quantity: number) => void
  updateQuantity: (productId: string, variantIndex: number, quantity: number) => void
  removeItem: (productId: string, variantIndex: number) => void
  toggleSelected: (productId: string, variantIndex: number) => void
  toggleAllSelected: () => void
  removeSelected: () => void
  clearCart: () => void
  selectedCount: number
  totalQuantity: number
  selectedTotal: number
  allSelected: boolean
}

export const CartContext = createContext<CartContextValue | null>(null)
