import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { describe, expect, it, vi } from "vitest"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import { AdminShell } from "./admin-shell"

function makeAuth(permissions: string[], logout = vi.fn().mockResolvedValue(undefined)): AuthContextValue {
  const permSet = new Set(permissions)
  return {
    status: "authenticated",
    user: {
      userId: "u-1",
      email: "admin@example.com",
      fullName: "Nguyễn Minh Anh",
      roles: ["ADMIN"],
    },
    permissions: permSet,
    hasAnyAdminAccess: true,
    can: (perm: string) => permSet.has(perm),
    login: vi.fn(),
    logout,
    refreshPermissions: vi.fn(),
  }
}

function renderShell(auth: AuthContextValue, initialPath = "/") {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <AuthContext.Provider value={auth}>
        <Routes>
          <Route element={<AdminShell />}>
            <Route path="/" element={<div>Trang Tổng Quan</div>} />
            <Route path="/orders" element={<div>Trang Đơn Hàng</div>} />
            <Route path="/products" element={<div>Trang Sản Phẩm</div>} />
          </Route>
          <Route path="/login" element={<div>Trang Đăng Nhập</div>} />
        </Routes>
      </AuthContext.Provider>
    </MemoryRouter>
  )
}

describe("AdminShell", () => {
  it("toggles desktop sidebar collapse state", async () => {
    renderShell(makeAuth(["product.view", "order.admin.view"]))

    const sidebar = screen.getByLabelText("Điều hướng quản trị")
    expect(sidebar).toHaveAttribute("data-collapsed", "false")

    await userEvent.click(screen.getByRole("button", { name: "Thu gọn menu" }))
    expect(sidebar).toHaveAttribute("data-collapsed", "true")

    await userEvent.click(screen.getByRole("button", { name: "Mở rộng menu" }))
    expect(sidebar).toHaveAttribute("data-collapsed", "false")
  })

  it("gates navigation items by permission and navigates via Quick Jump search", async () => {
    renderShell(makeAuth(["order.admin.view"]))

    const sidebar = screen.getByLabelText("Điều hướng quản trị")
    expect(within(sidebar).getByRole("link", { name: "Đơn hàng" })).toBeInTheDocument()
    expect(within(sidebar).queryByRole("link", { name: "Sản phẩm" })).not.toBeInTheDocument()
    expect(within(sidebar).queryByRole("link", { name: "Phân quyền RBAC" })).not.toBeInTheDocument()

    const searchInput = screen.getByPlaceholderText(/tìm nhanh trang/i)
    await userEvent.click(searchInput)
    await userEvent.type(searchInput, "đơn")

    const option = await screen.findByRole("button", { name: /đơn hàng/i })
    await userEvent.click(option)

    expect(await screen.findByText("Trang Đơn Hàng")).toBeInTheDocument()
  })

  it("displays operator profile and invokes logout", async () => {
    const logoutSpy = vi.fn().mockResolvedValue(undefined)
    renderShell(makeAuth(["product.view"], logoutSpy))

    expect(screen.getByText("Nguyễn Minh Anh")).toBeInTheDocument()
    await userEvent.click(screen.getByRole("button", { name: "Đăng xuất" }))

    expect(logoutSpy).toHaveBeenCalledTimes(1)
    expect(await screen.findByText("Trang Đăng Nhập")).toBeInTheDocument()
  })
})
