import { useEffect } from "react"
import { Link, useSearchParams } from "react-router-dom"
import { Minus, Plus, ShoppingCart, Trash2 } from "lucide-react"
import { formatVND, cn } from "@/lib/utils"
import { ProductImage } from "@/components/product-image"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Separator } from "@/components/ui/separator"
import { useCart } from "@/hooks/use-cart"

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
    clearCart,
  } = useCart()

  const [searchParams, setSearchParams] = useSearchParams()

  // "Mua ngay" từ trang sản phẩm: chuyển hướng kèm ?buy=<id> — chúng ta hiển thị thông báo
  const buyId = searchParams.get("buy")

  useEffect(() => {
    if (buyId) {
      const timer = window.setTimeout(() => {
        setSearchParams({}, { replace: true })
      }, 4000)
      return () => window.clearTimeout(timer)
    }
  }, [buyId, setSearchParams])

  if (items.length === 0) {
    return (
      <div className="mx-auto flex max-w-7xl flex-col items-center justify-center px-4 py-24 text-center">
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

  const shippingFee = selectedTotal > 0 ? (selectedTotal >= 299000 ? 0 : 25000) : 0
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

      <div className="mt-6 grid gap-6 lg:grid-cols-[1fr_320px]">
        <div>
          {/* Select all bar */}
          <div className="flex items-center justify-between rounded-xl bg-card px-4 py-3 ring-1 ring-foreground/10">
            <label className="flex cursor-pointer items-center gap-2.5 text-sm font-medium">
              <Checkbox checked={allSelected} onCheckedChange={toggleAllSelected} />
              Chọn tất cả ({items.length})
            </label>
            <Button variant="ghost" size="sm" onClick={removeSelected}>
              <Trash2 />
              Xóa đã chọn
            </Button>
          </div>

          {/* Items */}
          <div className="mt-4 grid gap-3">
            {items.map((item) => {
              return (
                <div
                  key={item.itemId}
                  className={cn(
                    "flex gap-4 rounded-xl bg-card p-4 ring-1 ring-foreground/10 transition-opacity",
                    !selectedItems.some((selected) => selected.itemId === item.itemId) && "opacity-70"
                  )}
                >
                  <label className="flex cursor-pointer items-start pt-1">
                    <Checkbox
                      checked={selectedItems.some((selected) => selected.itemId === item.itemId)}
                      onCheckedChange={() => toggleSelected(item.itemId)}
                      aria-label={`Chọn ${item.name}`}
                      disabled={item.status === "OUT_OF_STOCK" || item.status === "UNAVAILABLE"}
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
                         onClick={() => void removeItem(item.itemId)}
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

                    <div className="mt-auto flex flex-wrap items-center justify-between gap-2 pt-2">
                      <div className="flex items-center">
                        <Button
                          variant="outline"
                          size="icon-sm"
                          aria-label="Giảm số lượng"
                          onClick={() =>
                            void updateQuantity(item.itemId, item.quantity - 1)
                          }
                        >
                          <Minus />
                        </Button>
                        <span className="w-12 text-center text-sm font-semibold">
                          {item.quantity}
                        </span>
                        <Button
                          variant="outline"
                          size="icon-sm"
                          aria-label="Tăng số lượng"
                          onClick={() =>
                            void updateQuantity(item.itemId, item.quantity + 1)
                          }
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
            })}
          </div>
        </div>

        {/* Summary */}
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
            {selectedTotal > 0 && selectedTotal < 299000
              ? `Mua thêm ${formatVND(299000 - selectedTotal)} để được miễn phí vận chuyển`
              : selectedTotal > 0
                ? "Đơn hàng của bạn được miễn phí vận chuyển 🎉"
                : "Chọn ít nhất một sản phẩm để thanh toán"}
          </p>
          <Button
            size="lg"
            className="mt-4 w-full"
            disabled={selectedCount === 0}
            onClick={async () => {
              await clearCart()
              window.alert("Đặt hàng thành công! Cảm ơn bạn đã mua sắm tại S-Shop 🎉")
            }}
          >
            Thanh toán ({formatVND(grandTotal)})
          </Button>
          <Button variant="ghost" size="sm" className="mt-2 w-full" asChild>
            <Link to="/tim-kiem">Tiếp tục mua sắm</Link>
          </Button>
        </div>
      </div>
    </div>
  )
}
