import { createContext } from "react"
import type { LoginRequest, RegisterRequest } from "@/lib/types"

export interface AuthContextValue {
  token: string | null
  isAuthenticated: boolean
  login: (payload: LoginRequest) => Promise<void>
  register: (payload: RegisterRequest) => Promise<void>
  logout: () => void
}

export const AuthContext = createContext<AuthContextValue | null>(null)
