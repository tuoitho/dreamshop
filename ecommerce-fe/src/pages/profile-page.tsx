import { useEffect, useState, type FormEvent } from "react"
import { useNavigate } from "react-router-dom"
import { X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { SearchableSelect } from "@/components/ui/searchable-select"
import { ApiClientError } from "@/lib/api"
import { fetchProvinces, fetchWards } from "@/lib/locations"
import { fieldErrorsFromBackend, fieldErrorsFromZod, profileSchema, type ProfileField } from "@/lib/profile-validation"
import type { Gender, LocationOption } from "@/lib/types"
import { useAuth } from "@/hooks/use-auth"

const GENDERS = [
  { value: "MALE", label: "Nam" },
  { value: "FEMALE", label: "Nữ" },
  { value: "OTHER", label: "Khác" },
]

const PROVINCE_LOAD_ERROR = "Không tải được danh sách tỉnh/thành phố"
const WARD_LOAD_ERROR = "Không tải được danh sách phường/xã"

export default function ProfilePage() {
  const { user, updateProfile, logout } = useAuth()
  const navigate = useNavigate()
  const [fullName, setFullName] = useState(user?.fullName ?? "")
  const [phone, setPhone] = useState(user?.phone ?? "")
  const [dateOfBirth, setDateOfBirth] = useState(user?.dateOfBirth ?? "")
  const [gender, setGender] = useState<Gender | "">(user?.gender ?? "")
  const [provinceCode, setProvinceCode] = useState(user?.provinceCode ?? "")
  const [wardCode, setWardCode] = useState(user?.wardCode ?? "")
  const [addressLine, setAddressLine] = useState(user?.addressLine ?? "")

  const [provinces, setProvinces] = useState<LocationOption[]>([])
  const [provinceLoading, setProvinceLoading] = useState(true)
  const [provinceError, setProvinceError] = useState<string | null>(null)
  const [provinceReload, setProvinceReload] = useState(0)

  const [wards, setWards] = useState<LocationOption[]>([])
  const [wardLoading, setWardLoading] = useState(false)
  const [wardError, setWardError] = useState<string | null>(null)
  const [wardReload, setWardReload] = useState(0)

  const [error, setError] = useState<string | null>(null)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    let cancelled = false
    setProvinceLoading(true)
    setProvinceError(null)
    fetchProvinces()
      .then((data) => {
        if (!cancelled) setProvinces(data)
      })
      .catch(() => {
        if (!cancelled) setProvinceError(PROVINCE_LOAD_ERROR)
      })
      .finally(() => {
        if (!cancelled) setProvinceLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [provinceReload])

  useEffect(() => {
    if (!user) return
    setFullName(user.fullName)
    setPhone(user.phone ?? "")
    setDateOfBirth(user.dateOfBirth ?? "")
    setGender(user.gender ?? "")
    setProvinceCode(user.provinceCode ?? "")
    setWardCode(user.wardCode ?? "")
    setAddressLine(user.addressLine ?? "")
  }, [user])

  useEffect(() => {
    if (!provinceCode) {
      setWards([])
      setWardError(null)
      setWardLoading(false)
      return
    }
    let cancelled = false
    setWards([])
    setWardLoading(true)
    setWardError(null)
    fetchWards(provinceCode)
      .then((data) => {
        if (!cancelled) setWards(data)
      })
      .catch(() => {
        if (!cancelled) setWardError(WARD_LOAD_ERROR)
      })
      .finally(() => {
        if (!cancelled) setWardLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [provinceCode, wardReload])

  function clearFieldError(field: ProfileField) {
    setFieldErrors((current) => {
      if (!(field in current)) return current
      const next = { ...current }
      delete next[field]
      return next
    })
  }

  function handleProvinceChange(code: string) {
    setProvinceCode(code)
    setWardCode("")
    clearFieldError("provinceCode")
    clearFieldError("wardCode")
  }

  function clearProvince() {
    setProvinceCode("")
    setWardCode("")
    clearFieldError("provinceCode")
    clearFieldError("wardCode")
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError(null)
    setFieldErrors({})

    const validation = profileSchema.safeParse({
      fullName,
      phone,
      dateOfBirth,
      gender,
      provinceCode,
      wardCode,
      addressLine,
    })
    if (!validation.success) {
      setFieldErrors(fieldErrorsFromZod(validation.error))
      return
    }

    setSubmitting(true)
    try {
      await updateProfile({
        fullName,
        phone,
        dateOfBirth,
        gender,
        provinceCode,
        wardCode,
        addressLine,
      })
    } catch (reason) {
      if (reason instanceof ApiClientError && reason.errors) {
        setFieldErrors(fieldErrorsFromBackend(reason.errors))
      } else {
        setError(reason instanceof Error ? reason.message : "Không thể cập nhật hồ sơ")
      }
    } finally {
      setSubmitting(false)
    }
  }

  async function signOut() {
    await logout()
    navigate("/login", { replace: true })
  }

  if (!user) return null
  return (
    <div className="mx-auto w-full max-w-5xl px-4 py-8">
      <header className="mb-6">
        <h1 className="text-2xl font-bold">Hồ sơ</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Quản lý thông tin cá nhân và địa chỉ giao hàng.
        </p>
      </header>
      <form className="grid gap-6 lg:grid-cols-2" onSubmit={submit}>
        <Card>
          <CardHeader>
            <CardTitle>Thông tin cá nhân</CardTitle>
            <CardDescription>Email không thể thay đổi.</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <div className="grid gap-2">
              <Label htmlFor="full-name">Họ và tên</Label>
              <Input
                id="full-name"
                value={fullName}
                aria-invalid={fieldErrors.fullName ? true : undefined}
                aria-describedby={fieldErrors.fullName ? "full-name-error" : undefined}
                onChange={(event) => {
                  setFullName(event.target.value)
                  clearFieldError("fullName")
                }}
              />
              {fieldErrors.fullName ? (
                <p id="full-name-error" role="alert" className="text-sm text-destructive">
                  {fieldErrors.fullName}
                </p>
              ) : null}
            </div>
            <div className="grid gap-2">
              <Label htmlFor="email">Email</Label>
              <Input id="email" value={user.email} readOnly />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="phone">Số điện thoại</Label>
              <Input
                id="phone"
                value={phone}
                aria-invalid={fieldErrors.phone ? true : undefined}
                aria-describedby={fieldErrors.phone ? "phone-error" : undefined}
                onChange={(event) => {
                  setPhone(event.target.value)
                  clearFieldError("phone")
                }}
              />
              {fieldErrors.phone ? (
                <p id="phone-error" role="alert" className="text-sm text-destructive">
                  {fieldErrors.phone}
                </p>
              ) : null}
            </div>
            <div className="grid gap-2">
              <Label htmlFor="date-of-birth">Ngày sinh</Label>
              <Input
                id="date-of-birth"
                type="date"
                value={dateOfBirth}
                aria-invalid={fieldErrors.dateOfBirth ? true : undefined}
                aria-describedby={fieldErrors.dateOfBirth ? "date-of-birth-error" : undefined}
                onChange={(event) => {
                  setDateOfBirth(event.target.value)
                  clearFieldError("dateOfBirth")
                }}
              />
              {fieldErrors.dateOfBirth ? (
                <p id="date-of-birth-error" role="alert" className="text-sm text-destructive">
                  {fieldErrors.dateOfBirth}
                </p>
              ) : null}
            </div>
            <div className="grid gap-2">
              <div className="flex items-center justify-between gap-2">
                <Label htmlFor="gender">Giới tính</Label>
                {gender ? (
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    type="button"
                    aria-label="Xóa giới tính"
                    onClick={() => {
                      setGender("")
                      clearFieldError("gender")
                    }}
                  >
                    <X />
                  </Button>
                ) : null}
              </div>
              <Select
                value={gender}
                onValueChange={(value) => {
                  setGender(value as Gender)
                  clearFieldError("gender")
                }}
              >
                <SelectTrigger id="gender">
                  <SelectValue placeholder="Chọn giới tính" />
                </SelectTrigger>
                <SelectContent>
                  {GENDERS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {fieldErrors.gender ? (
                <p role="alert" className="text-sm text-destructive">
                  {fieldErrors.gender}
                </p>
              ) : null}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Địa chỉ giao hàng</CardTitle>
            <CardDescription>Chọn tỉnh/thành phố rồi phường/xã.</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <div className="grid gap-2">
              <div className="flex items-center justify-between gap-2">
                <Label htmlFor="province">Tỉnh/thành phố</Label>
                {provinceCode ? (
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    type="button"
                    aria-label="Xóa tỉnh/thành phố"
                    onClick={clearProvince}
                  >
                    <X />
                  </Button>
                ) : null}
              </div>
              <SearchableSelect
                id="province"
                value={provinceCode}
                placeholder="Chọn tỉnh/thành phố"
                options={provinces}
                onValueChange={handleProvinceChange}
                loading={provinceLoading}
                error={provinceError}
                onRetry={() => setProvinceReload((count) => count + 1)}
                searchPlaceholder="Tìm tỉnh/thành phố"
                searchLabel="Tìm tỉnh/thành phố"
              />
              {fieldErrors.provinceCode ? (
                <p role="alert" className="text-sm text-destructive">
                  {fieldErrors.provinceCode}
                </p>
              ) : null}
            </div>
            <div className="grid gap-2">
              <Label htmlFor="ward">Phường/xã</Label>
              <SearchableSelect
                id="ward"
                value={wardCode}
                placeholder="Chọn phường/xã"
                options={wards}
                onValueChange={(code) => {
                  setWardCode(code)
                  clearFieldError("wardCode")
                }}
                disabled={!provinceCode}
                loading={wardLoading}
                error={wardError}
                onRetry={() => setWardReload((count) => count + 1)}
                searchPlaceholder="Tìm phường/xã"
                searchLabel="Tìm phường/xã"
              />
              {fieldErrors.wardCode ? (
                <p role="alert" className="text-sm text-destructive">
                  {fieldErrors.wardCode}
                </p>
              ) : null}
            </div>
            <div className="grid gap-2">
              <Label htmlFor="address-line">Địa chỉ chi tiết</Label>
              <Input
                id="address-line"
                value={addressLine}
                aria-invalid={fieldErrors.addressLine ? true : undefined}
                aria-describedby={fieldErrors.addressLine ? "address-line-error" : undefined}
                onChange={(event) => {
                  setAddressLine(event.target.value)
                  clearFieldError("addressLine")
                }}
              />
              {fieldErrors.addressLine ? (
                <p id="address-line-error" role="alert" className="text-sm text-destructive">
                  {fieldErrors.addressLine}
                </p>
              ) : null}
            </div>
          </CardContent>
        </Card>

        <div className="flex flex-col gap-3 lg:col-span-2">
          {error ? (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          ) : null}
          <div className="flex flex-col gap-2 sm:flex-row">
            <Button type="submit" disabled={submitting}>
              {submitting ? "Đang lưu..." : "Lưu thay đổi"}
            </Button>
            <Button variant="outline" type="button" onClick={() => void signOut()}>
              Đăng xuất
            </Button>
          </div>
        </div>
      </form>
    </div>
  )
}
