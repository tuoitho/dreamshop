import { useEffect, useRef, useState, type ReactNode } from "react"
import { fetchCurrentUser, login as loginRequest, logoutRequest, refreshAccessToken, register as registerRequest, updateCurrentUser } from "@/lib/api"
import { advanceSessionEpoch, clearAccessToken, getAccessToken, setAccessToken, subscribeAccessToken } from "@/lib/auth-session"
import type { CurrentUser, LoginRequest, RegisterRequest, UpdateProfileRequest } from "@/lib/types"
import { AuthContext, type AuthContextValue } from "./auth-context"

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => getAccessToken())
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [ready, setReady] = useState(false)
  const bootstrapped = useRef(false)
  const authGenerationRef = useRef(0)

  useEffect(() => {
    return subscribeAccessToken((nextToken) => {
      if (nextToken === null) {
        authGenerationRef.current += 1
        setToken(null)
        setUser(null)
        return
      }
      setToken((currentToken) => (currentToken !== null ? nextToken : currentToken))
    })
  }, [])

  useEffect(() => {
    if (bootstrapped.current) return
    bootstrapped.current = true
    const generation = authGenerationRef.current
    void (async () => {
      try {
        const refreshedToken = await refreshAccessToken()
        if (authGenerationRef.current !== generation) return
        const currentUser = await fetchCurrentUser()
        if (authGenerationRef.current !== generation) return
        const activeToken = getAccessToken() ?? refreshedToken
        setAccessToken(activeToken)
        setToken(activeToken)
        setUser(currentUser)
      } catch {
        if (authGenerationRef.current !== generation) return
        clearAccessToken()
        setToken(null)
        setUser(null)
      } finally {
        setReady(true)
      }
    })()
  }, [])

  async function login(payload: LoginRequest) {
    const generation = ++authGenerationRef.current
    advanceSessionEpoch()
    try {
      const response = await loginRequest(payload)
      if (authGenerationRef.current !== generation) return
      setAccessToken(response.accessToken)
      const currentUser = await fetchCurrentUser()
      if (authGenerationRef.current !== generation) return
      const activeToken = getAccessToken() ?? response.accessToken
      setAccessToken(activeToken)
      setToken(activeToken)
      setUser(currentUser)
      setReady(true)
    } catch (error) {
      if (authGenerationRef.current === generation) {
        clearAccessToken()
        setToken(null)
        setUser(null)
        setReady(true)
      }
      throw error
    }
  }

  async function register(payload: RegisterRequest) {
    await registerRequest(payload)
  }

  async function updateProfile(payload: UpdateProfileRequest) {
    const generation = authGenerationRef.current
    const updatedUser = await updateCurrentUser(payload)
    if (authGenerationRef.current !== generation) return
    setUser(updatedUser)
  }

  async function logout() {
    const generation = ++authGenerationRef.current
    advanceSessionEpoch()
    try {
      await logoutRequest()
    } finally {
      if (authGenerationRef.current === generation) {
        clearAccessToken()
        setToken(null)
        setUser(null)
        setReady(true)
      }
    }
  }

  const value: AuthContextValue = { token, user, ready, isAuthenticated: token !== null, login, register, updateProfile, logout }
  return <AuthContext value={value}>{children}</AuthContext>
}
