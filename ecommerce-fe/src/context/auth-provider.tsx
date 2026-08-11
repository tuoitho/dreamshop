import { useEffect, useRef, useState, type ReactNode } from "react"
import { fetchCurrentUser, login as loginRequest, logoutRequest, refreshAccessToken, register as registerRequest, updateCurrentUser } from "@/lib/api"
import { clearAccessToken, getAccessToken, setAccessToken } from "@/lib/auth-session"
import type { CurrentUser, LoginRequest, RegisterRequest, UpdateProfileRequest } from "@/lib/types"
import { AuthContext, type AuthContextValue } from "./auth-context"

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => getAccessToken())
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [ready, setReady] = useState(false)
  const bootstrapped = useRef(false)

  useEffect(() => {
    if (bootstrapped.current) return
    bootstrapped.current = true
    void (async () => {
      try {
        const refreshedToken = await refreshAccessToken()
        const currentUser = await fetchCurrentUser()
        setAccessToken(refreshedToken)
        setToken(refreshedToken)
        setUser(currentUser)
      } catch {
        clearAccessToken()
        setToken(null)
        setUser(null)
      } finally {
        setReady(true)
      }
    })()
  }, [])

  async function login(payload: LoginRequest) {
    const response = await loginRequest(payload)
    setAccessToken(response.accessToken)
    setToken(response.accessToken)
    setUser(await fetchCurrentUser())
  }

  async function register(payload: RegisterRequest) {
    await registerRequest(payload)
  }

  async function updateProfile(payload: UpdateProfileRequest) {
    setUser(await updateCurrentUser(payload))
  }

  async function logout() {
    try {
      await logoutRequest()
    } finally {
      clearAccessToken()
      setToken(null)
      setUser(null)
    }
  }

  const value: AuthContextValue = { token, user, ready, isAuthenticated: token !== null, login, register, updateProfile, logout }
  return <AuthContext value={value}>{children}</AuthContext>
}
