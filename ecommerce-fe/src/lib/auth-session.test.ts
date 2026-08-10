import { beforeEach, describe, expect, it, vi } from "vitest"
import { clearAccessToken, getAccessToken, runSingleFlightRefresh, setAccessToken } from "./auth-session"

describe("auth session", () => {
  beforeEach(() => clearAccessToken())

  it("keeps the access token only in module memory", () => {
    const setItem = vi.spyOn(Storage.prototype, "setItem")
    setAccessToken("jwt-abc")
    expect(getAccessToken()).toBe("jwt-abc")
    expect(setItem).not.toHaveBeenCalled()
  })

  it("shares one concurrent refresh", async () => {
    const refresh = vi.fn().mockResolvedValue("new-jwt")
    const [first, second] = await Promise.all([
      runSingleFlightRefresh(refresh),
      runSingleFlightRefresh(refresh),
    ])
    expect(first).toBe("new-jwt")
    expect(second).toBe("new-jwt")
    expect(refresh).toHaveBeenCalledTimes(1)
  })
})
