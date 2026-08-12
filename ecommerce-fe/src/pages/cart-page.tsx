import { useEffect } from "react"
import { Link, useSearchParams } from "react-router-dom"
import { Minus, Plus, ShoppingCart, Trash2 } from "lucide-react"
import { formatVND, cn } from "@/lib/utils"
import { ProductImage } from "@/components/product-image"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Separator } from "@/components/ui/separator"
import { useCart } from "@/hooks/use-cart"
import type { CartItem } from "@/lib/types"

export const FREE_SHIPPING_THRESHOLD = 299000
export const STANDARD_SHIPPING_FEE = 25000

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
  const unavailable = !isPurchasable(item)
  const limited = item.status === "LIMITED"
  const overAvailableQuantity = limited && item.quantity > item.availableQuantity

  return (
    <div
      className={cn(
        "flex gap-4 rounded-xl bg-card p-4 ring-1 ring-foreground/10 transition-opacity",
        (!selected || unavailable) && "opacity-70"
      )}
    >
      <label className="flex cursor-pointer items-start pt-1">
        <Checkbox
          checked={selected}
          onCheckedChange={() => onToggle(item.itemId)}
          aria-label={`Chọn ${item.name}`}
          disabled={unavailable}
        />
      </label>

      <Link
        to={`/san-pham/${item.productId}`}
        className="block size-24 shrink-0 overflow-hidden rounded-lg ring-1 ring-foreground/10"
      >
        <ProductImage
          emoji={item.imageEmoji}
          hue={[item.imageHue[0] ?? 200, item.imageHue[1] ?? 240]}
          seed={item.productId}
          label={item.name}
        />
      </Link>

      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-start justify-between gap-2">
          <Link
            to={`/san-pham/${item.productId}`}
            className="line-clamp-2 text-sm font-medium hover:text-primary"
          >
            {item.name}
          </Link>
          <Button
            variant="ghost"
            size="icon"
            className="size-7 shrink-0"
            aria-label={`Xóa ${item.name}`}
            onClick={() => onRemove(item.itemId)}
          >
            <Trash2 />
          </Button>
        </div>

        {item.variantLabel && (
          <p className="mt-1 flex items-center gap-1.5 text-xs text-muted-foreground">
            {item.variantColorHex && (
              <span
                className="inline-block size-3 rounded-full border border-black/10"
                style={{ backgroundColor: item.variantColorHex }}
              />
            )}
            {item.variantLabel}
          </p>
        )}

        {item.status === "OUT_OF_STOCK" && (
          <p className="mt-2 text-xs font-semibold text-destructive">Hết hàng</p>
        )}
        {item.status === "UNAVAILABLE" && (
          <p className="mt-2 text-xs font-semibold text-muted-foreground">Không còn kinh doanh</p>
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
              aria-label={`Giảm số lượng ${item.name}`}
              disabled={unavailable || item.quantity <= 1}
              onClick={() => onUpdateQuantity(item.itemId, item.quantity - 1)}
            >
              <Minus />
            </Button>
            <span className="w-12 text-center text-sm font-semibold">{item.quantity}</span>
            <Button
              variant="outline"
              size="icon-sm"
              aria-label={`Tăng số lượng ${item.name}`}
              disabled={unavailable || (limited && item.quantity >= item.availableQuantity)}
              onClick={() => onUpdateQuantity(item.itemId, item.quantity + 1)}
            >
              <Plus />
            </Button>
          </div>
          <span className="text-sm font-bold text-red-600">
            {formatVND(item.unitPrice * item.quantity)}
          </span>
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

  if (loadError && items.length === 0) {
    return (
      <div className="mx-auto flex max-w-7xl flex-col items-center justify-center px-4 py-24 text-center">
        <span className="text-5xl">⚠️</span>
        <h1 className="mt-4 text-xl font-bold">Không tải được giỏ hàng</h1>
        <p role="alert" className="mt-1 max-w-sm text-sm text-muted-foreground">{loadError.message}</p>
        <Button className="mt-6" onClick={() => void retryLoad()}>Thử lại</Button>
      </div>
    )
  }

  if (loading && items.length === 0) {
    return (
      <div className="mx-auto flex max-w-7xl items-center justify-center px-4 py-24">
        <p className="text-sm text-muted-foreground" role="status">Đang tải giỏ hàng...</p>
      </div>
    )
  }

  if (items.length === 0) {
    return (
      <div className="mx-auto flex max-w-7xl flex-col items-center justify-center px-4 py-24 text-center">
        {degraded && (
          <div role="alert" className="mb-6 rounded-xl border border-amber-300/70 bg-amber-50 p-4 text-left text-sm text-amber-900 dark:border-amber-700/60 dark:bg-amber-950/30 dark:text-amber-100">
            Một số thông tin giỏ hàng chưa được cập nhật. Vui lòng thử lại trước khi đặt hàng.
          </div>
        )}
        <span className="grid size-20 place-items-center rounded-full bg-muted">
          <ShoppingCart className="size-9 text-muted-foreground" />
        </span>
        <h1 className="mt-6 text-xl font-bold sm:text-2xl">Giỏ hàng đang trống</h1>
        <p className="mt-2 max-w-sm text-sm text-muted-foreground">
          Bạn chưa có sản phẩm nào trong giỏ. Khám phá hàng nghìn sản phẩm hấp dẫn ngay
          bây giờ nhé!
        </p>
        <Button size="lg" className="mt-6" asChild>
          <Link to="/">Tiếp tục mua sắm</Link>
        </Button>
      </div>
    )
  }

  const purchasableItems = items.filter(isPurchasable)
  const unavailableItems = items.filter((item) => !isPurchasable(item))
  const shippingFee = selectedTotal > 0
    ? selectedTotal >= FREE_SHIPPING_THRESHOLD ? 0 : STANDARD_SHIPPING_FEE
    : 0
  const grandTotal = selectedTotal + shippingFee

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <h1 className="text-2xl font-bold">Giỏ hàng</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        {items.length} sản phẩm đang có · đã chọn {selectedCount} sản phẩm
      </p>

      {buyId && (
        <div className="mt-4 rounded-xl border border-primary/30 bg-primary/5 p-4 text-sm">
          Sản phẩm đã được thêm vào giỏ hàng của bạn. Kiểm tra và hoàn tất đơn hàng bên
          dưới nhé!
        </div>
      )}
      {degraded && (
        <div role="alert" className="mt-4 rounded-xl border border-amber-300/70 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-700/60 dark:bg-amber-950/30 dark:text-amber-100">
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
            <Button variant="ghost" size="sm" onClick={() => void removeSelected()}>
              <Trash2 />
              Xóa đã chọn
            </Button>
          </div>

          {purchasableItems.length > 0 && (
            <section className="mt-4" aria-labelledby="purchasable-heading">
              <h2 id="purchasable-heading" className="mb-3 text-sm font-semibold text-muted-foreground">
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
            <section className={cn(purchasableItems.length > 0 && "mt-6")} aria-labelledby="unavailable-heading">
              <div className="rounded-2xl border border-dashed border-foreground/15 bg-muted/30 p-4">
                <h2 id="unavailable-heading" className="text-sm font-semibold">Sản phẩm không khả dụng</h2>
                <div className="mt-3 grid gap-3">
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
