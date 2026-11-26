import { Link } from "react-router-dom"
import { BrandLogo } from "@/components/layout/brand-logo"
import { categories } from "@/data/products"

export function SiteFooter() {
  return (
    <footer className="bg-surface-ink text-[#d8e6d5]">
      <div className="mx-auto max-w-7xl px-4 py-14 sm:py-16">
        <div className="grid gap-10 sm:grid-cols-2 lg:grid-cols-[1.4fr_1fr_1fr_1fr]">
          <div>
            <Link to="/" className="inline-flex items-center gap-2.5">
              <BrandLogo className="size-8" />
              <span className="font-serif text-xl font-bold tracking-tight text-white">
                DreamShop
              </span>
            </Link>
            <p className="mt-4 max-w-xs text-sm leading-relaxed text-[#a3b89f]">
              Nền tảng mua sắm trực tuyến tinh chọn với sản phẩm chính hãng, trải nghiệm mua sắm
              chỉn chu và giao hàng tận nơi trên toàn quốc.
            </p>
          </div>

          <div>
            <p className="font-serif text-base font-semibold text-white">Danh mục</p>
            <ul className="mt-4 grid gap-2.5">
              {categories.slice(0, 6).map((category) => (
                <li key={category.id}>
                  <Link
                    to={`/tim-kiem?danh-muc=${category.id}`}
                    className="text-sm text-[#a3b89f] transition-colors hover:text-[#bef264]"
                  >
                    {category.name}
                  </Link>
                </li>
              ))}
            </ul>
          </div>

          <div>
            <p className="font-serif text-base font-semibold text-white">Hỗ trợ khách hàng</p>
            <ul className="mt-4 grid gap-2.5 text-sm text-[#a3b89f]">
              <li>
                <Link to="/tim-kiem" className="transition-colors hover:text-[#bef264]">
                  Trung tâm trợ giúp
                </Link>
              </li>
              <li>
                <Link to="/tim-kiem?giam-gia=1" className="transition-colors hover:text-[#bef264]">
                  Chương trình khuyến mãi
                </Link>
              </li>
              <li>
                <Link to="/gio-hang" className="transition-colors hover:text-[#bef264]">
                  Kiểm tra giỏ hàng
                </Link>
              </li>
              <li>
                <Link to="/profile" className="transition-colors hover:text-[#bef264]">
                  Quản lý tài khoản
                </Link>
              </li>
            </ul>
          </div>

          <div>
            <p className="font-serif text-base font-semibold text-white">Liên hệ</p>
            <ul className="mt-4 grid gap-2.5 text-sm text-[#a3b89f]">
              <li>Hotline: 1900 6789</li>
              <li>Email: hotro@dreamshop.vn</li>
              <li>Địa chỉ: 123 Nguyễn Trãi, TP. HCM</li>
            </ul>
          </div>
        </div>

        <div className="mt-12 flex flex-col items-center justify-between gap-4 border-t border-white/10 pt-6 sm:flex-row">
          <p className="text-xs text-[#869e82]">
            © {new Date().getFullYear()} DreamShop. Tất cả quyền được bảo lưu.
          </p>
          <div className="flex items-center gap-2.5 text-[11px] font-medium text-[#a3b89f]">
            <span className="rounded border border-white/15 px-2 py-0.5">VISA</span>
            <span className="rounded border border-white/15 px-2 py-0.5">Mastercard</span>
            <span className="rounded border border-white/15 px-2 py-0.5">MoMo</span>
            <span className="rounded border border-white/15 px-2 py-0.5">VNPay</span>
          </div>
        </div>
      </div>
    </footer>
  )
}
