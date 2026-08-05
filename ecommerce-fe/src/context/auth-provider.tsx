import { useCallback, useMemo, useState, type ReactNode } from "react"
import { login as loginRequest, register as registerRequest } from "@/lib/api"
import { clearAccessToken, getAccessToken, setAccessToken } from "@/lib/token-storage"
import type { LoginRequest, RegisterRequest } from "@/lib/types"
import { AuthContext, type AuthContextValue } from "./auth-context"

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => getAccessToken())

  const login = useCallback(async (payload: LoginRequest) => {
    const response = await loginRequest(payload)
    setAccessToken(response.access_token)
    setToken(response.access_token)
  }, [])

  const register = useCallback(async (payload: RegisterRequest) => {
    await registerRequest(payload)
  }, [])

  const logout = useCallback(() => {
    clearAccessToken()
    setToken(null)
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({ token, isAuthenticated: token !== null, login, register, logout }),
    [token, login, register, logout]
  )

  return <AuthContext value={value}>{children}</AuthContext>
}
