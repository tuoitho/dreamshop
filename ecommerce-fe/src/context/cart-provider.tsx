import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react"
import { getProductById } from "@/data/products"
import { CartContext, type CartContextValue, type CartItem } from "./cart-context"

const STORAGE_KEY = "ecommerce-fe.cart"

type StoredItem = Omit<CartItem, "selected">

function loadCart(): CartItem[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as StoredItem[]
    return parsed.map((item) => ({ ...item, selected: true }))
  } catch {
    return []
  }
}

export function CartProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<CartItem[]>(() => loadCart())

  useEffect(() => {
    const stored: StoredItem[] = items.map(({ productId, variantIndex, quantity }) => ({
      productId,
      variantIndex,
      quantity,
    }))
    localStorage.setItem(STORAGE_KEY, JSON.stringify(stored))
  }, [items])

  const addItem = useCallback((productId: string, variantIndex: number, quantity: number) => {
    setItems((prev) => {
      const existing = prev.find(
        (item) => item.productId === productId && item.variantIndex === variantIndex
      )
      if (existing) {
        return prev.map((item) =>
          item.productId === productId && item.variantIndex === variantIndex
            ? { ...item, quantity: Math.min(item.quantity + quantity, 99), selected: true }
            : item
        )
      }
      return [...prev, { productId, variantIndex, quantity, selected: true }]
    })
  }, [])

  const updateQuantity = useCallback((productId: string, variantIndex: number, quantity: number) => {
    setItems((prev) =>
      prev
        .map((item) =>
          item.productId === productId && item.variantIndex === variantIndex
            ? { ...item, quantity: Math.max(0, Math.min(quantity, 99)) }
            : item
        )
        .filter((item) => item.quantity > 0)
    )
  }, [])

  const removeItem = useCallback((productId: string, variantIndex: number) => {
    setItems((prev) =>
      prev.filter((item) => !(item.productId === productId && item.variantIndex === variantIndex))
    )
  }, [])

  const toggleSelected = useCallback((productId: string, variantIndex: number) => {
    setItems((prev) =>
      prev.map((item) =>
        item.productId === productId && item.variantIndex === variantIndex
          ? { ...item, selected: !item.selected }
          : item
      )
    )
  }, [])

  const toggleAllSelected = useCallback(() => {
    setItems((prev) => {
      const next = !prev.every((item) => item.selected)
      return prev.map((item) => ({ ...item, selected: next }))
    })
  }, [])

  const removeSelected = useCallback(() => {
    setItems((prev) => prev.filter((item) => !item.selected))
  }, [])

  const clearCart = useCallback(() => {
    setItems([])
  }, [])

  const value = useMemo<CartContextValue>(() => {
    const selectedItems = items.filter((item) => item.selected)
    const selectedTotal = selectedItems.reduce((sum, item) => {
      const product = getProductById(item.productId)
      return sum + (product ? product.price * item.quantity : 0)
    }, 0)

    return {
      items,
      addItem,
      updateQuantity,
      removeItem,
      toggleSelected,
      toggleAllSelected,
      removeSelected,
      clearCart,
      selectedCount: selectedItems.reduce((sum, item) => sum + item.quantity, 0),
      totalQuantity: items.reduce((sum, item) => sum + item.quantity, 0),
      selectedTotal,
      allSelected: items.length > 0 && items.every((item) => item.selected),
    }
  }, [items, addItem, updateQuantity, removeItem, toggleSelected, toggleAllSelected, removeSelected, clearCart])

  return <CartContext value={value}>{children}</CartContext>
}
