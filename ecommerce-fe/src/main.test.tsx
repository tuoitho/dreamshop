import { act, screen } from "@testing-library/react"
import { toast } from "sonner"
import { afterEach, describe, expect, it, vi } from "vitest"

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
    refreshAccessToken: vi.fn().mockRejectedValue(new Error("Unauthenticated")),
    searchProducts: vi.fn().mockResolvedValue({
      items: [],
      page: 1,
      pageSize: 20,
      totalItems: 0,
      totalPages: 0,
    }),
  }
})

describe("application root toaster", () => {
  afterEach(() => {
    act(() => {
      toast.dismiss()
    })
    document.body.innerHTML = ""
  })

  it("renders toast notifications at the top-center position", { timeout: 15000 }, async () => {
    document.body.innerHTML = '<div id="root"></div>'

    await act(async () => {
      await import("./main")
    })

    act(() => {
      toast.success("Đã lưu thay đổi")
    })

    const notification = await screen.findByText("Đã lưu thay đổi")
    const toaster = notification.closest("[data-sonner-toaster]")

    expect(toaster).not.toBeNull()
    expect(toaster).toHaveAttribute("data-y-position", "top")
    expect(toaster).toHaveAttribute("data-x-position", "center")
  })
})
