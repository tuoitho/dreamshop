import { useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import { addCartItem, clearCart as clearCartRequest, getCart, removeCartItem, toErrorInfo, updateCartItemQuantity } from "@/lib/api"
import type { CartData, CartItem } from "@/lib/types"
import { AuthContext } from "@/context/auth-context"
import { CartContext, type CartContextValue, type CartLoadError } from "./cart-context"

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

function authIdentity(ready: boolean, isAuthenticated: boolean, userId: string | undefined): string | null {
  return ready && isAuthenticated && userId ? userId : null
}

export function CartProvider({ children }: { children: ReactNode }) {
  const auth = useContext(AuthContext)
  const ready = auth?.ready ?? true
  const isAuthenticated = auth?.isAuthenticated ?? false
  const user = auth?.user ?? null
  const identity = authIdentity(ready, isAuthenticated, user?.userId)
  const sessionRef = useRef({ identity, version: 0 })
  if (sessionRef.current.identity !== identity) {
    sessionRef.current = { identity, version: sessionRef.current.version + 1 }
  }

  const [items, setItems] = useState<CartItem[]>([])
  const [selection, setSelection] = useState<Record<string, boolean>>({})
  const [selectionOwner, setSelectionOwner] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [degraded, setDegraded] = useState(false)
  const [loadError, setLoadError] = useState<CartLoadError | null>(null)
  const [loadAttempt, setLoadAttempt] = useState(0)
  const operationGeneration = useRef(0)
  const mutationQueue = useRef(Promise.resolve())

  useEffect(() => {
    mutationQueue.current = Promise.resolve()
  }, [identity])

  const currentSession = useCallback((version: number, expectedIdentity: string): boolean =>
    sessionRef.current.version === version && sessionRef.current.identity === expectedIdentity,
  [])

  const applyCart = useCallback((data: CartData) => {
    setItems(data.items)
    setDegraded(data.degraded)
    setSelection((current) => reconcileSelection(data.items, current))
  }, [])

  useEffect(() => {
    if (!identity) {
      setItems([])
      setSelection({})
      setSelectionOwner(null)
      setDegraded(false)
      setLoadError(null)
      setLoading(false)
      return
    }

    const sessionVersion = sessionRef.current.version
    const operation = ++operationGeneration.current
    const key = selectionKey(identity)
    setItems([])
    setDegraded(false)
    setLoadError(null)
    setSelection(readSelection(key))
    setSelectionOwner(identity)
    setLoading(true)
    let active = true
    void getCart()
      .then((data) => {
        if (active && currentSession(sessionVersion, identity) && operationGeneration.current === operation) {
          applyCart(data)
          setLoadError(null)
        }
      })
      .catch((error: unknown) => {
        if (active && currentSession(sessionVersion, identity) && operationGeneration.current === operation) {
          setLoadError(toErrorInfo(error))
        }
      })
      .finally(() => {
        if (active && currentSession(sessionVersion, identity) && operationGeneration.current === operation) {
          setLoading(false)
        }
      })

    return () => {
      active = false
    }
  }, [applyCart, currentSession, identity, loadAttempt])

  useEffect(() => {
    if (identity && selectionOwner === identity) {
      localStorage.setItem(selectionKey(identity), JSON.stringify(selection))
    }
  }, [identity, selection, selectionOwner])

  const retryLoad = useCallback(async () => {
    if (!identity) return
    setLoadAttempt((attempt) => attempt + 1)
  }, [identity])

  const runMutation = useCallback((request: () => Promise<CartData>): Promise<void> => {
    const expectedIdentity = identity
    const sessionVersion = sessionRef.current.version
    if (!expectedIdentity || !ready || !isAuthenticated || !user) return Promise.resolve()

    const execute = async () => {
      if (!currentSession(sessionVersion, expectedIdentity)) return
      const operation = ++operationGeneration.current
      setLoading(true)
      try {
        const data = await request()
        if (currentSession(sessionVersion, expectedIdentity) && operationGeneration.current === operation) {
          applyCart(data)
          setLoadError(null)
        }
      } finally {
        if (currentSession(sessionVersion, expectedIdentity) && operationGeneration.current === operation) {
          setLoading(false)
        }
      }
    }

    const queued = mutationQueue.current.then(execute, execute)
    mutationQueue.current = queued.catch(() => undefined)
    return queued
  }, [applyCart, currentSession, identity, isAuthenticated, ready, user])

  const addItem = useCallback((productId: string, variantLabel: string, quantity: number) =>
    runMutation(() => addCartItem({ productId, variantLabel, quantity })), [runMutation])

  const updateQuantity = useCallback((itemId: string, quantity: number) =>
    runMutation(() => updateCartItemQuantity(itemId, quantity)), [runMutation])

  const removeItem = useCallback((itemId: string) =>
    runMutation(() => removeCartItem(itemId)), [runMutation])

  const removeSelected = useCallback(() => {
    const selected = items.filter((item) => selection[item.itemId] && isSelectable(item))
    return selected.reduce(
      (promise, item) => promise.then(() => runMutation(() => removeCartItem(item.itemId))),
      Promise.resolve()
    )
  }, [items, runMutation, selection])

  const clearCart = useCallback(() => runMutation(clearCartRequest), [runMutation])

  const toggleSelected = useCallback((itemId: string) => {
    const item = items.find((entry) => entry.itemId === itemId)
    if (!item || !isSelectable(item)) return
    setSelection((current) => ({ ...current, [itemId]: !current[itemId] }))
  }, [items])

  const selectOnly = useCallback((itemId: string) => {
    const item = items.find((entry) => entry.itemId === itemId)
    if (!item || !isSelectable(item)) return
    setSelection(Object.fromEntries(items.map((entry) => [entry.itemId, entry.itemId === itemId])))
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
      selectOnly,
      toggleAllSelected,
      selectedItems,
      selectedCount: selectedItems.reduce((sum, item) => sum + item.quantity, 0),
      totalQuantity: items.reduce((sum, item) => sum + item.quantity, 0),
      selectedTotal: selectedItems.reduce((sum, item) => sum + item.unitPrice * item.quantity, 0),
      allSelected: selectedItems.length > 0 && selectedItems.length === items.filter(isSelectable).length,
      loading,
      degraded,
      loadError,
      retryLoad,
    }
  }, [addItem, clearCart, degraded, items, loadError, loading, removeItem, removeSelected, retryLoad, selectOnly, selection, toggleAllSelected, toggleSelected, updateQuantity])

  return <CartContext value={value}>{children}</CartContext>
}
