import { describe, expect, it } from "vitest"
import { ApiClientError } from "./api"
import { resolveErrorMessage } from "./error-messages"

describe("admin resolveErrorMessage", () => {
  it("translates known admin backend error codes to Vietnamese", () => {
    expect(
      resolveErrorMessage(
        new ApiClientError(
          409,
          "Physical stock quantity cannot be less than currently reserved units.",
          "STOCK_BELOW_RESERVED"
        )
      )
    ).toBe("Số lượng tồn kho không thể nhỏ hơn số lượng đang được giữ cho đơn hàng.")

    expect(
      resolveErrorMessage(
        new ApiClientError(
          409,
          "Cannot delete a variant while orders still hold reserved stock for it.",
          "VARIANT_HAS_RESERVED_STOCK",
          "trace-1",
          { skuId: "sku-1" }
        )
      )
    ).toBe('Không thể xóa phân loại "sku-1" vì đang có đơn hàng giữ tồn kho.')

    expect(
      resolveErrorMessage(
        new ApiClientError(
          409,
          "An administrator cannot disable their own account",
          "SELF_DISABLE_NOT_ALLOWED"
        )
      )
    ).toBe("Không thể tự vô hiệu hóa tài khoản quản trị của chính mình.")

    expect(
      resolveErrorMessage(
        new ApiClientError(
          409,
          "At least one active super administrator is required",
          "LAST_SUPER_ADMIN_REQUIRED"
        )
      )
    ).toBe("Hệ thống phải có ít nhất một tài khoản Super Admin đang hoạt động.")

    expect(
      resolveErrorMessage(
        new ApiClientError(409, "RBAC system item is protected", "RBAC_SYSTEM_ITEM_PROTECTED")
      )
    ).toBe("Mục phân quyền hệ thống được bảo vệ, không thể thay đổi hoặc xóa.")

    expect(
      resolveErrorMessage(
        new ApiClientError(401, "Email or password is invalid.", "INVALID_CREDENTIALS")
      )
    ).toBe("Email hoặc mật khẩu không đúng.")
  })

  it("handles network errors including fetch TypeError and falls back for non-error values", () => {
    expect(resolveErrorMessage(new ApiClientError(null, "Network Error"))).toBe(
      "Kết nối mạng bị gián đoạn. Vui lòng kiểm tra kết nối và thử lại."
    )
    expect(resolveErrorMessage(new TypeError("Failed to fetch"))).toBe(
      "Kết nối mạng bị gián đoạn. Vui lòng kiểm tra kết nối và thử lại."
    )
    expect(resolveErrorMessage(undefined, { fallback: "Thao tác thất bại." })).toBe(
      "Thao tác thất bại."
    )
  })

  it("uses fallback for unmapped English messages and keeps Vietnamese messages", () => {
    expect(
      resolveErrorMessage(new ApiClientError(500, "Internal failure", "SOMETHING_NEW"), {
        fallback: "Thao tác thất bại.",
      })
    ).toBe("Thao tác thất bại.")
    expect(
      resolveErrorMessage(
        new ApiClientError(409, "Trạng thái đơn vừa thay đổi", "CUSTOM_VI_CODE")
      )
    ).toBe("Trạng thái đơn vừa thay đổi")
  })
})
