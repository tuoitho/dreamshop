import { expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { AuthContext } from "@/context/auth-context"
import ProfilePage from "./profile-page"

it("updates the displayed profile name", async () => {
  const updateProfile = vi.fn().mockResolvedValue(undefined)
  render(<AuthContext value={{ token: "jwt", ready: true, isAuthenticated: true, user: { user_id: "u1", email: "user@example.com", full_name: "User", roles: ["CUSTOMER"] }, login: vi.fn(), register: vi.fn(), updateProfile, logout: vi.fn() }}><MemoryRouter><ProfilePage /></MemoryRouter></AuthContext>)
  await userEvent.clear(screen.getByLabelText("Họ và tên"))
  await userEvent.type(screen.getByLabelText("Họ và tên"), "Updated Name")
  await userEvent.click(screen.getByRole("button", { name: "Lưu thay đổi" }))
  expect(updateProfile).toHaveBeenCalledWith({ full_name: "Updated Name" })
})
