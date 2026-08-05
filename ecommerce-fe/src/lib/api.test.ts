import { describe, it, expect, vi, beforeEach } from "vitest"
import { AxiosError, AxiosHeaders } from "axios"
import { ApiClientError, fetchHealth, http, login } from "./api"
import { setAccessToken, clearAccessToken } from "./token-storage"

function axiosError(status: number, data: unknown): AxiosError {
  return new AxiosError(
    "Request failed",
    "ERR_BAD_REQUEST",
    undefined,
    undefined,
    {
      status,
      data,
      statusText: "",
      headers: new AxiosHeaders(),
      config: { headers: new AxiosHeaders() } as never,
    }
  )
}

describe("api client", () => {
  beforeEach(() => {
    clearAccessToken()
  })

  it("attaches the Bearer token when one is stored", async () => {
    setAccessToken("jwt-xyz")
    const adapter = vi.fn(async (config) => {
      throw new AxiosError("network error", "ERR_NETWORK", config)
    })
    http.defaults.adapter = adapter as never
    await fetchHealth().catch(() => undefined)
    const config = adapter.mock.calls[0][0] as { headers?: { Authorization?: string } }
    expect(config.headers?.Authorization).toBe("Bearer jwt-xyz")
    delete http.defaults.adapter
  })

  it("translates an ApiError envelope into ApiClientError", async () => {
    vi.spyOn(http, "post").mockRejectedValueOnce(
      axiosError(409, { code: "EMAIL_ALREADY_REGISTERED", message: "Email is already registered", trace_id: "t-1" })
    )
    const err = await login({ email: "a@b.com", password: "password-123" }).catch((e) => e)
    expect(err).toBeInstanceOf(ApiClientError)
    expect(err.status).toBe(409)
    expect(err.code).toBe("EMAIL_ALREADY_REGISTERED")
    expect(err.message).toBe("Email is already registered")
    expect(err.traceId).toBe("t-1")
  })

  it("falls back to the transport message when the body is not an ApiError", async () => {
    vi.spyOn(http, "post").mockRejectedValueOnce(axiosError(502, "<html>bad gateway</html>"))
    const err = await login({ email: "a@b.com", password: "password-123" }).catch((e) => e)
    expect(err).toBeInstanceOf(ApiClientError)
    expect(err.status).toBe(502)
  })
})
