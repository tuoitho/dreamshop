import { createContext } from "react"
import type { FavoriteItem } from "@/lib/types"

export interface FavoriteLoadError {
  message: string
  code?: string
}

export interface FavoritesContextValue {
  items: FavoriteItem[]
  totalCount: number
  isFavorite: (productId: string) => boolean
  isPending: (productId: string) => boolean
  addFavorite: (productId: string) => Promise<void>
  removeFavorite: (productId: string) => Promise<void>
  toggleFavorite: (productId: string) => Promise<void>
  clearFavorites: () => Promise<void>
  loading: boolean
  degraded: boolean
  forbidden: boolean
  loadError: FavoriteLoadError | null
  retryLoad: () => Promise<void>
}

const noopAsync = async () => undefined

export const DEFAULT_FAVORITES_CONTEXT: FavoritesContextValue = {
  items: [],
  totalCount: 0,
  isFavorite: () => false,
  isPending: () => false,
  addFavorite: noopAsync,
  removeFavorite: noopAsync,
  toggleFavorite: noopAsync,
  clearFavorites: noopAsync,
  loading: false,
  degraded: false,
  forbidden: false,
  loadError: null,
  retryLoad: noopAsync,
}

export const FavoritesContext = createContext<FavoritesContextValue>(DEFAULT_FAVORITES_CONTEXT)
