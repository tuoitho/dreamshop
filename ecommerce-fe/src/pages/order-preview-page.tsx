import { Link } from "react-router-dom"
import { ArrowLeft, ShoppingBag } from "lucide-react"
import { toast } from "sonner"
import { useCart } from "@/hooks/use-cart"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { formatVND } from "@/lib/utils"
import type { CartItem } from "@/lib/types"

const FREE_SHIPPING_THRESHOLD = 299000
const STANDARD_SHIPPING_FEE = 25000

function isPurchasable(item: CartItem): boolean {
  return item.status === "AVAILABLE" || item.status === "LIMITED"
}

export default function OrderPreviewPage() {
  const { selectedItems } = useCart()
  const validSelectedItems = selectedItems.filter(isPurchasable)
  const subtotal = validSelectedItems.reduce((total, item) => total + item.unitPrice * item.quantity, 0)
  const shippingFee = subtotal > 0
    ? subtotal >= FREE_SHIPPING_THRESHOLD ? 0 : STANDARD_SHIPPING_FEE
    : 0
  const total = subtotal + shippingFee

  if (validSelectedItems.length === 0) {
    return (
      <div className="mx-auto flex max-w-3xl flex-col items-center justify-center px-4 py-24 text-center">
        <span className="grid size-20 place-items-center rounded-full bg-muted">
          <ShoppingBag className="size-9 text-muted-foreground" />
        </span>
        <h1 className="mt-6 text-xl font-bold sm:text-2xl">Chưa có sản phẩm hợp lệ để đặt hàng</h1>
        <p className="mt-2 max-w-sm text-sm text-muted-foreground">
          Hãy quay lại giỏ hàng và chọn sản phẩm còn có thể đặt.
        </p>
        <Button size="lg" className="mt-6" asChild>
          <Link to="/gio-hang">Quay lại giỏ hàng</Link>
        </Button>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <Button variant="ghost" size="sm" className="-ml-2" asChild>
        <Link to="/gio-hang"><ArrowLeft /> Quay lại giỏ hàng</Link>
      </Button>
      <h1 className="mt-4 text-2xl font-bold">Xác nhận đơn hàng</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        Kiểm tra các sản phẩm đã chọn trước khi đặt hàng.
      </p>

      <div className="mt-6 grid gap-6 lg:grid-cols-[1fr_320px]">
        <section className="rounded-2xl bg-card p-5 ring-1 ring-foreground/10" aria-labelledby="preview-items-heading">
          <h2 id="preview-items-heading" className="text-base font-bold">Sản phẩm đã chọn</h2>
          <div className="mt-4 divide-y divide-border">
            {validSelectedItems.map((item) => (
              <div key={item.itemId} className="flex items-center justify-between gap-4 py-4 first:pt-0 last:pb-0">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{item.name}</p>
                  {item.variantLabel && <p className="mt-1 text-xs text-muted-foreground">{item.variantLabel}</p>}
                  <p className="mt-1 text-xs text-muted-foreground">Số lượng: {item.quantity}</p>
                </div>
                <span className="shrink-0 text-sm font-semibold">{formatVND(item.unitPrice * item.quantity)}</span>
              </div>
            ))}
          </div>
        </section>

        <section className="h-fit rounded-2xl bg-card p-5 ring-1 ring-foreground/10 lg:sticky lg:top-24" aria-labelledby="preview-summary-heading">
          <h2 id="preview-summary-heading" className="text-base font-bold">Tóm tắt đơn hàng</h2>
          <Separator className="my-4" />
          <div className="grid gap-3 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Tạm tính</span>
              <span className="font-medium">{formatVND(subtotal)}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Phí vận chuyển</span>
              <span className={shippingFee === 0 ? "font-medium text-emerald-600" : "font-medium"}>
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
