# Ecommerce Frontend (Auth Slice) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Xây dựng `ecommerce-fe` — SPA React kết nối fullstack tới Identity service qua Kong (register/login) với protected routes và gọi API có Bearer token.

**Architecture:** Vite SPA + React Router. Axios client dùng base URL Kong (`/api/v1`), interceptor tự gắn Bearer token và dịch `ApiError` thành lỗi hiển thị. Auth state tập trung trong React context.

**Tech Stack:** Vite, React 18, TypeScript, Tailwind CSS v4, shadcn/ui (Radix), React Router v7, Axios, Vitest + React Testing Library.

**Spec:** `docs/superpowers/specs/2026-08-05-ecommerce-frontend-auth-design.md`

---

### Task 1: Scaffold Vite React-TS trong ecommerce-fe

**Files:**
- Create: `ecommerce-fe/` (được sinh bởi Vite)

- [ ] **Step 1: Chạy lệnh scaffold Vite**

```bash
cd /e/microservice && npm create vite@latest ecommerce-fe -- --template react-ts
```

- [ ] **Step 2: Xoá boilerplate không cần (App.css, assets react.svg, logo)**

Xoá `src/App.css`, `src/assets/`, nội dung mặc định trong `src/App.tsx`.

- [ ] **Step 3: Commit**

```bash
git add ecommerce-fe
git commit -m "feat(frontend): scaffold vite react-ts app"
```

### Task 2: Cài dependency và cấu hình Tailwind v4 + shadcn/ui

**Files:**
- Modify: `ecommerce-fe/src/index.css`
- Modify: `ecommerce-fe/vite.config.ts`
- Create: `ecommerce-fe/src/lib/utils.ts`

- [ ] **Step 1: Cài dependencies**

```bash
cd /e/microservice/ecommerce-fe
npm install
npm install react-router-dom axios
npm install -D tailwindcss @tailwindcss/vite vitest jsdom @testing-library/react @testing-library/jest-dom @testing-library/user-event @vitejs/plugin-react
npx shadcn@latest init
npx shadcn@latest add button input label card
```

- [ ] **Step 2: Cấu hình Tailwind v4 plugin trong `vite.config.ts`**

```ts
import path from "path";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
});
```

- [ ] **Step 3: Thêm alias test config + setup vi test**

- [ ] **Step 4: Commit**

### Task 3: lib — axios client, token storage, types, error map

**Files:**
- Create: `ecommerce-fe/src/lib/types.ts`
- Create: `ecommerce-fe/src/lib/token-storage.ts`
- Create: `ecommerce-fe/src/lib/api.ts`
- Test: `ecommerce-fe/src/lib/token-storage.test.ts`, `ecommerce-fe/src/lib/api.test.ts`

- [ ] **Step 1: Viết failing test cho token-storage**
- [ ] **Step 2: Chạy test → fail**
- [ ] **Step 3: Implement token-storage + types**
- [ ] **Step 4: Implement api.ts (axios instance + interceptor + ApiError translate)**
- [ ] **Step 5: Test pass**
- [ ] **Step 6: Commit**

### Task 4: Auth context/hook

**Files:**
- Create: `ecommerce-fe/src/context/auth-context.tsx`
- Create: `ecommerce-fe/src/hooks/use-auth.ts`

- [ ] **Step 1: Implement AuthProvider (login/register/logout) dùng api client**
- [ ] **Step 2: Commit**

### Task 5: Pages + Routing + RequireAuth

**Files:**
- Create: `ecommerce-fe/src/pages/login-page.tsx`
- Create: `ecommerce-fe/src/pages/register-page.tsx`
- Create: `ecommerce-fe/src/pages/dashboard-page.tsx`
- Create: `ecommerce-fe/src/routes/require-auth.tsx`
- Modify: `ecommerce-fe/src/App.tsx`, `ecommerce-fe/src/main.tsx`

- [ ] **Step 1: Tạo App layout + routing**
- [ ] **Step 2: Tạo Login page (shadcn form)**
- [ ] **Step 3: Tạo Register page**
- [ ] **Step 4: Tạo Dashboard protected + gọi API có token**
- [ ] **Step 5: RequireAuth guard**
- [ ] **Step 6: Commit**

### Task 6: Vitest config + test form/guard

- [ ] **Step 1: Cấu hình vitest**
- [ ] **Step 2: Test form validation**
- [ ] **Step 3: Chạy test/vitest pass + build**
- [ ] **Step 4: Commit**

### Task 7: .env.example + README

- [ ] **Step 1: Tạo `.env.example` với `VITE_API_BASE_URL`**
- [ ] **Step 2: Tạo `ecommerce-fe/README.md`**
- [ ] **Step 3: Commit**

---
