import { createContext } from "react"
import type { CurrentUser, LoginRequest, RegisterRequest, UpdateProfileRequest } from "@/lib/types"

export interface AuthContextValue {
  token: string | null
  user: CurrentUser | null
  ready: boolean
  isAuthenticated: boolean
  login: (payload: LoginRequest) => Promise<void>
  register: (payload: RegisterRequest) => Promise<void>
  updateProfile: (payload: UpdateProfileRequest) => Promise<void>
  logout: () => Promise<void>
}

export const AuthContext = createContext<AuthContextValue | null>(null)
