import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, describe, expect, it, vi } from "vitest"
import * as api from "@/lib/api"
import { clearAccessToken } from "@/lib/auth-session"
import { AuthProvider } from "@/context/auth-provider"
import { RequireAdminAuth } from "@/routes/require-admin-auth"
import { LoginPage } from "./login-page"

describe("LoginPage & RequireAdminAuth", () => {
  afterEach(() => {
    clearAccessToken()
    vi.restoreAllMocks()
    vi.unstubAllEnvs()
  })

  it("submits credentials, restores effective permissions, and navigates into the console", async () => {
    vi.spyOn(api, "refreshAccessToken").mockRejectedValueOnce(new Error("No session"))
    vi.spyOn(api, "login").mockResolvedValue({
      accessToken: "admin-jwt",
      tokenType: "Bearer",
      expiresIn: 900,
    })
    vi.spyOn(api, "fetchCurrentUser").mockResolvedValue({
      userId: "u-1",
      email: "admin@sshop.vn",
      fullName: "Admin DreamShop",
      roles: ["ADMIN"],
    })
    vi.spyOn(api, "fetchMyPermissions").mockResolvedValue(["product.view", "inventory.view"])

    render(
      <MemoryRouter initialEntries={["/login"]}>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route element={<RequireAdminAuth />}>
              <Route path="/" element={<div>Bảng điều khiển DreamShop</div>} />
            </Route>
          </Routes>
        </AuthProvider>
      </MemoryRouter>
    )

    const emailInput = await screen.findByLabelText(/email/i)
    const passwordInput = screen.getByLabelText(/mật khẩu/i)
    await userEvent.type(emailInput, "admin@sshop.vn")
    await userEvent.type(passwordInput, "AdminPassword123!")
    await userEvent.click(screen.getByRole("button", { name: /đăng nhập/i }))

    await waitFor(() => expect(screen.getByText("Bảng điều khiển DreamShop")).toBeInTheDocument())
  })

  it("renders access-denied screen with switch-account action when signed-in account lacks admin permissions", async () => {
    vi.spyOn(api, "refreshAccessToken").mockResolvedValue("cust-jwt")
    vi.spyOn(api, "fetchCurrentUser").mockResolvedValue({
      userId: "u-2",
      email: "buyer@sshop.vn",
      fullName: "Người mua",
      roles: ["CUSTOMER"],
    })
    vi.spyOn(api, "fetchMyPermissions").mockResolvedValue(["profile.view"])
    const logoutSpy = vi.spyOn(api, "logoutRequest").mockResolvedValue(undefined)

    render(
      <MemoryRouter initialEntries={["/"]}>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<div>Trang đăng nhập</div>} />
            <Route element={<RequireAdminAuth />}>
              <Route path="/" element={<div>Bảng điều khiển DreamShop</div>} />
            </Route>
          </Routes>
        </AuthProvider>
      </MemoryRouter>
    )

    expect(await screen.findByText(/buyer@sshop\.vn/i)).toBeInTheDocument()
    const switchBtn = screen.getByRole("button", { name: /đăng xuất|đổi tài khoản/i })
    await userEvent.click(switchBtn)
    expect(logoutSpy).toHaveBeenCalled()
    await waitFor(() => expect(screen.getByText("Trang đăng nhập")).toBeInTheDocument())
  })

  it("prefills admin credentials in development mode", async () => {
    vi.stubEnv("MODE", "development")
    vi.spyOn(api, "refreshAccessToken").mockRejectedValueOnce(new Error("No session"))
    const loginSpy = vi.spyOn(api, "login").mockResolvedValue({
      accessToken: "admin-jwt",
      tokenType: "Bearer",
      expiresIn: 900,
    })
    vi.spyOn(api, "fetchCurrentUser").mockResolvedValue({
      userId: "u-1",
      email: "admin@example.com",
      fullName: "Admin DreamShop",
      roles: ["SUPER_ADMIN"],
    })
    vi.spyOn(api, "fetchMyPermissions").mockResolvedValue(["product.view"])

    render(
      <MemoryRouter initialEntries={["/login"]}>
        <AuthProvider>
          <LoginPage />
        </AuthProvider>
      </MemoryRouter>
    )

    expect(await screen.findByLabelText(/email/i)).toHaveValue("admin@example.com")
    expect(screen.getByLabelText(/mật khẩu/i)).toHaveValue("AdminPassword123!")

    await userEvent.click(screen.getByRole("button", { name: /đăng nhập vào trung tâm/i }))

    expect(loginSpy).toHaveBeenCalledWith({
      email: "admin@example.com",
      password: "AdminPassword123!",
    })
  })

  it("switches credentials between development accounts", async () => {
    vi.stubEnv("MODE", "development")
    vi.spyOn(api, "refreshAccessToken").mockRejectedValueOnce(new Error("No session"))

    render(
      <MemoryRouter initialEntries={["/login"]}>
        <AuthProvider>
          <LoginPage />
        </AuthProvider>
      </MemoryRouter>
    )

    const emailInput = await screen.findByLabelText(/email/i)
    const passwordInput = screen.getByLabelText(/mật khẩu/i)

    expect(emailInput).toHaveValue("admin@example.com")
    expect(passwordInput).toHaveValue("AdminPassword123!")

    await userEvent.click(screen.getByRole("button", { name: "Customer" }))
    expect(emailInput).toHaveValue("customer1@gmail.com")
    expect(passwordInput).toHaveValue("123456123456")

    await userEvent.click(screen.getByRole("button", { name: "Admin" }))
    expect(emailInput).toHaveValue("admin@example.com")
    expect(passwordInput).toHaveValue("AdminPassword123!")
  })

  it("omits development credentials and preset controls in production", async () => {
    vi.stubEnv("MODE", "production")
    vi.spyOn(api, "refreshAccessToken").mockRejectedValueOnce(new Error("No session"))

    render(
      <MemoryRouter initialEntries={["/login"]}>
        <AuthProvider>
          <LoginPage />
        </AuthProvider>
      </MemoryRouter>
    )

    expect(await screen.findByLabelText(/email/i)).toHaveValue("")
    expect(screen.getByLabelText(/mật khẩu/i)).toHaveValue("")
    expect(screen.queryByRole("button", { name: "Admin" })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Customer" })).not.toBeInTheDocument()
  })
})
