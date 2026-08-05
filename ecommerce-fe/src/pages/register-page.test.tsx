import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { AuthContext } from "@/context/auth-context"
import RegisterPage from "./register-page"

function renderRegister(registerMock = vi.fn().mockResolvedValue(undefined)) {
  const value = {
    token: null,
    isAuthenticated: false,
    login: vi.fn(),
    register: registerMock,
    logout: vi.fn(),
  }
  return {
    registerMock,
    ...render(
      <AuthContext value={value}>
        <MemoryRouter>
          <RegisterPage />
        </MemoryRouter>
      </AuthContext>
    ),
  }
}

describe("RegisterPage", () => {
  it("rejects passwords shorter than 12 characters", async () => {
    const { registerMock } = renderRegister()
    await userEvent.type(screen.getByLabelText("Họ và tên"), "Nguyen Van A")
    await userEvent.type(screen.getByLabelText("Email"), "customer@example.com")
    await userEvent.type(screen.getByLabelText("Mật khẩu"), "short")
    await userEvent.type(screen.getByLabelText("Xác nhận mật khẩu"), "short")
    await userEvent.click(screen.getByRole("button", { name: /đăng ký/i }))

    expect(registerMock).not.toHaveBeenCalled()
    expect(await screen.findByRole("alert")).toHaveTextContent("12 ký tự")
  })

  it("submits the full payload when validation passes", async () => {
    const { registerMock } = renderRegister()
    await userEvent.type(screen.getByLabelText("Họ và tên"), "Nguyen Van A")
    await userEvent.type(screen.getByLabelText("Email"), "customer@example.com")
    await userEvent.type(screen.getByLabelText("Mật khẩu"), "SecurePassword123!")
    await userEvent.type(screen.getByLabelText("Xác nhận mật khẩu"), "SecurePassword123!")
    await userEvent.click(screen.getByRole("button", { name: /đăng ký/i }))

    expect(registerMock).toHaveBeenCalledWith({
      email: "customer@example.com",
      password: "SecurePassword123!",
      full_name: "Nguyen Van A",
    })
  })
})
