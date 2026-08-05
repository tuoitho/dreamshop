# ecommerce-fe

Frontend SPA cho nền tảng e-commerce microservices. Kết nối fullstack với backend
qua Kong API Gateway: đăng ký, đăng nhập, đăng xuất và gọi API protected bằng JWT.

## Tech stack

- Vite + React 19 + TypeScript
- Tailwind CSS v4 + shadcn/ui
- React Router v7
- Axios (interceptor tự gắn Bearer token)
- Vitest + React Testing Library

## Chạy local

Prerequisite: backend đang chạy với Kong trên cổng `8000` (xem README gốc của monorepo,
`make up`).

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
| Protected health | `GET /api/v1/auth/actuator/health` (Bearer) |

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
├─ lib/             axios client, token storage, types, error map
├─ pages/           Login, Register, Dashboard
└─ routes/          RequireAuth guard
```

## Ghi chú bảo mật

- `npm audit` báo GHSA-qwww-vcr4-c8h2 (react-router RSC mode CSRF) nhưng advisory
  chỉ ảnh hưởng khi dùng React Server Components API; dự án này là SPA thuần nên
  không bị ảnh hưởng.
