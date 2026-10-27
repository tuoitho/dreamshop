# ecommerce-fe

Frontend SPA cho nền tảng e-commerce microservices. Browser chỉ gọi backend qua
Kong API Gateway. Ứng dụng hiện hỗ trợ storefront, tìm kiếm/lọc sản phẩm, chi
tiết sản phẩm, xác thực, hồ sơ người dùng, giỏ hàng và xem trước đơn hàng.

## Tech stack

- Vite 8 + React 19 + TypeScript 6
- Tailwind CSS 4 + Radix/shadcn components
- React Router 7
- Axios; Bearer access token chỉ lưu in-memory, refresh cookie dùng `HttpOnly`
- Zod, Sonner, Lucide React
- Vitest 4 + React Testing Library

## Chạy local

Để chạy toàn bộ storefront/cart flow, khởi động infrastructure, Identity,
Catalog, Inventory, Cart và BFF theo
[`docs/instructions/local-development.md`](../docs/instructions/local-development.md).

```bash
cp .env.example .env   # nếu chưa có
npm install
npm run dev
```

Mở `http://localhost:5173`.

Khi chạy ở chế độ `development` (`npm run dev`), trang `/login` điền sẵn tài
khoản `CUSTOMER` local (`customer1@gmail.com` / `123456123456`) và hiển thị nút
`DEV` để điền lại nhanh tài khoản khách hàng. Storefront chỉ cho phép tài khoản
thuần `CUSTOMER` đăng nhập; cụm preset `DEV` tự động bị loại bỏ khi build
production (`npm run build`). Nếu chạy trên database mới chưa có
`customer1@gmail.com`, xem lệnh khởi tạo tại
[`docs/instructions/local-development.md`](../docs/instructions/local-development.md#local-test-accounts).

## Cấu hình

| Biến | Mặc định | Mô tả |
|---|---|---|
| `VITE_API_BASE_URL` | `http://localhost:8000/api/v1` | Base URL của API qua Kong |

## Routes

| Route | Chức năng |
|---|---|
| `/` | Trang chủ và sản phẩm nổi bật |
| `/tim-kiem` | Tìm kiếm, lọc, sắp xếp và phân trang |
| `/san-pham/:id` | Chi tiết sản phẩm, thêm/mua ngay và thao tác ẩn/khôi phục sản phẩm cho admin |
| `/gio-hang` | Giỏ hàng có xác thực |
| `/gio-hang/dat-hang` | Xem trước line item đã chọn (yêu cầu đăng nhập); chưa submit Order |
| `/login`, `/register` | Xác thực |
| `/profile` (`/dashboard` redirect) | Xem và cập nhật hồ sơ (yêu cầu đăng nhập) |

## API được gọi

| Thao tác | Endpoint (qua Kong) |
|---|---|
| Register | `POST /api/v1/auth/register` |
| Login | `POST /api/v1/auth/login` |
| Refresh | `POST /api/v1/auth/refresh` |
| Logout | `POST /api/v1/auth/logout` |
| Profile | `GET/PATCH /api/v1/users/me` |
| Locations | `GET /api/v1/locations/provinces` và `/provinces/{code}/wards` |
| Product search/detail | `GET /api/v1/storefront/products[/{id}]` |
| Admin product soft-delete/restore | `POST /api/v1/admin/products/{id}/delete` |
| Cart | `GET/DELETE /api/v1/cart` |
| Cart items | `POST /api/v1/cart/items`, `PATCH/DELETE /api/v1/cart/items/{itemId}` |

## Scripts

```bash
npm run dev      # dev server
npm run build    # typecheck + production build
npm run lint     # oxlint
npm run test     # vitest run
npm run preview  # preview production build
```

## Cấu trúc

```
src/
├─ components/      layout, product presentation, and reusable UI components
├─ context/         Auth and Cart providers/contexts
├─ hooks/           typed auth and cart hooks
├─ lib/             API client, in-memory session, types, and validation
├─ pages/           storefront, auth/profile, cart, and order preview pages
└─ routes/          RequireAuth guard
```

## Ghi chú bảo mật

- Access token không được ghi vào `localStorage`; khi reload, provider gọi refresh endpoint bằng cookie `HttpOnly` để lấy access token mới.
- Các protected request chỉ refresh/retry một lần; refresh concurrent dùng một shared promise.

## Current limitations

- Order preview chỉ tính và hiển thị các line item đã chọn; chưa tạo đơn hàng.
- Cart selection được lưu local theo user, nhưng cart lines là dữ liệu server
  trong `cart-service`.
- Production build hiện phát cảnh báo main JavaScript chunk lớn hơn `500 kB`;
  route-level code splitting chưa được cấu hình.

- `npm audit` báo GHSA-qwww-vcr4-c8h2 (react-router RSC mode CSRF) nhưng advisory
  chỉ ảnh hưởng khi dùng React Server Components API; dự án này là SPA thuần nên
  không bị ảnh hưởng.
