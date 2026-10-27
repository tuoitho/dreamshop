import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react"
import * as api from "@/lib/api"
import {
  advanceSessionEpoch,
  clearAccessToken,
  getSessionEpoch,
  setAccessToken,
  subscribeAccessToken,
} from "@/lib/auth-session"
import { hasAnyAdminPermission } from "@/lib/permissions"
import type { CurrentUser, LoginRequest } from "@/lib/types"
import { AuthContext, type AuthStatus } from "./auth-context"

const EMPTY_PERMISSIONS: ReadonlySet<string> = new Set()

export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>("loading")
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [permissions, setPermissions] = useState<ReadonlySet<string>>(EMPTY_PERMISSIONS)

  const hydrateSession = useCallback(async (expectedEpoch: number) => {
    const [profile, effectivePerms] = await Promise.all([
      api.fetchCurrentUser(),
      api.fetchMyPermissions(),
    ])
    if (getSessionEpoch() !== expectedEpoch) {
      return
    }
    setUser(profile)
    setPermissions(new Set(effectivePerms))
    setStatus("authenticated")
  }, [])

  useEffect(() => {
    let active = true
    const bootEpoch = getSessionEpoch()

    void (async () => {
      try {
        await api.refreshAccessToken()
        if (!active || getSessionEpoch() !== bootEpoch) return
        await hydrateSession(bootEpoch)
      } catch {
        if (!active || getSessionEpoch() !== bootEpoch) return
        clearAccessToken()
        setUser(null)
        setPermissions(EMPTY_PERMISSIONS)
        setStatus("unauthenticated")
      }
    })()

    const unsubscribe = subscribeAccessToken((token) => {
      if (token === null) {
        setUser(null)
        setPermissions(EMPTY_PERMISSIONS)
        setStatus("unauthenticated")
      }
    })

    return () => {
      active = false
      unsubscribe()
    }
  }, [hydrateSession])

  const login = useCallback(
    async (payload: LoginRequest) => {
      const nextEpoch = advanceSessionEpoch()
      const loginResp = await api.login(payload)
      if (getSessionEpoch() !== nextEpoch) {
        return
      }
      setAccessToken(loginResp.accessToken)
      try {
        await hydrateSession(nextEpoch)
      } catch (error) {
        if (getSessionEpoch() === nextEpoch) {
          clearAccessToken()
          setUser(null)
          setPermissions(EMPTY_PERMISSIONS)
          setStatus("unauthenticated")
        }
        throw error
      }
    },
    [hydrateSession]
  )

  const logout = useCallback(async () => {
    clearAccessToken()
    setUser(null)
    setPermissions(EMPTY_PERMISSIONS)
    setStatus("unauthenticated")
    try {
      await api.logoutRequest()
    } catch {
      // Cookie cleanup best-effort; local memory token is already cleared.
    }
  }, [])

  const refreshPermissions = useCallback(async () => {
    const epoch = getSessionEpoch()
    const effectivePerms = await api.fetchMyPermissions()
    if (getSessionEpoch() !== epoch) return
    setPermissions(new Set(effectivePerms))
  }, [])

  const can = useCallback((permission: string) => permissions.has(permission), [permissions])

  const hasAnyAdminAccess = useMemo(() => hasAnyAdminPermission(permissions), [permissions])

  const value = useMemo(
    () => ({
      status,
      user,
      permissions,
      hasAnyAdminAccess,
      can,
      login,
      logout,
      refreshPermissions,
    }),
    [status, user, permissions, hasAnyAdminAccess, can, login, logout, refreshPermissions]
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
