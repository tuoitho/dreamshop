# Design: ecommerce-fe (Slice 1 — Auth)

## Context

Monorepo microservice đã có Identity service (register/login/JWKS hoạt động),
được public qua Kong API Gateway tại `http://localhost:8000/api/v1/`. Cần một
frontend React kết nối fullstack tới các API này, bắt đầu từ luồng xác thực.

## Quyết định

- **Vị trí:** `ecommerce-fe/` ở thư mục gốc của monorepo.
- **Stack:** Vite + React 18 + TypeScript + Tailwind CSS v4 + shadcn/ui +
  React Router v7 + Axios.
- **API qua Kong:** `VITE_API_BASE_URL=http://localhost:8000/api/v1`.

## API được gọi (qua Kong)

| Thao tác | Endpoint | Body | Response |
|---|---|---|---|
| Register | `POST /api/v1/auth/register` | `{email, password, full_name}` | 201 `{user_id, email, full_name, roles}` |
| Login | `POST /api/v1/auth/login` | `{email, password}` | `{access_token, token_type, expires_in}` |
| Protected ví dụ | `GET /api/v1/auth/actuator/health` | — | health |

## Cấu trúc

```
ecommerce-fe/
├─ src/
│  ├─ lib/            axios client (baseURL Kong), token storage, auth helpers
│  ├─ components/     shadcn/ui components + ui primitives
│  ├─ hooks/          useAuth, useAxios
│  ├─ pages/          Login, Register, Dashboard (protected), Home
│  ├─ routes/         <RequireAuth> guard
│  └─ main.tsx / App.tsx
├─ .env.example
└─ vite.config.ts
```

## Data flow

1. Login/Register page → validate form (email, password ≥12 chars) → gọi API.
2. Thành công: lưu `access_token`, gọi auth callback → navigate tới Dashboard.
3. Dashboard (protected): hiển thị trạng thái đăng nhập, một ví dụ gọi API có
   Bearer token, nút Logout.
4. Axios interceptor tự gắn `Authorization: Bearer <token>`; bắt lỗi
   400/401/409 và dịch `ApiError {code, message, trace_id}` thành thông báo UI.
5. RequireAuth redirect về `/login` nếu chưa có token.

## Error handling

- Dùng `ApiError` envelope từ backend (`VALIDATION_FAILED`,
  `EMAIL_ALREADY_REGISTERED`, `INVALID_CREDENTIALS`...) hiển thị `message`.
- Tách biệt lỗi mạng / backend-down.

## Testing

- Vitest + React Testing Library cho unit test cơ bản (form validation,
  auth helper logic, axios interceptor).

## Out of scope (slice này)

- Refresh token (backend chưa có), catalog/storefront CRUD, cart/checkout.
- Chỉ gồm auth + protected route + 1 API mẫu có token.
