import { Link } from "react-router-dom"
import { ShieldCheck, Truck, RotateCcw, Headphones } from "lucide-react"
import { categories } from "@/data/products"

const SERVICE_ITEMS = [
  { icon: Truck, title: "Giao hàng nhanh", desc: "Miễn phí từ 299.000đ" },
  { icon: ShieldCheck, title: "Chính hãng 100%", desc: "Bảo hành toàn quốc" },
  { icon: RotateCcw, title: "Đổi trả 30 ngày", desc: "Hoàn tiền nhanh chóng" },
  { icon: Headphones, title: "Hỗ trợ 24/7", desc: "Hotline 1900-6789" },
]

export function SiteFooter() {
  return (
    <footer className="border-t bg-card">
      <div className="mx-auto max-w-7xl px-4 py-10">
        <div className="grid gap-8 sm:grid-cols-2 lg:grid-cols-4">
          {SERVICE_ITEMS.map((item) => (
            <div key={item.title} className="flex items-start gap-3">
              <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
                <item.icon className="size-5" />
              </span>
              <div>
                <p className="text-sm font-semibold">{item.title}</p>
                <p className="text-xs text-muted-foreground">{item.desc}</p>
              </div>
            </div>
          ))}
        </div>

        <div className="mt-10 grid gap-8 border-t pt-8 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <div className="flex items-center gap-2">
              <span className="grid size-7 place-items-center rounded-lg bg-primary text-sm font-extrabold text-primary-foreground">
                S
              </span>
              <span className="text-base font-extrabold tracking-tight">S-Shop</span>
            </div>
            <p className="mt-3 text-xs leading-relaxed text-muted-foreground">
              Nền tảng mua sắm trực tuyến hiện đại với hàng nghìn sản phẩm chính hãng, giá tốt
              và giao hàng tận nơi trên toàn quốc.
            </p>
          </div>

          <div>
            <p className="text-sm font-semibold">Danh mục</p>
            <ul className="mt-3 grid gap-2">
              {categories.slice(0, 6).map((category) => (
                <li key={category.id}>
                  <Link
                    to={`/tim-kiem?danh-muc=${category.id}`}
                    className="text-xs text-muted-foreground hover:text-primary"
                  >
                    {category.name}
                  </Link>
                </li>
              ))}
            </ul>
          </div>

          <div>
            <p className="text-sm font-semibold">Hỗ trợ khách hàng</p>
            <ul className="mt-3 grid gap-2 text-xs text-muted-foreground">
              <li><span className="cursor-pointer hover:text-primary">Trung tâm trợ giúp</span></li>
              <li><span className="cursor-pointer hover:text-primary">Chính sách đổi trả</span></li>
              <li><span className="cursor-pointer hover:text-primary">Hướng dẫn mua hàng</span></li>
              <li><span className="cursor-pointer hover:text-primary">Phương thức thanh toán</span></li>
            </ul>
          </div>

          <div>
            <p className="text-sm font-semibold">Liên hệ</p>
            <ul className="mt-3 grid gap-2 text-xs text-muted-foreground">
              <li>Hotline: 1900 6789</li>
              <li>Email: hotro@sshop.vn</li>
              <li>Địa chỉ: 123 Nguyễn Trãi, TP. HCM</li>
            </ul>
          </div>
        </div>

        <div className="mt-8 flex flex-col items-center justify-between gap-2 border-t pt-6 sm:flex-row">
          <p className="text-xs text-muted-foreground">
            © {new Date().getFullYear()} S-Shop. Tất cả quyền được bảo lưu.
          </p>
          <div className="flex items-center gap-3 text-xs text-muted-foreground">
            <span className="rounded border px-1.5 py-0.5">VISA</span>
            <span className="rounded border px-1.5 py-0.5">Mastercard</span>
            <span className="rounded border px-1.5 py-0.5">MoMo</span>
            <span className="rounded border px-1.5 py-0.5">VNPay</span>
          </div>
        </div>
      </div>
    </footer>
  )
}
