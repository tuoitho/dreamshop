import { z } from "zod"

const VIETNAMESE_MOBILE = /^(?:(?:\+84|0)(?:3|5|7|8|9)\d{8})?$/
const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/

function isPastDate(value: string): boolean {
  if (!ISO_DATE.test(value)) return false
  const [year, month, day] = value.split("-").map(Number)
  const date = new Date(Date.UTC(year, month - 1, day))
  return (
    date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day &&
    date.getTime() < Date.now()
  )
}

export const profileSchema = z
  .object({
    fullName: z
      .string()
      .trim()
      .min(1, "Họ và tên không được để trống")
      .max(100, "Họ và tên tối đa 100 ký tự"),
    phone: z.string().refine((value) => value === "" || VIETNAMESE_MOBILE.test(value), {
      message: "Số điện thoại không hợp lệ",
    }),
    dateOfBirth: z.string().refine((value) => value === "" || isPastDate(value), {
      message: "Ngày sinh phải ở trong quá khứ",
    }),
    gender: z.enum(["MALE", "FEMALE", "OTHER"]).or(z.literal("")),
    provinceCode: z.string(),
    wardCode: z.string(),
    addressLine: z.string().max(255, "Địa chỉ chi tiết tối đa 255 ký tự"),
  })
  .superRefine((value, ctx) => {
    if (value.provinceCode && !value.wardCode) {
      ctx.addIssue({
        code: "custom",
        path: ["wardCode"],
        message: "Vui lòng chọn phường/xã",
      })
    } else if (value.wardCode && !value.provinceCode) {
      ctx.addIssue({
        code: "custom",
        path: ["provinceCode"],
        message: "Vui lòng chọn tỉnh/thành phố",
      })
    }
  })

export type ProfileField = keyof z.infer<typeof profileSchema>

type ZodLikeError = {
  issues: ReadonlyArray<{ path: ReadonlyArray<string | number | symbol>; message: string }>
}

export function fieldErrorsFromZod(error: ZodLikeError): Record<string, string> {
  const fieldErrors: Record<string, string> = {}
  for (const issue of error.issues) {
    const field = issue.path[0]
    if (typeof field === "string" && fieldErrors[field] === undefined) {
      fieldErrors[field] = issue.message
    }
  }
  return fieldErrors
}

const BACKEND_FIELD_NAMES: Record<string, ProfileField> = {
  fullName: "fullName",
  dateOfBirth: "dateOfBirth",
  provinceCode: "provinceCode",
  wardCode: "wardCode",
  addressLine: "addressLine",
}

export function fieldErrorsFromBackend(errors: Record<string, string>): Record<string, string> {
  const fieldErrors: Record<string, string> = {}
  for (const [field, message] of Object.entries(errors)) {
    fieldErrors[BACKEND_FIELD_NAMES[field] ?? field] = message
  }
  return fieldErrors
}
