import { useEffect, useState } from "react"
import { Link, useSearchParams } from "react-router-dom"
import { Minus, Plus, ShieldAlert, ShoppingCart, Trash2 } from "lucide-react"
import { FREE_SHIPPING_THRESHOLD, STANDARD_SHIPPING_FEE } from "@/lib/shipping"
import { formatVND, cn } from "@/lib/utils"
import { ProductImage } from "@/components/product-image"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { useCart } from "@/hooks/use-cart"
import type { CartItem } from "@/lib/types"

export { FREE_SHIPPING_THRESHOLD, STANDARD_SHIPPING_FEE }

function isPurchasable(item: CartItem): boolean {
  return item.status === "AVAILABLE" || item.status === "LIMITED"
}

function CartItemRow({
  item,
  selected,
  onToggle,
  onUpdateQuantity,
  onRemove,
}: {
  item: CartItem
  selected: boolean
  onToggle: (itemId: string) => void
  onUpdateQuantity: (itemId: string, quantity: number) => void
  onRemove: (itemId: string) => void
}) {
  const [imageError, setImageError] = useState(false)
  const unavailable = !isPurchasable(item)
  const limited = item.status === "LIMITED"
  const overAvailableQuantity = limited && item.quantity > item.availableQuantity
  const isOrphanDeleted = !item.name.trim()
  const displayName = item.name.trim() || "Sản phẩm không còn tồn tại"
  const hasImage = Boolean(item.imageUrl) && !imageError

  useEffect(() => {
    setImageError(false)
  }, [item.imageUrl])

  const mediaContent = hasImage ? (
    <img
      src={item.imageUrl}
      alt={displayName}
      loading="lazy"
      className="h-full w-full object-cover"
      onError={() => setImageError(true)}
    />
  ) : (
    <ProductImage
      emoji={item.imageEmoji || "📦"}
      hue={[item.imageHue[0] ?? 200, item.imageHue[1] ?? 240]}
      seed={item.productId}
      label={displayName}
    />
  )

  return (
    <div
      className={cn(
        "flex gap-4 rounded-xl p-4 ring-1 ring-foreground/10 transition-opacity",
        unavailable ? "bg-muted/30" : "bg-card",
        (!selected || unavailable) && "opacity-70"
      )}
    >
      <label
        className={cn(
          "flex shrink-0 items-center",
          unavailable ? "cursor-not-allowed" : "cursor-pointer"
        )}
      >
        <Checkbox
          checked={selected}
          onCheckedChange={() => onToggle(item.itemId)}
          aria-label={`Chọn ${displayName}`}
          disabled={unavailable}
        />
      </label>

      {isOrphanDeleted ? (
        <div className="block size-24 shrink-0 self-center overflow-hidden rounded-lg ring-1 ring-foreground/10">
          {mediaContent}
        </div>
      ) : (
        <Link
          to={`/san-pham/${item.productId}`}
          className="block size-24 shrink-0 self-center overflow-hidden rounded-lg ring-1 ring-foreground/10"
        >
          {mediaContent}
        </Link>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-start justify-between gap-2">
          {isOrphanDeleted ? (
            <span className="line-clamp-2 text-sm font-medium text-muted-foreground">
              {displayName}
            </span>
          ) : (
            <Link
              to={`/san-pham/${item.productId}`}
              className="line-clamp-2 text-sm font-medium hover:text-primary"
            >
              {displayName}
            </Link>
          )}
          <Button
            variant="ghost"
            size="icon"
            className="size-7 shrink-0"
            aria-label={`Xóa ${displayName}`}
            onClick={() => onRemove(item.itemId)}
          >
            <Trash2 />
          </Button>
        </div>

        {item.variantLabel && (
          <p className="mt-1 text-xs text-muted-foreground">
            {item.variantLabel}
          </p>
        )}

        {item.status === "OUT_OF_STOCK" && (
          <p className="mt-2 text-xs font-semibold text-destructive">Hết hàng</p>
        )}
        {item.status === "UNAVAILABLE" && (
          <p className="mt-2 text-xs font-semibold text-muted-foreground">
            {isOrphanDeleted ? "Sản phẩm đã bị gỡ khỏi gian hàng" : "Không còn kinh doanh"}
          </p>
        )}
        {limited && (
          <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-amber-700 dark:text-amber-300">
            <span>Chỉ còn {item.availableQuantity} sản phẩm</span>
            {overAvailableQuantity && (
              <Button
                variant="outline"
                size="xs"
                onClick={() => onUpdateQuantity(item.itemId, item.availableQuantity)}
              >
                Đặt về {item.availableQuantity}
              </Button>
            )}
          </div>
        )}

        <div className="mt-auto flex flex-wrap items-center justify-between gap-2 pt-2">
          <div className="flex items-center">
            <Button
              variant="outline"
              size="icon-sm"
              aria-label={`Giảm số lượng ${displayName}`}
              disabled={unavailable || item.quantity <= 1}
              onClick={() => onUpdateQuantity(item.itemId, item.quantity - 1)}
            >
              <Minus />
            </Button>
            <span className="w-12 text-center text-sm font-semibold">{item.quantity}</span>
            <Button
              variant="outline"
              size="icon-sm"
              aria-label={`Tăng số lượng ${displayName}`}
              disabled={
                unavailable ||
                (item.availableQuantity > 0 && item.quantity >= item.availableQuantity)
              }
              onClick={() => onUpdateQuantity(item.itemId, item.quantity + 1)}
            >
              <Plus />
            </Button>
          </div>
          <div className="flex flex-col items-end">
            {!isOrphanDeleted && (item.originalPrice ?? 0) > item.unitPrice && (
              <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span className="line-through">{formatVND(item.originalPrice!)}</span>
                {item.quantity > 1 && <span>· {formatVND(item.unitPrice)} / sp</span>}
              </div>
            )}
            <span
              className={cn(
                "text-sm font-bold",
                isOrphanDeleted && item.unitPrice <= 0 ? "text-muted-foreground" : "text-red-600"
              )}
            >
              {isOrphanDeleted && item.unitPrice <= 0
                ? "—"
                : formatVND(item.unitPrice * item.quantity)}
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}

export default function CartPage() {
  const {
    items,
    allSelected,
    toggleAllSelected,
    toggleSelected,
    updateQuantity,
    removeItem,
    removeSelected,
    selectedItems,
    selectedCount,
    selectedTotal,
    degraded,
    forbidden,
    loading,
    loadError,
    retryLoad,
  } = useCart()
  const [searchParams, setSearchParams] = useSearchParams()
  const buyId = searchParams.get("buy")

  useEffect(() => {
    if (!buyId) return
    const timer = window.setTimeout(() => setSearchParams({}, { replace: true }), 4000)
    return () => window.clearTimeout(timer)
  }, [buyId, setSearchParams])

  if (forbidden) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-8">
        <h1 className="text-2xl font-bold">Giỏ hàng</h1>
        <div className="mt-6 flex flex-col items-center justify-center rounded-3xl bg-card px-6 py-16 text-center ring-1 ring-foreground/10">
          <ShieldAlert className="size-12 text-muted-foreground" />
          <p className="mt-4 max-w-md text-base font-medium">
            Tính năng giỏ hàng chỉ dành cho tài khoản khách hàng.
          </p>
          <Button className="mt-6" asChild>
            <Link to="/">Về trang chủ</Link>
          </Button>
        </div>
      </div>
    )
  }

  if (loadError && items.length === 0) {
    return (
      <div className="mx-auto flex max-w-7xl flex-col items-center justify-center px-4 py-24 text-center">
        <span className="text-5xl">⚠️</span>
        <h1 className="mt-4 text-xl font-bold">Không tải được giỏ hàng</h1>
        <p role="alert" className="mt-1 max-w-sm text-sm text-muted-foreground">
          {loadError.message}
        </p>
        <Button className="mt-6" onClick={() => void retryLoad()}>
          Thử lại
        </Button>
      </div>
    )
  }

  if (loading && items.length === 0) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-8" role="status">
        <p className="sr-only">Đang tải giỏ hàng...</p>
        <Skeleton className="h-8 w-40" />
        <Skeleton className="mt-2 h-4 w-56" />
        <div className="mt-6 grid gap-6 lg:grid-cols-[1fr_320px]">
          <div className="space-y-3">
            {Array.from({ length: 3 }).map((_, idx) => (
              <Skeleton key={idx} className="h-32 w-full rounded-xl" />
            ))}
          </div>
          <Skeleton className="h-72 w-full rounded-2xl" />
        </div>
      </div>
    )
  }

  if (items.length === 0) {
    return (
      <div className="mx-auto flex max-w-7xl flex-col items-center justify-center px-4 py-24 text-center">
        {degraded && (
          <div
            role="alert"
            className="mb-6 rounded-xl border border-amber-300/70 bg-amber-50 p-4 text-left text-sm text-amber-900 dark:border-amber-700/60 dark:bg-amber-950/30 dark:text-amber-100"
          >
            Một số thông tin giỏ hàng chưa được cập nhật. Vui lòng thử lại trước khi đặt hàng.
          </div>
        )}
        <span className="grid size-20 place-items-center rounded-full bg-muted">
          <ShoppingCart className="size-9 text-muted-foreground" />
        </span>
        <h1 className="mt-6 text-xl font-bold sm:text-2xl">Giỏ hàng đang trống</h1>
        <p className="mt-2 max-w-sm text-sm text-muted-foreground">
          Bạn chưa có sản phẩm nào trong giỏ. Khám phá hàng nghìn sản phẩm hấp dẫn ngay bây giờ nhé!
        </p>
        <Button size="lg" className="mt-6" asChild>
          <Link to="/">Tiếp tục mua sắm</Link>
        </Button>
      </div>
    )
  }

  const purchasableItems = items.filter(isPurchasable)
  const unavailableItems = items.filter((item) => !isPurchasable(item))
  const shippingFee =
    selectedTotal > 0
      ? selectedTotal >= FREE_SHIPPING_THRESHOLD
        ? 0
        : STANDARD_SHIPPING_FEE
      : 0
  const grandTotal = selectedTotal + shippingFee

  async function handleRemoveAllUnavailable() {
    for (const item of unavailableItems) {
      try {
        await removeItem(item.itemId)
      } catch {
        // Error notification is handled by the Axios response interceptor.
        break
      }
    }
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <h1 className="text-2xl font-bold">Giỏ hàng</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        {items.length} sản phẩm đang có · đã chọn {selectedCount} sản phẩm
      </p>

      {buyId && (
        <div className="mt-4 rounded-xl border border-primary/30 bg-primary/5 p-4 text-sm">
          Sản phẩm đã được thêm vào giỏ hàng của bạn. Kiểm tra và hoàn tất đơn hàng bên dưới nhé!
        </div>
      )}
      {degraded && (
        <div
          role="alert"
          className="mt-4 rounded-xl border border-amber-300/70 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-700/60 dark:bg-amber-950/30 dark:text-amber-100"
        >
          Một số thông tin giỏ hàng chưa được cập nhật. Vui lòng thử lại trước khi đặt hàng.
        </div>
      )}

      <div className="mt-6 grid gap-6 lg:grid-cols-[1fr_320px]">
        <div>
          <div className="flex items-center justify-between rounded-xl bg-card px-4 py-3 ring-1 ring-foreground/10">
            <label className="flex cursor-pointer items-center gap-2.5 text-sm font-medium">
              <Checkbox checked={allSelected} onCheckedChange={toggleAllSelected} />
              Chọn tất cả ({purchasableItems.length})
            </label>
            <Button
              variant="ghost"
              size="sm"
              disabled={selectedItems.length === 0 || loading}
              onClick={() => void removeSelected()}
            >
              <Trash2 />
              Xóa đã chọn
            </Button>
          </div>

          {purchasableItems.length > 0 && (
            <section className="mt-4" aria-labelledby="purchasable-heading">
              <h2
                id="purchasable-heading"
                className="mb-3 text-sm font-semibold text-muted-foreground"
              >
                Sản phẩm có thể đặt
              </h2>
              <div className="grid gap-3">
                {purchasableItems.map((item) => (
                  <CartItemRow
                    key={item.itemId}
                    item={item}
                    selected={selectedItems.some((selected) => selected.itemId === item.itemId)}
                    onToggle={toggleSelected}
                    onUpdateQuantity={(itemId, quantity) => void updateQuantity(itemId, quantity)}
                    onRemove={(itemId) => void removeItem(itemId)}
                  />
                ))}
              </div>
            </section>
          )}

          {unavailableItems.length > 0 && (
            <section
              className={cn(purchasableItems.length > 0 ? "mt-6" : "mt-4")}
              aria-labelledby="unavailable-heading"
            >
              <div className="mb-3 flex items-center justify-between gap-2">
                <h2
                  id="unavailable-heading"
                  className="text-sm font-semibold text-muted-foreground"
                >
                  Sản phẩm không khả dụng
                </h2>
                <Button
                  variant="ghost"
                  size="xs"
                  disabled={loading}
                  onClick={() => void handleRemoveAllUnavailable()}
                >
                  <Trash2 />
                  Xóa tất cả ({unavailableItems.length})
                </Button>
              </div>
              <div className="grid gap-3">
                {unavailableItems.map((item) => (
                  <CartItemRow
                    key={item.itemId}
                    item={item}
                    selected={false}
                    onToggle={toggleSelected}
                    onUpdateQuantity={(itemId, quantity) => void updateQuantity(itemId, quantity)}
                    onRemove={(itemId) => void removeItem(itemId)}
                  />
                ))}
              </div>
            </section>
          )}
        </div>

        <div className="h-fit rounded-2xl bg-card p-5 ring-1 ring-foreground/10 lg:sticky lg:top-24">
          <h2 className="text-base font-bold">Tóm tắt đơn hàng</h2>
          <Separator className="my-4" />
          <div className="grid gap-3 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Sản phẩm đã chọn</span>
              <span className="font-medium">{selectedCount}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Tạm tính</span>
              <span className="font-medium">{formatVND(selectedTotal)}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Phí vận chuyển</span>
              <span className={cn("font-medium", shippingFee === 0 && "text-emerald-600")}>
                {shippingFee === 0 ? "Miễn phí" : formatVND(shippingFee)}
              </span>
            </div>
          </div>
          <Separator className="my-4" />
          <div className="flex items-center justify-between">
            <span className="text-sm font-semibold">Tổng cộng</span>
            <span className="text-xl font-extrabold text-red-600">{formatVND(grandTotal)}</span>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            {selectedTotal > 0 && selectedTotal < FREE_SHIPPING_THRESHOLD
              ? `Mua thêm ${formatVND(FREE_SHIPPING_THRESHOLD - selectedTotal)} để được miễn phí vận chuyển`
              : selectedTotal > 0
                ? "Đơn hàng của bạn được miễn phí vận chuyển 🎉"
                : "Chọn ít nhất một sản phẩm để đặt hàng"}
          </p>
          {selectedCount > 0 ? (
            <Button size="lg" className="mt-4 w-full" asChild>
              <Link to="/gio-hang/dat-hang">Tiếp tục đặt hàng ({formatVND(grandTotal)})</Link>
            </Button>
          ) : (
            <Button size="lg" className="mt-4 w-full" disabled>
              Tiếp tục đặt hàng
            </Button>
          )}
          <Button variant="ghost" size="sm" className="mt-2 w-full" asChild>
            <Link to="/tim-kiem">Tiếp tục mua sắm</Link>
          </Button>
        </div>
      </div>
    </div>
  )
}
