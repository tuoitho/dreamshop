import { createContext } from "react"
import type { CurrentUser, LoginRequest } from "@/lib/types"

export type AuthStatus = "loading" | "authenticated" | "unauthenticated"

export interface AuthContextValue {
  status: AuthStatus
  user: CurrentUser | null
  permissions: ReadonlySet<string>
  hasAnyAdminAccess: boolean
  can: (permission: string) => boolean
  login: (payload: LoginRequest) => Promise<void>
  logout: () => Promise<void>
  refreshPermissions: () => Promise<void>
}

export const AuthContext = createContext<AuthContextValue | null>(null)
