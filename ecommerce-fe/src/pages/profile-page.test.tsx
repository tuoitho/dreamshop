import { beforeAll, beforeEach, expect, it, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { AuthContext } from "@/context/auth-context"
import { ApiClientError } from "@/lib/api"
import { fetchProvinces, fetchWards } from "@/lib/locations"
import type { CurrentUser, UpdateProfileRequest } from "@/lib/types"
import ProfilePage from "./profile-page"

vi.mock("@/lib/locations", () => ({
  fetchProvinces: vi.fn(),
  fetchWards: vi.fn(),
}))

const PROVINCES = [
  { code: "01", name: "Hà Nội" },
  { code: "48", name: "Đà Nẵng" },
  { code: "79", name: "TP. Hồ Chí Minh" },
]

const WARDS_BY_PROVINCE: Record<string, { code: string; name: string }[]> = {
  "01": [{ code: "00101", name: "Phường Phúc Xá" }],
  "48": [{ code: "20491", name: "Phường Hải Châu 1" }],
  "79": [
    { code: "26740", name: "Phường Bến Nghé" },
    { code: "26734", name: "Phường Bến Thành" },
  ],
}

beforeAll(() => {
  window.HTMLElement.prototype.scrollIntoView = vi.fn()
  window.HTMLElement.prototype.hasPointerCapture = vi.fn()
  window.HTMLElement.prototype.releasePointerCapture = vi.fn()
})

beforeEach(() => {
  vi.mocked(fetchProvinces).mockReset()
  vi.mocked(fetchWards).mockReset()
  vi.mocked(fetchProvinces).mockResolvedValue(PROVINCES)
  vi.mocked(fetchWards).mockImplementation(async (code: string) => WARDS_BY_PROVINCE[code] ?? [])
})

const baseUser: CurrentUser = {
  userId: "u1",
  email: "user@example.com",
  fullName: "User",
  roles: ["CUSTOMER"],
  phone: null,
  dateOfBirth: null,
  gender: null,
  provinceCode: null,
  wardCode: null,
  addressLine: null,
}

function renderProfile(updateProfile = vi.fn().mockResolvedValue(undefined), user: CurrentUser = baseUser) {
  const value = {
    token: "jwt",
    user,
    ready: true,
    isAuthenticated: true,
    login: vi.fn(),
    register: vi.fn(),
    updateProfile,
    logout: vi.fn(),
  }
  const view = render(
    <AuthContext value={value}>
      <MemoryRouter>
        <ProfilePage />
      </MemoryRouter>
    </AuthContext>
  )
  return {
    updateProfile,
    setUser: (next: CurrentUser) => {
      view.rerender(
        <AuthContext value={{ ...value, user: next }}>
          <MemoryRouter>
            <ProfilePage />
          </MemoryRouter>
        </AuthContext>
      )
    },
    ...view,
  }
}

async function selectProvince(name: string) {
  await userEvent.click(screen.getByLabelText("Tỉnh/thành phố"))
  await userEvent.click(await screen.findByRole("option", { name }))
}

async function selectWard(name: string) {
  await userEvent.click(screen.getByLabelText("Phường/xã"))
  await userEvent.click(await screen.findByRole("option", { name }))
}

it("renders email as a readonly field", () => {
  renderProfile()

  const email = screen.getByLabelText("Email")
  expect(email).toHaveValue("user@example.com")
  expect(email).toHaveAttribute("readonly")
})

it("keeps the ward selector disabled until a province is chosen", async () => {
  renderProfile()

  expect(screen.getByLabelText("Phường/xã")).toBeDisabled()
  await selectProvince("TP. Hồ Chí Minh")
  expect(screen.getByLabelText("Phường/xã")).toBeEnabled()
})

it("submits the whole mutable profile when every field is filled", async () => {
  const { updateProfile } = renderProfile()

  await userEvent.clear(screen.getByLabelText("Họ và tên"))
  await userEvent.type(screen.getByLabelText("Họ và tên"), "Nguyen Van A")
  await userEvent.type(screen.getByLabelText("Số điện thoại"), "0901234567")
  await userEvent.type(screen.getByLabelText("Ngày sinh"), "1995-04-20")
  await userEvent.click(screen.getByLabelText("Giới tính"))
  await userEvent.click(await screen.findByRole("option", { name: "Nam" }))
  await selectProvince("TP. Hồ Chí Minh")
  await selectWard("Phường Bến Nghé")
  await userEvent.type(screen.getByLabelText("Địa chỉ chi tiết"), "12 Le Loi")
  await userEvent.click(screen.getByRole("button", { name: "Lưu thay đổi" }))

  expect(fetchWards).toHaveBeenCalledWith("79")
  expect(updateProfile).toHaveBeenCalledWith({
    fullName: "Nguyen Van A",
    phone: "0901234567",
    dateOfBirth: "1995-04-20",
    gender: "MALE",
    provinceCode: "79",
    wardCode: "26740",
    addressLine: "12 Le Loi",
  })
  const payload = updateProfile.mock.calls[0][0] as UpdateProfileRequest
  expect(payload).not.toHaveProperty("district")
  expect(payload).not.toHaveProperty("ward")
})

it("keeps gender restricted to the profile enum or the empty clear state", () => {
  const cleared: UpdateProfileRequest = {
    fullName: "User",
    phone: "",
    dateOfBirth: "",
    gender: "",
    provinceCode: "",
    wardCode: "",
    addressLine: "",
  }
  const selected: UpdateProfileRequest = { ...cleared, gender: "MALE" }
  // @ts-expect-error arbitrary strings are outside the gender contract
  const rejected: UpdateProfileRequest = { ...cleared, gender: "UNKNOWN" }
  expect(cleared.gender).toBe("")
  expect(selected.gender).toBe("MALE")
  expect(rejected.gender).toBe("UNKNOWN")
})

it("clears an existing gender and location back to the empty state", async () => {
  const { updateProfile } = renderProfile(vi.fn().mockResolvedValue(undefined), {
    ...baseUser,
    phone: "0901234567",
    gender: "MALE",
      provinceCode: "79",
      wardCode: "26740",
  })

  await userEvent.click(screen.getByRole("button", { name: "Xóa giới tính" }))
  await userEvent.click(screen.getByRole("button", { name: "Xóa tỉnh/thành phố" }))
  await userEvent.click(screen.getByRole("button", { name: "Lưu thay đổi" }))

  expect(updateProfile).toHaveBeenCalledWith({
      fullName: "User",
    phone: "0901234567",
      dateOfBirth: "",
    gender: "",
      provinceCode: "",
      wardCode: "",
      addressLine: "",
  })
})

it("resets the ward selection when the province changes", async () => {
  renderProfile()

  await selectProvince("TP. Hồ Chí Minh")
  await selectWard("Phường Bến Nghé")
  expect(screen.getByLabelText("Phường/xã")).toHaveTextContent("Phường Bến Nghé")

  await selectProvince("Hà Nội")
  expect(screen.getByLabelText("Phường/xã")).toHaveTextContent("Chọn phường/xã")
  expect(fetchWards).toHaveBeenCalledTimes(2)
  expect(fetchWards).toHaveBeenLastCalledWith("01")
})

it("shows a loading state while provinces are being fetched", async () => {
  let resolveProvinces!: (value: { code: string; name: string }[]) => void
  vi.mocked(fetchProvinces).mockReturnValueOnce(
    new Promise((resolve) => {
      resolveProvinces = resolve
    })
  )
  renderProfile()

  await userEvent.click(screen.getByLabelText("Tỉnh/thành phố"))
  expect(screen.getByText("Đang tải...")).toBeInTheDocument()

  resolveProvinces(PROVINCES)
  expect(await screen.findByRole("option", { name: "TP. Hồ Chí Minh" })).toBeInTheDocument()
})

it("shows a loading state while wards are being fetched", async () => {
  let resolveWards!: (value: { code: string; name: string }[]) => void
  vi.mocked(fetchWards).mockReturnValueOnce(
    new Promise((resolve) => {
      resolveWards = resolve
    })
  )
  renderProfile()

  await selectProvince("TP. Hồ Chí Minh")
  await userEvent.click(screen.getByLabelText("Phường/xã"))
  expect(screen.getByText("Đang tải...")).toBeInTheDocument()

  resolveWards(WARDS_BY_PROVINCE["79"])
  expect(await screen.findByRole("option", { name: "Phường Bến Nghé" })).toBeInTheDocument()
})

it("reports a province loading failure and retries", async () => {
  vi.mocked(fetchProvinces).mockRejectedValueOnce(new Error("network down"))
  renderProfile()

  await userEvent.click(screen.getByLabelText("Tỉnh/thành phố"))
  expect(await screen.findByText("Không tải được danh sách tỉnh/thành phố")).toBeInTheDocument()

  await userEvent.click(screen.getByRole("button", { name: "Thử lại" }))
  expect(await screen.findByRole("option", { name: "TP. Hồ Chí Minh" })).toBeInTheDocument()
  expect(screen.getByLabelText("Phường/xã")).toBeDisabled()
})

it("reports a ward loading failure and retries", async () => {
  vi.mocked(fetchWards).mockRejectedValueOnce(new Error("network down"))
  renderProfile()

  await selectProvince("TP. Hồ Chí Minh")
  await userEvent.click(screen.getByLabelText("Phường/xã"))
  expect(await screen.findByText("Không tải được danh sách phường/xã")).toBeInTheDocument()

  await userEvent.click(screen.getByRole("button", { name: "Thử lại" }))
  expect(await screen.findByRole("option", { name: "Phường Bến Nghé" })).toBeInTheDocument()
})

it("filters province options by the search term", async () => {
  renderProfile()

  await userEvent.click(screen.getByLabelText("Tỉnh/thành phố"))
  await userEvent.type(screen.getByRole("combobox", { name: "Tìm tỉnh/thành phố" }), "Hà N")
  expect(screen.getByRole("option", { name: "Hà Nội" })).toBeInTheDocument()
  expect(screen.queryByRole("option", { name: "TP. Hồ Chí Minh" })).not.toBeInTheDocument()
})

it("reflects a replacement user from the context after an update", async () => {
  const { setUser } = renderProfile()
  setUser({
    ...baseUser,
      fullName: "Server Name",
    phone: "0909999999",
      dateOfBirth: "1990-01-02",
    gender: "FEMALE",
      provinceCode: "48",
      wardCode: "20491",
      addressLine: "9 Bach Dang",
  })

  expect(await screen.findByLabelText("Họ và tên")).toHaveValue("Server Name")
  expect(screen.getByLabelText("Số điện thoại")).toHaveValue("0909999999")
  expect(screen.getByLabelText("Ngày sinh")).toHaveValue("1990-01-02")
  expect(screen.getByLabelText("Giới tính")).toHaveTextContent("Nữ")
  await waitFor(() => expect(screen.getByLabelText("Tỉnh/thành phố")).toHaveTextContent("Đà Nẵng"))
  await waitFor(() => expect(screen.getByLabelText("Phường/xã")).toHaveTextContent("Phường Hải Châu 1"))
  expect(screen.getByLabelText("Địa chỉ chi tiết")).toHaveValue("9 Bach Dang")
  expect(fetchWards).toHaveBeenCalledWith("48")
})

it("blocks submit and shows a field error for an invalid Vietnamese phone", async () => {
  const { updateProfile } = renderProfile()

  await userEvent.type(screen.getByLabelText("Số điện thoại"), "1234567890")
  await userEvent.click(screen.getByRole("button", { name: "Lưu thay đổi" }))

  expect(updateProfile).not.toHaveBeenCalled()
  expect(screen.getByText("Số điện thoại không hợp lệ")).toBeInTheDocument()
})

it("blocks submit and shows a field error for a future date of birth", async () => {
  const { updateProfile } = renderProfile()

  await userEvent.type(screen.getByLabelText("Ngày sinh"), "2999-01-01")
  await userEvent.click(screen.getByRole("button", { name: "Lưu thay đổi" }))

  expect(updateProfile).not.toHaveBeenCalled()
  expect(screen.getByText("Ngày sinh phải ở trong quá khứ")).toBeInTheDocument()
})

it("blocks submit when a province is chosen without a ward", async () => {
  const { updateProfile } = renderProfile()

  await selectProvince("TP. Hồ Chí Minh")
  await userEvent.click(screen.getByRole("button", { name: "Lưu thay đổi" }))

  expect(updateProfile).not.toHaveBeenCalled()
  expect(screen.getByText("Vui lòng chọn phường/xã")).toBeInTheDocument()
})

it("maps backend validation field errors to the matching form fields", async () => {
  const backendError = new ApiClientError(
    400,
    "Request validation failed",
    "VALIDATION_FAILED",
    "trace-1",
    {
      dateOfBirth: "must be a past date",
      wardCode: "Ward is not in the selected province",
    }
  )
  const { updateProfile } = renderProfile(vi.fn().mockRejectedValue(backendError))

  await userEvent.click(screen.getByRole("button", { name: "Lưu thay đổi" }))

  expect(await screen.findByText("must be a past date")).toBeInTheDocument()
  expect(screen.getByText("Ward is not in the selected province")).toBeInTheDocument()
  expect(updateProfile).toHaveBeenCalledTimes(1)
})
