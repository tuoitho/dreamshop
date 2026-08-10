# ecommerce-fe

Frontend SPA cho nền tảng e-commerce microservices. Kết nối fullstack với backend
qua Kong API Gateway: đăng ký, đăng nhập, khôi phục session, cập nhật profile và logout.

## Tech stack

- Vite + React 19 + TypeScript
- Tailwind CSS v4 + shadcn/ui
- React Router v7
- Axios (Bearer access token chỉ lưu in-memory, refresh cookie HttpOnly)
- Vitest + React Testing Library

## Chạy local

Prerequisite: chạy `make identity-keys`, `make infra-up`, và `make identity` từ root repository.

```bash
cp .env.example .env   # nếu chưa có
npm install
npm run dev
```

Mở `http://localhost:5173`.

## Cấu hình

| Biến | Mặc định | Mô tả |
|---|---|---|
| `VITE_API_BASE_URL` | `http://localhost:8000/api/v1` | Base URL của API qua Kong |

## API được gọi

| Thao tác | Endpoint (qua Kong) |
|---|---|
| Register | `POST /api/v1/auth/register` |
| Login | `POST /api/v1/auth/login` |
| Refresh | `POST /api/v1/auth/refresh` |
| Logout | `POST /api/v1/auth/logout` |
| Profile | `GET/PATCH /api/v1/users/me` |

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
├─ components/ui/   shadcn/ui components
├─ context/         AuthProvider + AuthContext
├─ hooks/           useAuth
├─ lib/             axios client, in-memory session, types, error map
├─ pages/           Login, Register, Profile
└─ routes/          RequireAuth guard
```

## Ghi chú bảo mật

- Access token không được ghi vào `localStorage`; khi reload, provider gọi refresh endpoint bằng cookie `HttpOnly` để lấy access token mới.
- Các protected request chỉ refresh/retry một lần; refresh concurrent dùng một shared promise.

- `npm audit` báo GHSA-qwww-vcr4-c8h2 (react-router RSC mode CSRF) nhưng advisory
  chỉ ảnh hưởng khi dùng React Server Components API; dự án này là SPA thuần nên
  không bị ảnh hưởng.
