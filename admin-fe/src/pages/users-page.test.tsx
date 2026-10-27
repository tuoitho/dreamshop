import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, describe, expect, it, vi } from "vitest"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import * as api from "@/lib/api"
import type { AdminUser, AdminUserPage, RbacItem } from "@/lib/types"
import { UsersPage } from "./users-page"

function makeAuth(permissions: string[]): AuthContextValue {
  const permSet = new Set(permissions)
  return {
    status: "authenticated",
    user: {
      userId: "u-super",
      email: "root@sshop.vn",
      fullName: "Super Admin",
      roles: ["SUPER_ADMIN"],
    },
    permissions: permSet,
    hasAnyAdminAccess: true,
    can: (perm: string) => permSet.has(perm),
    login: vi.fn(),
    logout: vi.fn(),
    refreshPermissions: vi.fn(),
  }
}

const sampleUsersPage: AdminUserPage = {
  items: [
    {
      userId: "u-ops-1",
      email: "linh.nguyen@sshop.vn",
      fullName: "Nguyễn Khánh Linh",
      status: "ACTIVE",
      roles: ["CUSTOMER"],
      createdAt: "2026-09-10T08:00:00Z",
      updatedAt: "2026-10-20T09:30:00Z",
    },
    {
      userId: "u-super",
      email: "root@sshop.vn",
      fullName: "Super Admin",
      status: "ACTIVE",
      roles: ["SUPER_ADMIN"],
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-10-01T00:00:00Z",
    },
  ],
  page: 1,
  pageSize: 15,
  totalItems: 2,
  totalPages: 1,
  activeCount: 18,
  disabledCount: 3,
}

const sampleRoles: RbacItem[] = [
  {
    name: "SUPER_ADMIN",
    type: "ROLE",
    description: "Quản trị tối cao",
    systemItem: true,
    directChildren: ["ADMIN"],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
  {
    name: "ADMIN",
    type: "ROLE",
    description: "Quản trị viên vận hành",
    systemItem: true,
    directChildren: ["product.view"],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
  {
    name: "CUSTOMER",
    type: "ROLE",
    description: "Khách mua hàng",
    systemItem: true,
    directChildren: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
]

describe("UsersPage & UserDetailSheet", () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it("filters users by status tab with global counts and searches by keyword", async () => {
    const listSpy = vi.spyOn(api, "listAdminUsers").mockResolvedValue(sampleUsersPage)
    vi.spyOn(api, "listRbacItems").mockResolvedValue(sampleRoles)

    render(
      <MemoryRouter>
        <AuthContext.Provider
          value={makeAuth(["user.view", "user.status.update", "user.role.assign", "rbac.view"])}
        >
          <UsersPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    expect(await screen.findByText("Nguyễn Khánh Linh")).toBeInTheDocument()
    expect(screen.getByText("18")).toBeInTheDocument()
    expect(screen.getByText("3")).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: /đã khóa/i }))
    await waitFor(() =>
      expect(listSpy).toHaveBeenCalledWith(
        expect.objectContaining({ status: "DISABLED", page: 1 }),
        expect.anything()
      )
    )

    const searchInput = screen.getByPlaceholderText(/tìm theo email/i)
    await userEvent.type(searchInput, "linh.nguyen")
    await userEvent.click(screen.getByRole("button", { name: /^lọc$/i }))
    await waitFor(() =>
      expect(listSpy).toHaveBeenCalledWith(
        expect.objectContaining({ q: "linh.nguyen" }),
        expect.anything()
      )
    )
  })

  it("toggles account status with confirmation, assigns/revokes roles, and surfaces protection errors", async () => {
    vi.spyOn(api, "listAdminUsers").mockResolvedValue(sampleUsersPage)
    vi.spyOn(api, "listRbacItems").mockResolvedValue(sampleRoles)

    const targetUser: AdminUser = sampleUsersPage.items[0]!
    vi.spyOn(api, "getAdminUser").mockResolvedValue(targetUser)

    const statusSpy = vi
      .spyOn(api, "updateAdminUserStatus")
      .mockRejectedValueOnce(
        new api.ApiClientError(
          403,
          "Không thể tự vô hiệu hóa tài khoản đang đăng nhập.",
          "SELF_DISABLE_FORBIDDEN"
        )
      )
      .mockResolvedValueOnce({
        ...targetUser,
        status: "DISABLED",
      })

    const assignSpy = vi.spyOn(api, "assignAdminUserRole").mockResolvedValue({
      ...targetUser,
      roles: ["CUSTOMER", "ADMIN"],
    })
    const revokeSpy = vi.spyOn(api, "revokeAdminUserRole").mockResolvedValue({
      ...targetUser,
      roles: ["ADMIN"],
    })

    render(
      <MemoryRouter>
        <AuthContext.Provider
          value={makeAuth(["user.view", "user.status.update", "user.role.assign", "rbac.view"])}
        >
          <UsersPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    await screen.findByText("Nguyễn Khánh Linh")
    await userEvent.click(screen.getByRole("button", { name: /chi tiết nguyễn khánh linh/i }))

    expect(await screen.findByText("Hồ sơ & Quyền hạn tài khoản")).toBeInTheDocument()

    // Trigger status toggle -> confirm dialog -> first attempt fails with SELF_DISABLE_FORBIDDEN
    await userEvent.click(await screen.findByRole("button", { name: /khóa tài khoản/i }))
    await userEvent.click(await screen.findByRole("button", { name: /xác nhận khóa/i }))

    await waitFor(() => expect(statusSpy).toHaveBeenCalledWith("u-ops-1", "DISABLED"))
    expect(
      await screen.findByText(/không thể tự vô hiệu hóa tài khoản đang đăng nhập/i)
    ).toBeInTheDocument()

    // Assign ADMIN role and revoke CUSTOMER role
    await userEvent.click(screen.getByRole("button", { name: /gán vai trò admin/i }))
    await waitFor(() => expect(assignSpy).toHaveBeenCalledWith("u-ops-1", "ADMIN"))

    await userEvent.click(screen.getByRole("button", { name: /thu hồi vai trò customer/i }))
    await waitFor(() => expect(revokeSpy).toHaveBeenCalledWith("u-ops-1", "CUSTOMER"))
  })

  it("disables self-lock button and refreshes permissions when mutating current operator roles", async () => {
    const authValue = makeAuth([
      "user.view",
      "user.status.update",
      "user.role.assign",
      "rbac.view",
    ])
    vi.spyOn(api, "listAdminUsers").mockResolvedValue(sampleUsersPage)
    vi.spyOn(api, "listRbacItems").mockResolvedValue(sampleRoles)
    vi.spyOn(api, "getAdminUser").mockResolvedValue(sampleUsersPage.items[1]!)
    vi.spyOn(api, "assignAdminUserRole").mockResolvedValue({
      ...sampleUsersPage.items[1]!,
      roles: ["SUPER_ADMIN", "ADMIN"],
    })

    render(
      <MemoryRouter>
        <AuthContext.Provider value={authValue}>
          <UsersPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    await screen.findByText("Super Admin")
    await userEvent.click(screen.getByRole("button", { name: /chi tiết super admin/i }))
    expect(await screen.findByText("Hồ sơ & Quyền hạn tài khoản")).toBeInTheDocument()

    expect(screen.getByRole("button", { name: /khóa tài khoản/i })).toBeDisabled()

    await userEvent.click(screen.getByRole("button", { name: /gán vai trò admin/i }))
    await waitFor(() => expect(authValue.refreshPermissions).toHaveBeenCalledTimes(1))
  })
})
