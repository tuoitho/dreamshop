import { describe, it, expect, vi, beforeEach } from "vitest"
import { AxiosError, AxiosHeaders } from "axios"
import { toast } from "sonner"
import { ApiClientError, fetchHealth, http, login, logoutRequest, updateCurrentUser } from "./api"
import { setAccessToken, clearAccessToken } from "./auth-session"

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

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
       config: { headers: new AxiosHeaders(), method: "post" } as never,
    }
  )
}

describe("api client", () => {
  beforeEach(() => {
    clearAccessToken()
    vi.clearAllMocks()
    delete http.defaults.adapter
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

  it("uses credentials for cookie-backed refresh", () => {
    expect(http.defaults.withCredentials).toBe(true)
  })

  it("translates an ApiError envelope into ApiClientError", async () => {
    vi.spyOn(http, "post").mockRejectedValueOnce(
      axiosError(409, { code: "EMAIL_ALREADY_REGISTERED", message: "Email is already registered", traceId: "t-1" })
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

  it("carries field-level validation errors from the ApiError envelope", async () => {
    vi.spyOn(http, "patch").mockRejectedValueOnce(
      axiosError(400, {
        code: "VALIDATION_FAILED",
        message: "Request validation failed",
        errors: {
          phone: "must be a valid Vietnamese mobile number",
          dateOfBirth: "must be a past date",
        },
        traceId: "t-1",
      })
    )
    const err = await updateCurrentUser({ phone: "123" }).catch((e) => e)
    expect(err).toBeInstanceOf(ApiClientError)
    expect(err.code).toBe("VALIDATION_FAILED")
    expect(err.errors).toEqual({
      phone: "must be a valid Vietnamese mobile number",
      dateOfBirth: "must be a past date",
    })
  })

  it("shows the API success message for a mutation", async () => {
    http.defaults.adapter = vi.fn(async (config) => ({
      data: { message: "Logged out successfully" },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    })) as never

    await logoutRequest()

    expect(toast.success).toHaveBeenCalledWith("Logged out successfully")
    expect(toast.error).not.toHaveBeenCalled()
  })

  it("shows the API error message for a failed mutation", async () => {
    http.defaults.adapter = vi.fn(async (config) => {
      throw new AxiosError(
        "Request failed",
        "ERR_BAD_REQUEST",
        config,
        undefined,
        {
          status: 400,
          data: { message: "Request validation failed" },
          statusText: "Bad Request",
          headers: new AxiosHeaders(),
          config,
        }
      )
    }) as never

    await logoutRequest().catch(() => undefined)

    expect(toast.error).toHaveBeenCalledWith("Request validation failed")
    expect(toast.success).not.toHaveBeenCalled()
  })

  it("does not show a toast for GET requests", async () => {
    http.defaults.adapter = vi.fn(async (config) => ({
      data: { status: "UP", message: "Health loaded" },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    })) as never

    await fetchHealth()

    expect(toast.success).not.toHaveBeenCalled()
    expect(toast.error).not.toHaveBeenCalled()
  })
})
