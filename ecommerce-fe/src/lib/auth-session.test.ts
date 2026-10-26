import { beforeEach, describe, expect, it, vi } from "vitest"
import {
  advanceSessionEpoch,
  clearAccessToken,
  getAccessToken,
  runSingleFlightRefresh,
  setAccessToken,
  subscribeAccessToken,
} from "./auth-session"

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

  it("notifies subscribers when the access token changes or clears and stops after unsubscribe", () => {
    const listener = vi.fn()
    const unsubscribe = subscribeAccessToken(listener)

    setAccessToken("jwt-1")
    setAccessToken("jwt-1")
    setAccessToken("jwt-2")
    clearAccessToken()
    clearAccessToken()
    unsubscribe()
    setAccessToken("jwt-3")

    expect(listener.mock.calls).toEqual([["jwt-1"], ["jwt-2"], [null]])
  })

  it("rejects an in-flight refresh when the session epoch advances before resolution", async () => {
    let resolveRefresh!: (token: string) => void
    const inFlight = runSingleFlightRefresh(
      () => new Promise<string>((resolve) => {
        resolveRefresh = resolve
      })
    )

    advanceSessionEpoch()
    resolveRefresh("stale-jwt")

    await expect(inFlight).rejects.toThrow(/session changed/i)
  })
})
