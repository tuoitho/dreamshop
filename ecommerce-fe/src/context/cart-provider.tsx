import { useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react"
import { addCartItem, clearCart as clearCartRequest, getCart, removeCartItem, updateCartItemQuantity } from "@/lib/api"
import type { CartData, CartItem } from "@/lib/types"
import { AuthContext } from "@/context/auth-context"
import { CartContext, type CartContextValue } from "./cart-context"

function selectionKey(userId: string): string {
  return `ecommerce-fe.cart-selection.${userId}`
}

function readSelection(key: string): Record<string, boolean> {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(key) ?? "{}")
    if (!value || typeof value !== "object" || Array.isArray(value)) return {}
    return Object.fromEntries(
      Object.entries(value).filter((entry): entry is [string, boolean] => typeof entry[1] === "boolean")
    )
  } catch {
    return {}
  }
}

function reconcileSelection(items: CartItem[], current: Record<string, boolean>): Record<string, boolean> {
  return Object.fromEntries(items.map((item) => [
    item.itemId,
    item.status === "OUT_OF_STOCK" || item.status === "UNAVAILABLE"
      ? false
      : current[item.itemId] ?? true,
  ]))
}

function isSelectable(item: CartItem): boolean {
  return item.status === "AVAILABLE" || item.status === "LIMITED"
}

export function CartProvider({ children }: { children: ReactNode }) {
  const auth = useContext(AuthContext)
  const ready = auth?.ready ?? true
  const isAuthenticated = auth?.isAuthenticated ?? false
  const user = auth?.user ?? null
  const [items, setItems] = useState<CartItem[]>([])
  const [selection, setSelection] = useState<Record<string, boolean>>({})
  const [selectionOwner, setSelectionOwner] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [degraded, setDegraded] = useState(false)

  const applyCart = useCallback((data: CartData) => {
    setItems(data.items)
    setDegraded(data.degraded)
    setSelection((current) => reconcileSelection(data.items, current))
  }, [])

  useEffect(() => {
    if (!ready || !isAuthenticated || !user) {
      setItems([])
      setSelection({})
      setSelectionOwner(null)
      setDegraded(false)
      setLoading(false)
      return
    }

    const key = selectionKey(user.userId)
    setItems([])
    setDegraded(false)
    setSelection(readSelection(key))
    setSelectionOwner(user.userId)
    setLoading(true)
    let active = true
    void getCart()
      .then((data) => {
        if (active) applyCart(data)
      })
      .catch(() => {
        if (active) setItems([])
      })
      .finally(() => {
        if (active) setLoading(false)
      })

    return () => {
      active = false
    }
  }, [applyCart, isAuthenticated, ready, user])

  useEffect(() => {
    if (ready && isAuthenticated && user && selectionOwner === user.userId) {
      localStorage.setItem(selectionKey(user.userId), JSON.stringify(selection))
    }
  }, [isAuthenticated, ready, selection, selectionOwner, user])

  const runMutation = useCallback(async (request: () => Promise<CartData>) => {
    setLoading(true)
    try {
      applyCart(await request())
    } finally {
      setLoading(false)
    }
  }, [applyCart])

  const addItem = useCallback((productId: string, variantLabel: string, quantity: number) =>
    runMutation(() => addCartItem({ productId, variantLabel, quantity })), [runMutation])

  const updateQuantity = useCallback((itemId: string, quantity: number) =>
    runMutation(() => updateCartItemQuantity(itemId, quantity)), [runMutation])

  const removeItem = useCallback((itemId: string) =>
    runMutation(() => removeCartItem(itemId)), [runMutation])

  const removeSelected = useCallback(async () => {
    const selected = items.filter((item) => selection[item.itemId] && isSelectable(item))
    setLoading(true)
    try {
      for (const item of selected) {
        applyCart(await removeCartItem(item.itemId))
      }
    } finally {
      setLoading(false)
    }
  }, [applyCart, items, selection])

  const clearCart = useCallback(() => runMutation(clearCartRequest), [runMutation])

  const toggleSelected = useCallback((itemId: string) => {
    const item = items.find((entry) => entry.itemId === itemId)
    if (!item || !isSelectable(item)) return
    setSelection((current) => ({ ...current, [itemId]: !current[itemId] }))
  }, [items])

  const toggleAllSelected = useCallback(() => {
    setSelection((current) => {
      const selectableItems = items.filter(isSelectable)
      const shouldSelect = !selectableItems.every((item) => current[item.itemId])
      return {
        ...current,
        ...Object.fromEntries(selectableItems.map((item) => [item.itemId, shouldSelect])),
      }
    })
  }, [items])

  const value = useMemo<CartContextValue>(() => {
    const selectedItems = items.filter((item) => selection[item.itemId] && isSelectable(item))
    return {
      items,
      addItem,
      updateQuantity,
      removeItem,
      removeSelected,
      clearCart,
      toggleSelected,
      toggleAllSelected,
      selectedItems,
      selectedCount: selectedItems.reduce((sum, item) => sum + item.quantity, 0),
      totalQuantity: items.reduce((sum, item) => sum + item.quantity, 0),
      selectedTotal: selectedItems.reduce((sum, item) => sum + item.unitPrice * item.quantity, 0),
      allSelected: selectedItems.length > 0 && selectedItems.length === items.filter(isSelectable).length,
      loading,
      degraded,
    }
  }, [addItem, clearCart, degraded, items, loading, removeItem, removeSelected, selection, toggleAllSelected, toggleSelected, updateQuantity])

  return <CartContext value={value}>{children}</CartContext>
}
