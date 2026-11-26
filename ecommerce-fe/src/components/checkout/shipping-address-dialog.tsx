import { useEffect, useState, type FormEvent } from "react"
import { Dialog } from "radix-ui"
import { Check, MapPin, Plus, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { SearchableSelect } from "@/components/ui/searchable-select"
import { createShippingAddress } from "@/lib/api"
import { resolveErrorMessage } from "@/lib/error-messages"
import { fetchProvinces, fetchWards } from "@/lib/locations"
import type { LocationOption, ShippingAddress } from "@/lib/types"
import { cn } from "@/lib/utils"

const VIETNAMESE_PHONE_PATTERN = /^(?:\+84|0)(?:3|5|7|8|9)\d{8}$/

interface ShippingAddressDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  addresses: ShippingAddress[]
  selectedAddressId: string | null
  onSelectAddress: (address: ShippingAddress) => void
  onAddressCreated: (address: ShippingAddress) => void
}

export function ShippingAddressDialog({
  open,
  onOpenChange,
  addresses,
  selectedAddressId,
  onSelectAddress,
  onAddressCreated,
}: ShippingAddressDialogProps) {
  const [creating, setCreating] = useState(addresses.length === 0)
  const [recipientName, setRecipientName] = useState("")
  const [phone, setPhone] = useState("")
  const [provinceCode, setProvinceCode] = useState("")
  const [wardCode, setWardCode] = useState("")
  const [addressLine, setAddressLine] = useState("")
  const [label, setLabel] = useState("")
  const [isDefault, setIsDefault] = useState(addresses.length === 0)

  const [provinces, setProvinces] = useState<LocationOption[]>([])
  const [wards, setWards] = useState<LocationOption[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) return
    setCreating(addresses.length === 0)
    setFormError(null)
    let cancelled = false
    fetchProvinces()
      .then((list) => {
        if (!cancelled) setProvinces(list)
      })
      .catch(() => {
        if (!cancelled) setProvinces([])
      })
    return () => {
      cancelled = true
    }
  }, [open, addresses.length])

  useEffect(() => {
    if (!provinceCode) {
      setWards([])
      return
    }
    let cancelled = false
    fetchWards(provinceCode)
      .then((list) => {
        if (!cancelled) setWards(list)
      })
      .catch(() => {
        if (!cancelled) setWards([])
      })
    return () => {
      cancelled = true
    }
  }, [provinceCode])

  async function handleCreateSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setFormError(null)
    if (!recipientName.trim()) {
      setFormError("Vui lòng nhập họ tên người nhận.")
      return
    }
    if (!VIETNAMESE_PHONE_PATTERN.test(phone.trim())) {
      setFormError("Số điện thoại không hợp lệ (ví dụ: 0901234567).")
      return
    }
    if (!provinceCode || !wardCode || !addressLine.trim()) {
      setFormError("Vui lòng chọn đầy đủ tỉnh/thành, phường/xã và địa chỉ chi tiết.")
      return
    }

    setSubmitting(true)
    try {
      const created = await createShippingAddress({
        recipientName: recipientName.trim(),
        phone: phone.trim(),
        provinceCode,
        wardCode,
        addressLine: addressLine.trim(),
        label: label.trim() || undefined,
        isDefault,
      })
      onAddressCreated(created)
      setRecipientName("")
      setPhone("")
      setProvinceCode("")
      setWardCode("")
      setAddressLine("")
      setLabel("")
      setCreating(false)
      onOpenChange(false)
    } catch (error) {
      setFormError(resolveErrorMessage(error, { fallback: "Không thể lưu địa chỉ mới." }))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/50 backdrop-blur-xs" />
        <Dialog.Content className="fixed top-1/2 left-1/2 z-50 max-h-[85vh] w-[calc(100%-2rem)] max-w-lg -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-2xl border bg-card p-6 shadow-xl outline-none">
          <div className="flex items-center justify-between gap-3">
            <Dialog.Title className="flex items-center gap-2 text-lg font-bold">
              <MapPin className="size-4 text-primary" />
              <span>{creating ? "Thêm địa chỉ nhận hàng" : "Chọn địa chỉ nhận hàng"}</span>
            </Dialog.Title>
            <Dialog.Close asChild>
              <Button type="button" variant="ghost" size="icon" aria-label="Đóng hộp thoại địa chỉ">
                <X />
              </Button>
            </Dialog.Close>
          </div>
          <Dialog.Description className="mt-1 text-xs text-muted-foreground">
            Địa chỉ đã chọn sẽ được chụp cố định vào đơn hàng khi xác nhận đặt mua.
          </Dialog.Description>

          {!creating ? (
            <div className="mt-4 space-y-3">
              {addresses.map((addr) => {
                const selected = addr.id === selectedAddressId
                return (
                  <button
                    key={addr.id}
                    type="button"
                    onClick={() => {
                      onSelectAddress(addr)
                      onOpenChange(false)
                    }}
                    className={cn(
                      "flex w-full items-start justify-between gap-3 rounded-xl border p-3.5 text-left transition-colors",
                      selected
                        ? "border-primary bg-primary/5"
                        : "border-border hover:bg-muted/50"
                    )}
                  >
                    <div className="min-w-0 text-sm">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-semibold">{addr.recipientName}</span>
                        <span className="text-muted-foreground">· {addr.phone}</span>
                        {addr.label ? (
                          <span className="rounded bg-muted px-1.5 py-0.5 text-[11px] font-medium">
                            {addr.label}
                          </span>
                        ) : null}
                        {addr.isDefault ? (
                          <span className="rounded bg-primary/10 px-1.5 py-0.5 text-[11px] font-medium text-primary">
                            Mặc định
                          </span>
                        ) : null}
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">{addr.addressLine}</p>
                    </div>
                    {selected ? <Check className="mt-0.5 size-4 shrink-0 text-primary" /> : null}
                  </button>
                )
              })}

              <Button
                type="button"
                variant="outline"
                className="w-full"
                onClick={() => setCreating(true)}
              >
                <Plus /> Thêm địa chỉ mới
              </Button>
            </div>
          ) : (
            <form onSubmit={handleCreateSubmit} className="mt-4 space-y-3">
              {formError ? (
                <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-xs text-destructive">
                  {formError}
                </p>
              ) : null}

              <div className="grid gap-1.5">
                <Label htmlFor="ship-recipient">Họ và tên người nhận</Label>
                <Input
                  id="ship-recipient"
                  value={recipientName}
                  onChange={(e) => setRecipientName(e.target.value)}
                  placeholder="Nguyễn Văn A"
                />
              </div>

              <div className="grid gap-1.5">
                <Label htmlFor="ship-phone">Số điện thoại</Label>
                <Input
                  id="ship-phone"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  placeholder="0901234567"
                />
              </div>

              <div className="grid gap-3 sm:grid-cols-2">
                <div className="grid gap-1.5">
                  <Label htmlFor="ship-province">Tỉnh / Thành phố</Label>
                  <SearchableSelect
                    id="ship-province"
                    value={provinceCode}
                    onValueChange={(code) => {
                      setProvinceCode(code)
                      setWardCode("")
                    }}
                    options={provinces}
                    placeholder="Chọn tỉnh/thành"
                    searchPlaceholder="Tìm tỉnh/thành..."
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="ship-ward">Phường / Xã</Label>
                  <SearchableSelect
                    id="ship-ward"
                    value={wardCode}
                    onValueChange={setWardCode}
                    options={wards}
                    placeholder="Chọn phường/xã"
                    searchPlaceholder="Tìm phường/xã..."
                    disabled={!provinceCode}
                  />
                </div>
              </div>

              <div className="grid gap-1.5">
                <Label htmlFor="ship-line">Số nhà, tên đường</Label>
                <Input
                  id="ship-line"
                  value={addressLine}
                  onChange={(e) => setAddressLine(e.target.value)}
                  placeholder="1 Lê Duẩn"
                />
              </div>

              <div className="grid gap-1.5">
                <Label htmlFor="ship-label">Nhãn địa chỉ (tùy chọn)</Label>
                <Input
                  id="ship-label"
                  value={label}
                  onChange={(e) => setLabel(e.target.value)}
                  placeholder="Nhà riêng / Văn phòng"
                />
              </div>

              <label className="flex items-center gap-2 text-xs text-muted-foreground">
                <Checkbox
                  checked={isDefault}
                  onCheckedChange={(checked) => setIsDefault(checked === true)}
                />
                <span>Đặt làm địa chỉ mặc định</span>
              </label>

              <div className="flex items-center justify-end gap-2 pt-2">
                {addresses.length > 0 ? (
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={() => setCreating(false)}
                    disabled={submitting}
                  >
                    Quay lại danh sách
                  </Button>
                ) : null}
                <Button type="submit" disabled={submitting}>
                  {submitting ? "Đang lưu..." : "Lưu địa chỉ"}
                </Button>
              </div>
            </form>
          )}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
