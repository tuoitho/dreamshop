import { useContext, useEffect, useRef, useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { AlertTriangle, ArrowLeft, CreditCard, Loader2, MapPin, ShoppingBag, Truck } from "lucide-react"
import { toast } from "sonner"
import { ShippingAddressDialog } from "@/components/checkout/shipping-address-dialog"
import { ProductImage } from "@/components/product-image"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { AuthContext } from "@/context/auth-context"
import { useCart } from "@/hooks/use-cart"
import { ApiClientError, createOrder, listShippingAddresses } from "@/lib/api"
import { fetchProvinces, fetchWards } from "@/lib/locations"
import { FREE_SHIPPING_THRESHOLD, STANDARD_SHIPPING_FEE } from "@/lib/shipping"
import type { CartItem, PaymentMethod, ShippingAddress } from "@/lib/types"
import { cn, formatVND } from "@/lib/utils"

interface FrozenItemSnapshot {
  unitPrice: number
  expectedName: string
  variantLabel: string
  expectedProductVersion: number
}

function generateIdempotencyKey(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID()
  }
  return `idem-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

function isPurchasable(item: CartItem): boolean {
  return item.status === "AVAILABLE" || item.status === "LIMITED"
}

function PreviewItemThumbnail({ item }: { item: CartItem }) {
  const [imageError, setImageError] = useState(false)
  const displayName = item.name.trim() || "Sản phẩm"

  if (item.imageUrl && !imageError) {
    return (
      <img
        src={item.imageUrl}
        alt={displayName}
        loading="lazy"
        className="size-16 shrink-0 rounded-lg object-cover ring-1 ring-foreground/10"
        onError={() => setImageError(true)}
      />
    )
  }

  return (
    <div className="size-16 shrink-0 overflow-hidden rounded-lg ring-1 ring-foreground/10">
      <ProductImage
        emoji={item.imageEmoji || "📦"}
        hue={[item.imageHue[0] ?? 200, item.imageHue[1] ?? 240]}
        seed={item.productId}
        label={displayName}
      />
    </div>
  )
}

export default function OrderPreviewPage() {
  const auth = useContext(AuthContext)
  const user = auth?.user ?? null
  const navigate = useNavigate()
  const { selectedItems, retryLoad } = useCart()

  const [addresses, setAddresses] = useState<ShippingAddress[]>([])
  const [selectedAddress, setSelectedAddress] = useState<ShippingAddress | null>(null)
  const [addressDialogOpen, setAddressDialogOpen] = useState(false)
  const [provinceName, setProvinceName] = useState<string>("")
  const [wardName, setWardName] = useState<string>("")

  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>("COD")
  const [note, setNote] = useState<string>("")
  const [submitting, setSubmitting] = useState(false)
  const [inProgressRetry, setInProgressRetry] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [snapshotMismatch, setSnapshotMismatch] = useState<{
    message: string
    details: Record<string, string>
  } | null>(null)

  const idempotencyKeyRef = useRef<string>(generateIdempotencyKey())
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const snapshotMapRef = useRef<Map<string, FrozenItemSnapshot>>(new Map())

  const validSelectedItems = selectedItems.filter(isPurchasable)

  for (const item of validSelectedItems) {
    if (!snapshotMapRef.current.has(item.skuId)) {
      snapshotMapRef.current.set(item.skuId, {
        unitPrice: item.unitPrice,
        expectedName: item.name,
        variantLabel: item.variantLabel,
        expectedProductVersion: item.productVersion ?? 0,
      })
    }
  }

  useEffect(() => {
    return () => {
      if (retryTimerRef.current !== null) {
        clearTimeout(retryTimerRef.current)
      }
    }
  }, [])

  useEffect(() => {
    if (!user) {
      setAddresses([])
      setSelectedAddress(null)
      return
    }
    let active = true
    listShippingAddresses()
      .then((list) => {
        if (!active) return
        setAddresses(list)
        if (list.length > 0) {
          const preferred = list.find((entry) => entry.isDefault) ?? list[0]
          setSelectedAddress(preferred)
        } else {
          setSelectedAddress(null)
        }
      })
      .catch(() => {
        if (active) {
          setAddresses([])
          setSelectedAddress(null)
        }
      })
    return () => {
      active = false
    }
  }, [user])

  const activeProvinceCode = selectedAddress?.provinceCode ?? user?.provinceCode ?? null
  const activeWardCode = selectedAddress?.wardCode ?? user?.wardCode ?? null
  const activeRecipientName =
    selectedAddress?.recipientName ?? (user ? user.fullName.trim() || user.email : "")
  const activePhone = selectedAddress?.phone ?? user?.phone ?? null
  const activeAddressLine = selectedAddress?.addressLine ?? user?.addressLine ?? null
  const activeAddressLabel = selectedAddress?.label ?? undefined

  useEffect(() => {
    if (!activeProvinceCode) {
      setProvinceName("")
      setWardName("")
      return
    }
    let active = true
    fetchProvinces()
      .then((list) => {
        if (active) {
          setProvinceName(list.find((entry) => entry.code === activeProvinceCode)?.name ?? "")
        }
      })
      .catch(() => {
        if (active) setProvinceName("")
      })

    if (activeWardCode) {
      fetchWards(activeProvinceCode)
        .then((list) => {
          if (active) {
            setWardName(list.find((entry) => entry.code === activeWardCode)?.name ?? "")
          }
        })
        .catch(() => {
          if (active) setWardName("")
        })
    } else {
      setWardName("")
    }

    return () => {
      active = false
    }
  }, [activeProvinceCode, activeWardCode])

  function rotateIdempotencyKey() {
    idempotencyKeyRef.current = generateIdempotencyKey()
  }

  const subtotal = validSelectedItems.reduce(
    (total, item) => total + item.unitPrice * item.quantity,
    0
  )
  const shippingFee =
    subtotal > 0
      ? subtotal >= FREE_SHIPPING_THRESHOLD
        ? 0
        : STANDARD_SHIPPING_FEE
      : 0
  const total = subtotal + shippingFee

  if (validSelectedItems.length === 0) {
    return (
      <div className="mx-auto flex max-w-3xl flex-col items-center justify-center px-4 py-24 text-center">
        <span className="grid size-20 place-items-center rounded-full bg-muted">
          <ShoppingBag className="size-9 text-muted-foreground" />
        </span>
        <h1 className="mt-6 text-xl font-bold sm:text-2xl">
          Chưa có sản phẩm hợp lệ để đặt hàng
        </h1>
        <p className="mt-2 max-w-sm text-sm text-muted-foreground">
          Hãy quay lại giỏ hàng và chọn sản phẩm còn có thể đặt.
        </p>
        <Button size="lg" className="mt-6" asChild>
          <Link to="/gio-hang">Quay lại giỏ hàng</Link>
        </Button>
      </div>
    )
  }

  const addressParts = [activeAddressLine, wardName, provinceName].filter(
    (part): part is string => Boolean(part && part.trim())
  )
  const formattedAddress = addressParts.join(", ")
  const hasCompleteShipping = Boolean(
    activeRecipientName && activePhone && activeProvinceCode && activeWardCode && activeAddressLine
  )

  async function submitOrder() {
    if (snapshotMismatch !== null) {
      return
    }
    if (!hasCompleteShipping || !activePhone || !activeProvinceCode || !activeWardCode || !activeAddressLine) {
      toast.error("Vui lòng chọn hoặc bổ sung đầy đủ địa chỉ nhận hàng trước khi đặt mua.")
      setAddressDialogOpen(true)
      return
    }

    if (retryTimerRef.current !== null) {
      clearTimeout(retryTimerRef.current)
      retryTimerRef.current = null
    }
    setSubmitting(true)
    setSubmitError(null)

    const payload = {
      paymentMethod,
      ...(note.trim() ? { note: note.trim() } : {}),
      shippingAddress: {
        recipientName: activeRecipientName,
        phone: activePhone,
        provinceCode: activeProvinceCode,
        provinceName: provinceName || activeProvinceCode,
        wardCode: activeWardCode,
        wardName: wardName || activeWardCode,
        addressLine: activeAddressLine,
        ...(activeAddressLabel ? { label: activeAddressLabel } : {}),
      },
      items: validSelectedItems.map((item) => {
        const snap = snapshotMapRef.current.get(item.skuId)
        return {
          cartItemId: item.itemId,
          productId: item.productId,
          skuId: item.skuId,
          quantity: item.quantity,
          unitPrice: snap?.unitPrice ?? item.unitPrice,
          expectedName: snap?.expectedName ?? item.name,
          brand: item.brand,
          categoryName: item.categoryName,
          variantLabel: snap?.variantLabel ?? item.variantLabel,
          variantColorHex: item.variantColorHex,
          imageUrl: item.imageUrl,
          imageEmoji: item.imageEmoji,
          imageHue: item.imageHue,
          expectedProductVersion: snap?.expectedProductVersion ?? item.productVersion ?? 0,
        }
      }),
    }

    try {
      const result = await createOrder(payload, idempotencyKeyRef.current)
      setInProgressRetry(false)
      setSubmitting(false)

      if (paymentMethod === "MOMO" && result.payUrl) {
        window.location.assign(result.payUrl)
        return
      }

      void retryLoad()
      navigate(`/don-hang/${encodeURIComponent(result.order.orderId)}?placed=1`, { replace: true })
    } catch (error) {
      const clientErr = error instanceof ApiClientError ? error : null

      if (clientErr?.code === "REQUEST_IN_PROGRESS") {
        // Retain current Idempotency-Key and auto-retry once previous attempt settles.
        setInProgressRetry(true)
        retryTimerRef.current = setTimeout(() => {
          retryTimerRef.current = null
          void submitOrder()
        }, 1500)
        return
      }

      setInProgressRetry(false)
      setSubmitting(false)

      if (clientErr?.code === "PRODUCT_SNAPSHOT_MISMATCH") {
        rotateIdempotencyKey()
        setSnapshotMismatch({
          message: clientErr.message || "Giá hoặc thông tin sản phẩm vừa thay đổi.",
          details: clientErr.errors ?? {},
        })
        return
      }

      if (clientErr !== null && clientErr.status === null) {
        // Network drop without HTTP response: retain Idempotency-Key for safe manual retry.
        setSubmitError("Kết nối mạng bị gián đoạn. Bạn có thể bấm Đặt hàng lại mà không lo bị tạo đơn trùng.")
        return
      }

      rotateIdempotencyKey()
      setSubmitError(clientErr?.message ?? "Không thể tạo đơn hàng lúc này. Vui lòng thử lại.")
    }
  }

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <Button variant="ghost" size="sm" className="-ml-2" asChild>
        <Link to="/gio-hang">
          <ArrowLeft /> Quay lại giỏ hàng
        </Link>
      </Button>
      <h1 className="mt-4 text-2xl font-bold">Xác nhận đơn hàng</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        Kiểm tra địa chỉ nhận hàng, phương thức thanh toán và sản phẩm trước khi đặt mua.
      </p>

      {snapshotMismatch ? (
        <div
          role="alert"
          className="mt-5 rounded-2xl border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-950 dark:text-amber-100"
        >
          <div className="flex items-start gap-2.5">
            <AlertTriangle className="mt-0.5 size-5 shrink-0 text-amber-600 dark:text-amber-400" />
            <div className="flex-1 space-y-2">
              <p className="font-semibold">{snapshotMismatch.message}</p>
              {Object.keys(snapshotMismatch.details).length > 0 ? (
                <ul className="list-disc space-y-1 pl-5 text-xs">
                  {Object.entries(snapshotMismatch.details).map(([skuId, detail]) => (
                    <li key={skuId}>
                      <span className="font-medium">{skuId}:</span> {detail}
                    </li>
                  ))}
                </ul>
              ) : null}
              <div className="pt-1">
                <Button size="sm" variant="outline" asChild>
                  <Link to="/gio-hang">Quay lại giỏ hàng để cập nhật</Link>
                </Button>
              </div>
            </div>
          </div>
        </div>
      ) : null}

      <div className="mt-6 grid gap-6 lg:grid-cols-[1fr_340px]">
        <div className="space-y-6">
          {user ? (
            <section
              className="rounded-2xl bg-card p-5 ring-1 ring-foreground/10"
              aria-labelledby="shipping-recipient-heading"
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <MapPin className="size-4 text-primary" />
                  <h2 id="shipping-recipient-heading" className="text-base font-bold">
                    Thông tin nhận hàng
                  </h2>
                </div>
                <div className="flex items-center gap-3">
                  <button
                    type="button"
                    onClick={() => setAddressDialogOpen(true)}
                    className="text-xs font-medium text-primary hover:underline"
                  >
                    {addresses.length > 0 ? "Chọn / Thêm địa chỉ" : "Thêm địa chỉ nhận hàng"}
                  </button>
                  {!hasCompleteShipping ? (
                    <Link
                      to="/profile"
                      className="text-xs font-medium text-muted-foreground hover:underline"
                    >
                      Cập nhật hồ sơ
                    </Link>
                  ) : null}
                </div>
              </div>
              <div className="mt-3 text-sm">
                <p className="font-medium">
                  <span>{activeRecipientName}</span>
                  {activePhone ? (
                    <span className="ml-2 font-normal text-muted-foreground">
                      · {activePhone}
                    </span>
                  ) : null}
                  {activeAddressLabel ? (
                    <span className="ml-2 rounded bg-muted px-1.5 py-0.5 text-[11px] font-medium">
                      {activeAddressLabel}
                    </span>
                  ) : null}
                </p>
                {formattedAddress ? (
                  <p className="mt-1 text-xs text-muted-foreground">{formattedAddress}</p>
                ) : null}
                {!hasCompleteShipping ? (
                  <p className="mt-2 flex items-center gap-1.5 text-xs text-amber-700 dark:text-amber-300">
                    <AlertTriangle className="size-3.5 shrink-0" />
                    <span>
                      Vui lòng bổ sung đầy đủ số điện thoại và địa chỉ giao hàng.
                    </span>
                  </p>
                ) : null}
              </div>
            </section>
          ) : null}

          <section
            className="rounded-2xl bg-card p-5 ring-1 ring-foreground/10"
            aria-labelledby="preview-items-heading"
          >
            <h2 id="preview-items-heading" className="text-base font-bold">
              Sản phẩm đã chọn
            </h2>
            <div className="mt-4 divide-y divide-border">
              {validSelectedItems.map((item) => {
                const overStock =
                  item.status === "LIMITED" && item.quantity > item.availableQuantity
                return (
                  <div
                    key={item.itemId}
                    className="flex items-center justify-between gap-4 py-4 first:pt-0 last:pb-0"
                  >
                    <div className="flex min-w-0 items-center gap-3">
                      <PreviewItemThumbnail item={item} />
                      <div className="min-w-0">
                        <p className="truncate text-sm font-medium">{item.name}</p>
                        {item.variantLabel && (
                          <p className="mt-0.5 text-xs text-muted-foreground">
                            {item.variantLabel}
                          </p>
                        )}
                        <p className="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                          {(item.originalPrice ?? 0) > item.unitPrice && (
                            <span className="line-through">{formatVND(item.originalPrice!)}</span>
                          )}
                          <span>
                            {formatVND(item.unitPrice)} × {item.quantity}
                          </span>
                        </p>
                        {overStock ? (
                          <p className="mt-1 text-xs font-medium text-amber-700 dark:text-amber-300">
                            Vượt tồn kho hiện tại (chỉ còn {item.availableQuantity} sản phẩm)
                          </p>
                        ) : null}
                      </div>
                    </div>
                    <span className="shrink-0 text-sm font-semibold">
                      {formatVND(item.unitPrice * item.quantity)}
                    </span>
                  </div>
                )
              })}
            </div>
          </section>

          <section
            className="rounded-2xl bg-card p-5 ring-1 ring-foreground/10"
            aria-labelledby="payment-method-heading"
          >
            <h2 id="payment-method-heading" className="text-base font-bold">
              Phương thức thanh toán
            </h2>
            <div className="mt-3 grid gap-3 sm:grid-cols-2" role="radiogroup" aria-labelledby="payment-method-heading">
              <label
                className={cn(
                  "flex cursor-pointer items-start gap-3 rounded-xl border p-3.5 transition-colors",
                  paymentMethod === "COD"
                    ? "border-primary bg-primary/5"
                    : "border-border hover:bg-muted/50"
                )}
              >
                <input
                  type="radio"
                  name="paymentMethod"
                  value="COD"
                  checked={paymentMethod === "COD"}
                  onChange={() => {
                    setPaymentMethod("COD")
                    rotateIdempotencyKey()
                  }}
                  className="mt-1 accent-primary"
                />
                <div className="text-sm">
                  <div className="flex items-center gap-1.5 font-semibold">
                    <Truck className="size-4 text-primary" />
                    <span>Thanh toán khi nhận hàng (COD)</span>
                  </div>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    Kiểm tra và thanh toán tiền mặt khi nhận kiện hàng.
                  </p>
                </div>
              </label>

              <label
                className={cn(
                  "flex cursor-pointer items-start gap-3 rounded-xl border p-3.5 transition-colors",
                  paymentMethod === "MOMO"
                    ? "border-primary bg-primary/5"
                    : "border-border hover:bg-muted/50"
                )}
              >
                <input
                  type="radio"
                  name="paymentMethod"
                  value="MOMO"
                  checked={paymentMethod === "MOMO"}
                  onChange={() => {
                    setPaymentMethod("MOMO")
                    rotateIdempotencyKey()
                  }}
                  className="mt-1 accent-primary"
                />
                <div className="text-sm">
                  <div className="flex items-center gap-1.5 font-semibold">
                    <CreditCard className="size-4 text-pink-600" />
                    <span>Ví điện tử MoMo</span>
                  </div>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    Quét mã QR hoặc thanh toán nhanh qua cổng MoMo v2.
                  </p>
                </div>
              </label>
            </div>

            <div className="mt-4 grid gap-1.5">
              <Label htmlFor="order-note">Ghi chú cho đơn hàng (tùy chọn)</Label>
              <Input
                id="order-note"
                value={note}
                onChange={(e) => {
                  setNote(e.target.value)
                  rotateIdempotencyKey()
                }}
                placeholder="Ví dụ: Giao trong giờ hành chính..."
                maxLength={500}
              />
            </div>
          </section>
        </div>

        <section
          className="h-fit rounded-2xl bg-card p-5 ring-1 ring-foreground/10 lg:sticky lg:top-24"
          aria-labelledby="preview-summary-heading"
        >
          <h2 id="preview-summary-heading" className="text-base font-bold">
            Tóm tắt đơn hàng
          </h2>
          <Separator className="my-4" />
          <div className="grid gap-3 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Tạm tính</span>
              <span className="font-medium">{formatVND(subtotal)}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Phí vận chuyển</span>
              <span
                className={
                  shippingFee === 0 ? "font-medium text-emerald-600" : "font-medium"
                }
              >
                {shippingFee === 0 ? "Miễn phí" : formatVND(shippingFee)}
              </span>
            </div>
          </div>
          <Separator className="my-4" />
          <div className="flex items-center justify-between">
            <span className="text-sm font-semibold">Tổng cộng</span>
            <span className="text-xl font-extrabold text-red-600">{formatVND(total)}</span>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            {subtotal < FREE_SHIPPING_THRESHOLD
              ? `Mua thêm ${formatVND(FREE_SHIPPING_THRESHOLD - subtotal)} để được miễn phí vận chuyển`
              : "Đơn hàng của bạn được miễn phí vận chuyển 🎉"}
          </p>

          {inProgressRetry ? (
            <p className="mt-3 flex items-center gap-1.5 text-xs font-medium text-primary">
              <Loader2 className="size-3.5 animate-spin" />
              <span>Đơn hàng đang được xử lý...</span>
            </p>
          ) : null}

          {submitError ? (
            <p className="mt-3 text-xs text-destructive">{submitError}</p>
          ) : null}

          <Button
            size="lg"
            className="mt-4 w-full"
            disabled={submitting || snapshotMismatch !== null}
            onClick={() => void submitOrder()}
          >
            {submitting ? "Đang xử lý..." : "Đặt hàng"}
          </Button>
        </section>
      </div>

      <ShippingAddressDialog
        open={addressDialogOpen}
        onOpenChange={setAddressDialogOpen}
        addresses={addresses}
        selectedAddressId={selectedAddress?.id ?? null}
        onSelectAddress={(addr) => {
          setSelectedAddress(addr)
          rotateIdempotencyKey()
        }}
        onAddressCreated={(addr) => {
          setAddresses((prev) => [addr, ...prev.filter((item) => item.id !== addr.id)])
          setSelectedAddress(addr)
          rotateIdempotencyKey()
        }}
      />
    </div>
  )
}
