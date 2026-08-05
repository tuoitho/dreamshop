import { describe, it, expect, beforeEach } from "vitest"
import {
  clearAccessToken,
  getAccessToken,
  isAuthenticated,
  setAccessToken,
} from "./token-storage"

describe("token-storage", () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it("returns null when no token is stored", () => {
    expect(getAccessToken()).toBeNull()
    expect(isAuthenticated()).toBe(false)
  })

  it("stores and retrieves the access token", () => {
    setAccessToken("jwt-abc")
    expect(getAccessToken()).toBe("jwt-abc")
    expect(isAuthenticated()).toBe(true)
  })

  it("clears the access token", () => {
    setAccessToken("jwt-abc")
    clearAccessToken()
    expect(getAccessToken()).toBeNull()
    expect(isAuthenticated()).toBe(false)
  })
})
