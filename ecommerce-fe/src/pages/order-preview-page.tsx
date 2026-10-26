import { useContext, useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { AlertTriangle, ArrowLeft, MapPin, ShoppingBag } from "lucide-react"
import { toast } from "sonner"
import { AuthContext } from "@/context/auth-context"
import { useCart } from "@/hooks/use-cart"
import { fetchProvinces, fetchWards } from "@/lib/locations"
import { FREE_SHIPPING_THRESHOLD, STANDARD_SHIPPING_FEE } from "@/lib/shipping"
import { ProductImage } from "@/components/product-image"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { formatVND } from "@/lib/utils"
import type { CartItem } from "@/lib/types"

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
  const { selectedItems } = useCart()

  const [provinceName, setProvinceName] = useState<string>("")
  const [wardName, setWardName] = useState<string>("")

  useEffect(() => {
    const pCode = user?.provinceCode
    const wCode = user?.wardCode
    if (!pCode) {
      setProvinceName("")
      setWardName("")
      return
    }
    let active = true
    fetchProvinces()
      .then((list) => {
        if (active) {
          setProvinceName(list.find((entry) => entry.code === pCode)?.name ?? "")
        }
      })
      .catch(() => {
        if (active) setProvinceName("")
      })

    if (wCode) {
      fetchWards(pCode)
        .then((list) => {
          if (active) {
            setWardName(list.find((entry) => entry.code === wCode)?.name ?? "")
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
  }, [user?.provinceCode, user?.wardCode])

  const validSelectedItems = selectedItems.filter(isPurchasable)
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

  const addressParts = [user?.addressLine, wardName, provinceName].filter(
    (part): part is string => Boolean(part && part.trim())
  )
  const formattedAddress = addressParts.join(", ")
  const hasCompleteShipping = Boolean(
    user?.phone && user.provinceCode && user.wardCode && user.addressLine
  )

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <Button variant="ghost" size="sm" className="-ml-2" asChild>
        <Link to="/gio-hang">
          <ArrowLeft /> Quay lại giỏ hàng
        </Link>
      </Button>
      <h1 className="mt-4 text-2xl font-bold">Xác nhận đơn hàng</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        Kiểm tra các sản phẩm đã chọn trước khi đặt hàng.
      </p>

      <div className="mt-6 grid gap-6 lg:grid-cols-[1fr_320px]">
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
                <Link
                  to="/profile"
                  className="text-xs font-medium text-primary hover:underline"
                >
                  {hasCompleteShipping ? "Thay đổi địa chỉ" : "Cập nhật địa chỉ giao hàng"}
                </Link>
              </div>
              <div className="mt-3 text-sm">
                <p className="font-medium">
                  <span>{user.fullName.trim() || user.email}</span>
                  {user.phone ? (
                    <span className="ml-2 font-normal text-muted-foreground">
                      · {user.phone}
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
                      Vui lòng bổ sung đầy đủ số điện thoại và địa chỉ giao hàng trong hồ sơ.
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
                        <p className="mt-0.5 text-xs text-muted-foreground">
                          {formatVND(item.unitPrice)} × {item.quantity}
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
          <Button
            size="lg"
            className="mt-4 w-full"
            onClick={() => toast.info("Chức năng đặt hàng chưa khả dụng")}
          >
            Đặt hàng
          </Button>
        </section>
      </div>
    </div>
  )
}
