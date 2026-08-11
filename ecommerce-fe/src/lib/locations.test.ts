import { describe, expect, it, vi } from "vitest"
import { AxiosError, AxiosHeaders } from "axios"
import { ApiClientError, http } from "./api"
import { fetchProvinces, fetchWards } from "./locations"

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

describe("locations client", () => {
  it("loads province code-name pairs from the locations endpoint", async () => {
    const get = vi
      .spyOn(http, "get")
      .mockResolvedValueOnce({ data: [{ code: "79", name: "TP. Hồ Chí Minh" }] } as never)

    await expect(fetchProvinces()).resolves.toEqual([{ code: "79", name: "TP. Hồ Chí Minh" }])
    expect(get).toHaveBeenCalledWith("/locations/provinces")
  })

  it("loads ward code-name pairs for the selected province", async () => {
    const get = vi
      .spyOn(http, "get")
      .mockResolvedValueOnce({ data: [{ code: "26740", name: "Phường Bến Nghé" }] } as never)

    await expect(fetchWards("79")).resolves.toEqual([{ code: "26740", name: "Phường Bến Nghé" }])
    expect(get).toHaveBeenCalledWith("/locations/provinces/79/wards")
  })

  it("translates a failed province request into ApiClientError", async () => {
    vi.spyOn(http, "get").mockRejectedValueOnce(
      axiosError(500, { code: "INTERNAL", message: "boom", traceId: "t-1" })
    )

    const err = await fetchProvinces().catch((e) => e)
    expect(err).toBeInstanceOf(ApiClientError)
    expect(err.status).toBe(500)
    expect(err.code).toBe("INTERNAL")
  })

  it("translates a failed ward request into ApiClientError", async () => {
    vi.spyOn(http, "get").mockRejectedValueOnce(
      axiosError(404, { code: "PROVINCE_NOT_FOUND", message: "unknown province", traceId: "t-2" })
    )

    const err = await fetchWards("999").catch((e) => e)
    expect(err).toBeInstanceOf(ApiClientError)
    expect(err.status).toBe(404)
    expect(err.code).toBe("PROVINCE_NOT_FOUND")
  })
})
