import { useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import {
  addFavoriteItem,
  clearFavorites as clearFavoritesRequest,
  getFavorites,
  removeFavoriteItem,
  toClientError,
  toErrorInfo,
} from "@/lib/api"
import type { FavoriteData, FavoriteItem } from "@/lib/types"
import { AuthContext } from "@/context/auth-context"
import {
  FavoritesContext,
  type FavoriteLoadError,
  type FavoritesContextValue,
} from "./favorites-context"

function authIdentity(ready: boolean, isAuthenticated: boolean, userId: string | undefined): string | null {
  return ready && isAuthenticated && userId ? userId : null
}

function isForbiddenError(error: unknown): boolean {
  const clientError = toClientError(error)
  return clientError.code === "FORBIDDEN" || clientError.status === 403
}

export function FavoritesProvider({ children }: { children: ReactNode }) {
  const auth = useContext(AuthContext)
  const ready = auth?.ready ?? true
  const isAuthenticated = auth?.isAuthenticated ?? false
  const user = auth?.user ?? null
  const identity = authIdentity(ready, isAuthenticated, user?.userId)
  const sessionRef = useRef({ identity, version: 0 })
  if (sessionRef.current.identity !== identity) {
    sessionRef.current = { identity, version: sessionRef.current.version + 1 }
  }

  const [items, setItems] = useState<FavoriteItem[]>([])
  const [loading, setLoading] = useState(false)
  const [degraded, setDegraded] = useState(false)
  const [forbidden, setForbidden] = useState(false)
  const [loadError, setLoadError] = useState<FavoriteLoadError | null>(null)
  const [loadAttempt, setLoadAttempt] = useState(0)
  const [pendingCounts, setPendingCounts] = useState<Record<string, number>>({})
  const [clearing, setClearing] = useState(false)
  const operationGeneration = useRef(0)
  const mutationQueue = useRef(Promise.resolve())

  const favoritedIds = useMemo(() => new Set(items.map((item) => item.productId)), [items])

  useEffect(() => {
    mutationQueue.current = Promise.resolve()
  }, [identity])

  const currentSession = useCallback(
    (version: number, expectedIdentity: string): boolean =>
      sessionRef.current.version === version && sessionRef.current.identity === expectedIdentity,
    []
  )

  const applyFavorites = useCallback((data: FavoriteData) => {
    setItems(data.items)
    setDegraded(data.degraded)
    setForbidden(false)
  }, [])

  useEffect(() => {
    if (!identity) {
      setItems([])
      setDegraded(false)
      setForbidden(false)
      setLoadError(null)
      setLoading(false)
      setPendingCounts({})
      setClearing(false)
      return
    }

    const sessionVersion = sessionRef.current.version
    const operation = ++operationGeneration.current
    setItems([])
    setDegraded(false)
    setForbidden(false)
    setLoadError(null)
    setPendingCounts({})
    setClearing(false)
    setLoading(true)
    let active = true

    void getFavorites()
      .then((data) => {
        if (active && currentSession(sessionVersion, identity) && operationGeneration.current === operation) {
          applyFavorites(data)
          setLoadError(null)
        }
      })
      .catch((error: unknown) => {
        if (active && currentSession(sessionVersion, identity) && operationGeneration.current === operation) {
          if (isForbiddenError(error)) {
            setForbidden(true)
            setItems([])
            setDegraded(false)
            setLoadError(null)
          } else {
            setLoadError(toErrorInfo(error))
          }
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
  }, [applyFavorites, currentSession, identity, loadAttempt])

  const retryLoad = useCallback(async () => {
    if (!identity) return
    setLoadAttempt((attempt) => attempt + 1)
  }, [identity])

  const runMutation = useCallback(
    (productId: string | null, request: () => Promise<FavoriteData>): Promise<void> => {
      const expectedIdentity = identity
      const sessionVersion = sessionRef.current.version
      if (!expectedIdentity || !ready || !isAuthenticated || !user) return Promise.resolve()

      if (productId) {
        setPendingCounts((prev) => ({ ...prev, [productId]: (prev[productId] ?? 0) + 1 }))
      } else {
        setClearing(true)
      }

      const decrementPending = () => {
        if (!currentSession(sessionVersion, expectedIdentity)) return
        if (productId) {
          setPendingCounts((prev) => {
            const nextCount = (prev[productId] ?? 1) - 1
            if (nextCount <= 0) {
              const rest = { ...prev }
              delete rest[productId]
              return rest
            }
            return { ...prev, [productId]: nextCount }
          })
        } else {
          setClearing(false)
        }
      }

      const execute = async () => {
        if (!currentSession(sessionVersion, expectedIdentity)) return
        try {
          const data = await request()
          if (currentSession(sessionVersion, expectedIdentity)) {
            operationGeneration.current += 1
            applyFavorites(data)
            setLoadError(null)
            setLoading(false)
          }
        } catch (error: unknown) {
          if (currentSession(sessionVersion, expectedIdentity) && isForbiddenError(error)) {
            operationGeneration.current += 1
            setForbidden(true)
            setItems([])
            setDegraded(false)
            setLoadError(null)
            setLoading(false)
          }
          throw error
        } finally {
          decrementPending()
        }
      }

      const queued = mutationQueue.current.then(execute, execute)
      mutationQueue.current = queued.catch(() => undefined)
      return queued
    },
    [applyFavorites, currentSession, identity, isAuthenticated, ready, user]
  )

  const addFavorite = useCallback(
    (productId: string) => runMutation(productId, () => addFavoriteItem(productId)),
    [runMutation]
  )

  const removeFavorite = useCallback(
    (productId: string) => runMutation(productId, () => removeFavoriteItem(productId)),
    [runMutation]
  )

  const toggleFavorite = useCallback(
    (productId: string) =>
      favoritedIds.has(productId) ? removeFavorite(productId) : addFavorite(productId),
    [addFavorite, favoritedIds, removeFavorite]
  )

  const clearFavorites = useCallback(
    () => runMutation(null, clearFavoritesRequest),
    [runMutation]
  )

  const isFavorite = useCallback(
    (productId: string): boolean => favoritedIds.has(productId),
    [favoritedIds]
  )

  const isPending = useCallback(
    (productId: string): boolean => clearing || Boolean(pendingCounts[productId]),
    [clearing, pendingCounts]
  )

  const value = useMemo<FavoritesContextValue>(
    () => ({
      items,
      totalCount: items.length,
      isFavorite,
      isPending,
      addFavorite,
      removeFavorite,
      toggleFavorite,
      clearFavorites,
      loading,
      degraded,
      forbidden,
      loadError,
      retryLoad,
    }),
    [
      addFavorite,
      clearFavorites,
      degraded,
      forbidden,
      isFavorite,
      isPending,
      items,
      loadError,
      loading,
      removeFavorite,
      retryLoad,
      toggleFavorite,
    ]
  )

  return <FavoritesContext value={value}>{children}</FavoritesContext>
}
