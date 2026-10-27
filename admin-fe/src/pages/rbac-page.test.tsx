import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, describe, expect, it, vi } from "vitest"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import * as api from "@/lib/api"
import type { RbacItem } from "@/lib/types"
import { RbacPage } from "./rbac-page"

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

const sampleRbacItems: RbacItem[] = [
  {
    name: "SUPER_ADMIN",
    type: "ROLE",
    description: "Toàn quyền điều hành hệ thống",
    systemItem: true,
    directChildren: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
  {
    name: "ADMIN",
    type: "ROLE",
    description: "Quản trị viên thương mại",
    systemItem: true,
    directChildren: ["CATALOG_EDITOR"],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
  {
    name: "CATALOG_EDITOR",
    type: "ROLE",
    description: "Biên tập viên danh mục",
    systemItem: false,
    directChildren: ["product.view"],
    createdAt: "2026-10-01T00:00:00Z",
    updatedAt: "2026-10-01T00:00:00Z",
  },
  {
    name: "product.view",
    type: "PERMISSION",
    description: "Xem sản phẩm quản trị",
    systemItem: true,
    directChildren: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
  {
    name: "inventory.view",
    type: "PERMISSION",
    description: "Xem tồn kho",
    systemItem: true,
    directChildren: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
]

describe("RbacPage", () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it("displays system locks, distinguishes direct vs inherited permissions on roles, and keeps PERMISSION and SUPER_ADMIN read-only", async () => {
    vi.spyOn(api, "listRbacItems").mockResolvedValue(sampleRbacItems)
    const addChildSpy = vi
      .spyOn(api, "addRbacChild")
      .mockRejectedValueOnce(
        new api.ApiClientError(409, "Phát hiện vòng lặp kế thừa phân quyền.", "RBAC_HIERARCHY_CONFLICT")
      )
      .mockResolvedValueOnce(undefined)
    const removeChildSpy = vi.spyOn(api, "removeRbacChild").mockResolvedValue(undefined)

    render(
      <MemoryRouter>
        <AuthContext.Provider value={makeAuth(["rbac.view", "rbac.manage"])}>
          <RbacPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    // Default selected role is ADMIN: CATALOG_EDITOR is direct child, product.view is inherited via CATALOG_EDITOR
    expect((await screen.findAllByText("Quản trị viên thương mại")).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/hệ thống/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/kế thừa qua/i).length).toBe(1)

    // SUPER_ADMIN is excluded from attachable child options
    const childSelect = screen.getByLabelText(/chọn mục con để gắn/i)
    expect(within(childSelect).queryByRole("option", { name: /super_admin/i })).not.toBeInTheDocument()

    // Attach child -> first attempt hits 409 cycle error
    await userEvent.selectOptions(childSelect, "inventory.view")
    await userEvent.click(screen.getByRole("button", { name: /gắn mục con/i }))

    await waitFor(() => expect(addChildSpy).toHaveBeenCalledWith("ADMIN", "inventory.view"))
    expect(await screen.findByText(/phát hiện vòng lặp kế thừa phân quyền/i)).toBeInTheDocument()

    // Detach direct child CATALOG_EDITOR from ADMIN
    await userEvent.click(screen.getByRole("button", { name: /gỡ liên kết catalog_editor/i }))
    await waitFor(() => expect(removeChildSpy).toHaveBeenCalledWith("ADMIN", "CATALOG_EDITOR"))

    // Selecting a fixed PERMISSION shows read-only dictionary view without attach/edit/delete controls
    const masterList = screen.getByRole("listbox", { name: /danh sách mục rbac/i })
    await userEvent.click(within(masterList).getByRole("option", { name: /product\.view/i }))
    expect(screen.queryByLabelText(/chọn mục con để gắn/i)).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /sửa thông tin/i })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /xóa vai trò tùy chỉnh/i })).not.toBeInTheDocument()

    // Selecting SUPER_ADMIN also hides child mutation form because SUPER_ADMIN holds full wildcard access
    await userEvent.click(within(masterList).getByRole("option", { name: /super_admin/i }))
    expect(screen.queryByLabelText(/chọn mục con để gắn/i)).not.toBeInTheDocument()
  })

  it("creates, updates, and deletes a custom ROLE without offering PERMISSION creation", async () => {
    vi.spyOn(api, "listRbacItems").mockResolvedValue(sampleRbacItems)
    const createSpy = vi.spyOn(api, "createRbacItem").mockResolvedValue({
      name: "WAREHOUSE_LEAD",
      type: "ROLE",
      description: "Trưởng ca kho",
      systemItem: false,
      directChildren: [],
      createdAt: "2026-10-27T00:00:00Z",
      updatedAt: "2026-10-27T00:00:00Z",
    })
    const updateSpy = vi.spyOn(api, "updateRbacItem").mockResolvedValue({
      ...sampleRbacItems[2]!,
      description: "Biên tập viên nội dung sản phẩm",
    })
    const deleteSpy = vi.spyOn(api, "deleteRbacItem").mockResolvedValue(undefined)

    render(
      <MemoryRouter>
        <AuthContext.Provider value={makeAuth(["rbac.view", "rbac.manage"])}>
          <RbacPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    expect((await screen.findAllByText("Quản trị viên thương mại")).length).toBeGreaterThan(0)

    // Create custom role (no PERMISSION type selector exists)
    await userEvent.click(screen.getByRole("button", { name: /^tạo vai trò$/i }))
    expect(screen.queryByLabelText(/loại định nghĩa/i)).not.toBeInTheDocument()
    await userEvent.type(await screen.findByLabelText(/mã vai trò/i), "WAREHOUSE_LEAD")
    await userEvent.type(screen.getByLabelText(/mô tả chức năng/i), "Trưởng ca kho")
    await userEvent.click(screen.getByRole("button", { name: /lưu vai trò/i }))

    await waitFor(() =>
      expect(createSpy).toHaveBeenCalledWith({
        name: "WAREHOUSE_LEAD",
        type: "ROLE",
        description: "Trưởng ca kho",
      })
    )

    // Select custom role CATALOG_EDITOR -> edit description -> delete custom role
    const masterList = screen.getByRole("listbox", { name: /danh sách mục rbac/i })
    await userEvent.click(within(masterList).getByRole("option", { name: /catalog_editor/i }))
    await userEvent.click(screen.getByRole("button", { name: /sửa thông tin/i }))

    const descInput = await screen.findByLabelText(/mô tả chức năng/i)
    await userEvent.clear(descInput)
    await userEvent.type(descInput, "Biên tập viên nội dung sản phẩm")
    await userEvent.click(screen.getByRole("button", { name: /lưu vai trò/i }))

    await waitFor(() =>
      expect(updateSpy).toHaveBeenCalledWith("CATALOG_EDITOR", {
        name: "CATALOG_EDITOR",
        description: "Biên tập viên nội dung sản phẩm",
      })
    )

    await userEvent.click(screen.getByRole("button", { name: /xóa vai trò tùy chỉnh/i }))
    await userEvent.click(await screen.findByRole("button", { name: /xác nhận xóa/i }))
    await waitFor(() => expect(deleteSpy).toHaveBeenCalledWith("CATALOG_EDITOR"))
  })
})
