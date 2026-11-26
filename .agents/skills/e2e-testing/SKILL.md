---
name: e2e-testing
description: 'Quy chuẩn Playwright E2E và khóa hợp đồng REST HTTP 2 chiều cho ecommerce-fe (:5173) và admin-fe (:5174). Gồm Mode 2 (Shape-Locked Mock E2E không cần dựng full stack, nạp/ghép golden snapshots theo domain tại contracts/http-snapshots/*.json từ BFF Go httptest và Identity Spring MockMvc) làm mặc định cho kiểm thử trình duyệt, và Mode 1 (Full-Stack Live E2E qua Kong :8000). Dùng khi lập kế hoạch/phân tầng kiểm chứng E2E trước release, viết Playwright spec/Page Object Model (POM), tạo/cập nhật/ghép REST HTTP snapshot (Go/Java), kiểm thử browser upload MinIO presigned PUT (ADR-012), hoặc xuất báo cáo visual evidence desktop (1440x900) — kể cả khi chỉ nói "test luồng trình duyệt" hay "khóa schema REST BE-FE". Không dùng khi chỉ viết/chạy Vitest/Go/Java unit test, @DataJpaTest hoặc concurrency-sim, sửa protobuf/event/Kong route (dùng contract-change), khoanh vùng bug runtime (dùng debugging-services), hay kế hoạch triển khai feature ngoài phạm vi E2E (dùng fullstack-feature).'
---

# E2E Testing (Microservices Monorepo)

Skill này chuẩn hóa kiểm thử End-to-End bằng Playwright và cơ chế khóa hợp đồng REST hai chiều cho hai ứng dụng frontend của repository:

- `ecommerce-fe` (Storefront — `http://localhost:5173`)
- `admin-fe` (Admin Console — `http://localhost:5174`)

Mọi yêu cầu API công khai đều đi qua Kong (`http://localhost:8000/api/v1` ở local hoặc same-origin `/api/v1` trên production Compose). Workspace Playwright tập trung tại `e2e/` (chỉ đọc JSON trong `contracts/http-snapshots/`; không import mã nguồn từ `services/`, `ecommerce-fe/src` hay `admin-fe/src` để giữ sạch `make check-boundaries`).

---

## 1. Phối hợp với các Skill khác của Repository

| Skill | Vai trò phối hợp |
|---|---|
| `fullstack-feature` | Sở hữu Surface Map, danh sách AC, chốt **Mode Lock** trong spec/plan và Gate lớp 4 (`make verify SERVICE=e2e` cho Mode 2, `make e2e-live` cho Mode 1). Skill này thực thi phần kiểm chứng browser & REST snapshot trong gate đó. |
| `contract-change` | Khi backend snapshot test hoặc Playwright phát hiện lỗi do **contract REST sai hoặc thay đổi breaking** (không phải bug implementation), dừng lại và đi qua quy trình `contract-change` (phân loại compatibility → tìm consumer → sửa provider trước → cập nhật `contracts/http-snapshots/*.json` → sửa FE). |
| `debugging-services` | Khi Mode 1/Mode 2 fail hoặc flaky, điều tra theo hop (`FE → Kong → BFF/Identity → gRPC → DB/RabbitMQ`) để tìm root cause có evidence trước khi sửa. |
| `reviewing-code` & `reviewing-service-boundaries` | Review cuối nhánh kiểm tra thêm việc tuân thủ `mountContractRoutes`, zero `localStorage` token, không vi phạm service boundary và không xóa/hạ assertion. |

---

## 2. Phân tầng E2E & Quy tắc Mode Lock

Chọn tầng kiểm chứng thấp nhất đủ chứng minh acceptance criterion (AC) để tránh vừa **over-mock** hành vi phân tán vừa **over-test** logic UI thuần trên full stack:

| Tầng | Phạm vi chứng minh | Công cụ / Mode | Khi nào dùng |
|---|---|---|---|
| **Tầng 1: Unit / Slice / Integration** | Domain logic, SQL migration, gRPC handler, Spring `@WebMvcTest`, Vitest component | `make verify SERVICE=<surface>` | Mọi thay đổi (bắt buộc) |
| **Tầng 2 — Mode 2: Shape-Locked Mock E2E** *(Mặc định cho browser)* | Hành vi trình duyệt thật (routing, reload cookie session, form, toast, trạng thái lỗi hiếm, visual desktop) bám chặt REST shape thật từ `bff-service` & `identity-service` | Playwright + `contracts/http-snapshots/*.json` (`E2E_MODE=mock`, `make verify SERVICE=e2e`) | Feature chạm UI/luồng người dùng mà không cần dựng Docker + 7 services |
| **Tầng 3 — Mode 1: Live Browser E2E qua Kong** *(Opt-in)* | Wiring thật Browser ↔ Kong (CORS, `HttpOnly` cookie, JWT `caller-context`) ↔ Identity/BFF ↔ private gRPC ↔ DB/MinIO/RabbitMQ | Playwright + Kong `:8000` (`E2E_MODE=live`, `make e2e-live`) | Khi user yêu cầu, hoặc release critical path / thay đổi Kong route, cookie, auth wiring |
| **Tầng 4: Distributed API / Saga E2E** | Concurrency storm, oversell race, idempotency storm, MoMo callback race, outbox RabbitMQ & SQL invariants | `scripts/concurrency-sim`, `scripts/verify-foundation`, `psql` / `rabbitmqctl` | Luồng phân tán Order ↔ Inventory ↔ Payment không thể mô phỏng đúng bằng mock trình duyệt |

> **Quy tắc Mode Lock:** Trong spec/plan, lập ma trận gán rõ từng AC vào một mức kiểm chứng (`unit/slice`, `mock-e2e`, `live-e2e`, `visual`, `distributed-sim`) và chốt E2E mode cho feature (mặc định **Mode 2**). Trong quá trình thực thi, **không tự ý hạ cấp hoặc chuyển mode** (ví dụ từ `live-e2e`/`distributed-sim` xuống `mock-e2e`) khi chưa có quyết định của người dùng.

---

## 3. Mode 2: Shape-Locked Mock E2E (Golden Snapshot Lock theo Domain)

Mock `page.route()` tự viết dễ bị trôi khỏi backend khi DTO đổi field. Mode 2 khóa hai chiều bằng **Golden Contract Snapshots tập trung theo domain sở hữu** tại `contracts/http-snapshots/<domain>.json`:

```text
contracts/http-snapshots/
├── identity-auth.json     # identity-service: /auth/**, /users/me, /users/me/permissions, /users/me/addresses, /locations/**
├── identity-admin.json    # identity-service: /admin/users**, /admin/rbac/items**
├── bff-storefront.json    # bff-service: /storefront/products**, /cart**, /favorites**
├── bff-orders.json        # bff-service: /orders** (customer-safe DTO, không rò rỉ refundLastError/refundReason/refundActorType)
└── bff-admin.json         # bff-service: /admin/products**, /admin/inventory**, /admin/orders**
```

**Vì sao tách theo domain thay vì gộp chung 1 file feature?**
- `bff-service` (Go `go test`) và `identity-service` (Java `mvn test`) chạy độc lập; mỗi service chỉ ghi vào file domain của chính nó nên không bao giờ xảy ra xung đột ghi file chéo service và giữ nguyên ranh giới ownership.
- Mỗi endpoint công khai chỉ có một nguồn chân lý duy nhất dùng chung cho cả `ecommerce-fe` và `admin-fe`.
- Ở phía Playwright, `loadContractSnapshots(["../contracts/http-snapshots/identity-auth.json", "../contracts/http-snapshots/bff-orders.json"])` nạp và ghép nhiều file domain vào cùng một `ContractSnapshotDocument` (tự báo lỗi nếu trùng snapshot key).

### Chiều 1 — Backend thật khóa snapshot
- **BFF (`services/bff-service`, Go):** Handler test dùng `httptest.NewRecorder()` chạy qua real `http.ServeMux`, middleware (`RequireCaller`, `correlateTrace`) và serializer thật; dùng helper chuẩn (mẫu tại [`examples/bff_contract_snapshot_test.go`](examples/bff_contract_snapshot_test.go), chỉ dùng Go stdlib).
- **Identity (`services/identity-service`, Java 21 / Spring Boot 4 / Jackson 3):** API test dùng `MockMvc` `MvcResult` và `tools.jackson.databind.*` (Jackson 3 — không dùng `com.fasterxml.jackson.databind.*`); dùng helper chuẩn (mẫu tại [`examples/IdentityContractSnapshotSupport.java`](examples/IdentityContractSnapshotSupport.java)).
- **Chuẩn so sánh mảng:** Cả Go và Java đều chuẩn hóa phần tử mảng về **tập shape duy nhất đã sắp xếp** (unique element shapes) thay vì so sánh cứng độ dài mảng — nhờ vậy đổi số lượng phần tử (`1` role vs `2` roles) không gây false drift, trong khi mảng rỗng `[]` vs có phần tử hoặc sai kiểu/key bên trong vẫn fail ngay.
- **Chế độ cập nhật:** Mặc định khi thiếu cờ `UPDATE_CONTRACT_SNAPSHOTS=1`, mọi sai khác về `service`, `endpoint`, `status`, `requestBody` hoặc `responseBody` đều làm fail `go test` / `mvn test`. Khi chủ động đổi contract, chạy với `UPDATE_CONTRACT_SNAPSHOTS=1`, review `git diff contracts/http-snapshots/`, rồi chạy lại Playwright Mode 2.

### Chiều 2 — Playwright Mode 2 khóa FE bằng helper dùng chung
**Nguyên tắc authority:** Nếu repo đã có sẵn `e2e/fixtures/contract-routes.ts` và backend recorder, dùng bản trong repo làm nguồn chuẩn; chỉ bootstrap từ `examples/` khi chưa có.

**Những gì helper dùng chung (`mountContractRoutes` — [`examples/contract-route-fixture.ts`](examples/contract-route-fixture.ts)) tự động bảo đảm:**
1. **Điểm đăng ký duy nhất cho `/api/v1/**`:** Spec khai báo mọi route API qua `mountContractRoutes`, không gọi `page.route()` / `context.route()` trực tiếp trên `/api/v1/**` để tránh lọt khỏi bộ kiểm tra shape.
2. **Cross-origin CORS + `withCredentials: true`:** Vì `:5173` / `:5174` gọi sang `http://localhost:8000/api/v1`, helper tự xử lý `OPTIONS` preflight và trả `Access-Control-Allow-Origin` + `Access-Control-Allow-Credentials: true`.
3. **Unmocked-request trap:** Mọi `/api/v1/**` chưa khai báo đều đánh fail test ngay thay vì rớt ra mạng thật.
4. **Auth boundary (`auth: "bearer"` | `"refreshCookie"`):** Thiếu `Authorization: Bearer ...` hoặc cookie `refresh_token` sẽ trả đúng snapshot `401` của backend.
5. **Request & Response shape lock:** Đối chiếu payload FE gửi lên với `requestSnapshot` và đối chiếu mọi response trả về (kể cả override động từ in-memory state) 100% key/type với snapshot (chỉ cho lệch `null` ở các key khai báo `nullable`).
6. **Visible stubs:** Background call ngoài phạm vi contract của bài test (ví dụ layout fetch `/cart`, `/favorites`) khai báo rõ bằng `stub` để tách biệt khỏi route khóa contract.

**Những gì mỗi file `.spec.ts` tự chịu trách nhiệm (xem [`examples/mode2-contract-mock.spec.ts`](examples/mode2-contract-mock.spec.ts)):**
- Seed mutable in-memory state từ `responseBody` đã ghi, cập nhật state sau mutation để read/reload tiếp theo phản ánh trạng thái mới.
- Áp dụng đúng nhánh nghiệp vụ/validation/role (trả snapshot `400`/`403`/`404` tương ứng).
- **MinIO Presigned `PUT` (`ADR-012`, upload media ở `admin-fe`):** Dùng `mountPresignedUploadTrap` trên `**/product-media/**` để khẳng định có `Content-Type` hợp lệ, **không** gắn header `Authorization` (vi phạm đánh fail test ngay), và phục vụ lại binary qua `GET`/`HEAD` trong bộ nhớ để `<img>` decode thật (`complete && naturalWidth > 0`) không cần Docker/MinIO.

### Quy tắc Auth & Security (Áp dụng cho cả Mode 2 và Mode 1)
Cả `ecommerce-fe` và `admin-fe` giữ `accessToken` **thuần trong memory** và khôi phục phiên sau `page.reload()` bằng `HttpOnly` cookie `refresh_token` (`Path=/api/v1/auth`) qua `POST /api/v1/auth/refresh`.
- **Không bao giờ** dùng `localStorage.setItem("accessToken", ...)` hay `sessionStorage` để giả lập đăng nhập — làm vậy che mất lỗi thật ở vòng đời refresh cookie và vi phạm mô hình bảo mật của repo.
- Trong Mode 2, cấp cookie `refresh_token` khi login (`Set-Cookie` + `context.addCookies`), kiểm tra cookie trên `/api/v1/auth/refresh`, và assert `localStorage`/`sessionStorage` không chứa token key.

---

## 4. Mode 1: Full-Stack Live E2E qua Kong

Dùng khi feature yêu cầu `live-e2e` hoặc smoke wiring thật trước release.

### Điều kiện tiên quyết (Local Port Topology theo root `AGENTS.md`)
- Foundation chạy (`make infra-up`): Kong `:8000` (admin `:8001`), PostgreSQL `:5432`, Redis `:6379`, RabbitMQ `:5672`/`:15672`, MinIO `:9000`.
- Khởi động ứng dụng theo thứ tự **provider-first**:
  1. `identity-service` HTTP `:8080` (mgmt `:8090`), gRPC `:50054`
  2. `catalog-service` HTTP `:8085` (`:50051`), `inventory-service` `:8082` (`:50052`), `cart-service` `:8084` (`:50053`), `order-service` `:8083` (mgmt `:8093`, `:50055`), `payment-service` `:8086` (`:50056`)
  3. `bff-service` HTTP `:8081` (`BFF_HTTP_ADDR=0.0.0.0:8081`)
  4. Frontend: `ecommerce-fe` `:5173` / `admin-fe` `:5174` (do `webServer` của Playwright quản lý).
- Mọi request từ browser và fixture phải đi qua Kong (`http://localhost:8000/api/v1`), không gọi trực tiếp HTTP port nội bộ (`:8080`–`:8086`).

### Health Guard & Auth Bootstrap ([`examples/kong-live-auth-fixture.ts`](examples/kong-live-auth-fixture.ts))
- **Health Guard:** Trước khi chạy test live, kiểm tra `GET /api/v1/auth/.well-known/jwks.json` và `GET /api/v1/storefront/products?page=1&pageSize=1` với timeout ngắn (`3_000ms`). Nếu stack chưa bật, fail (khi `E2E_STRICT_LIVE=1` / `CI=true`) hoặc skip kèm thông báo chẩn đoán rõ — không để treo timeout.
- **Tài khoản test mặc định (tuân thủ root `AGENTS.md`):** Luôn tái sử dụng tài khoản seeded có sẵn:
  - Customer: `customer1@gmail.com` / `123456123456`
  - Admin: `admin@example.com` / `AdminPassword123!`
  - Không tự động đăng ký tài khoản tạm vào `identity_db` (`autoRegisterCustomer: false` theo mặc định), trừ khi bài test nhắm đích danh luồng registration hoặc cô lập đa người dùng và nêu rõ lý do.
- **Cookie-jar Auth Bootstrap:** Gọi `page.request.post("http://localhost:8000/api/v1/auth/login", ...)` (dùng chung cookie jar với `BrowserContext`) để nhận real `HttpOnly` `refresh_token` cookie, verify `{ data, message }` envelope và `httpOnly === true`, rồi mở FE để `AuthProvider` tự gọi `POST /api/v1/auth/refresh` nạp `accessToken` vào memory.

---

## 5. REST Contract Envelope của Repository

Bám đúng `docs/steering/api-contracts.md` và root `AGENTS.md`:
- **Success:** `{ "data": <payload>, "message": "..." }` (`"data": null` khi không có payload).
- **Pagination:** `{ "data": { "items": [], "page": 1, "pageSize": 20, "totalItems": 0, "totalPages": 0 }, "message": "..." }`.
- **Error:** `{ "code": "STABLE_ERROR_CODE", "message": "...", "errors"?: { "<field>": "..." }, "traceId": "..." }` (bỏ khóa `errors` khi không có lỗi theo field).
- **Ngoại lệ hiện hành (không sửa để tránh breaking):** `GET /api/v1/users/me` trả flat `MeResponse` object; `/api/v1/locations/**` trả raw array. Golden snapshot ghi nhận chính xác shape hiện hành từ test backend.

---

## 6. Page Object Model (POM), Selectors & Chống Flaky

1. **Selector bền vững:** Ưu tiên `[data-testid="..."]` → ARIA role & accessible label (`getByRole`, `getByLabel`, `getByAltText`). Tránh chọn theo Tailwind utility class (`.bg-emerald-700`, `.rounded-2xl`) hay chuỗi DOM sâu (`div > div > span`).
2. **Đóng gói trong `pages/*Page.ts`:** Spec chỉ gọi hành vi nghiệp vụ (`loginPage.submit(...)`, `ordersPage.openOrder(...)`).
3. **Chống flaky:** Không dùng `page.waitForTimeout(...)`; dùng auto-wait của Playwright (`expect(locator).toBeVisible()`, `page.waitForResponse(...)`, `expect.poll(...)`). Xác minh độ ổn định bằng `npx playwright test <spec> --repeat-each=3`.
4. **Vòng đời Vite server:** Quản lý `:5173` / `:5174` bằng `webServer` trong `playwright.config.ts`, không để dev server chạy trôi nổi.

---

## 7. Visual Evidence, Report & Xử lý Lỗi

1. **Desktop-only (`1440x900`):** Chỉ chạy Playwright và chụp ảnh ở viewport desktop `1440x900` trừ khi người dùng yêu cầu rõ viewport mobile. Lưu ảnh đánh số vào `reports/<feature>/imgs/` (`01-<step>-desktop.png`, `02-...png`).
2. **Báo cáo (`reports/<feature>/report.md`):** Ghi đủ:
   - Execution Mode (`Mode 2: Shape-Locked Mock E2E` hoặc `Mode 1: Live E2E via Kong`),
   - Commands đã chạy (kèm `--repeat-each=3` nếu có) & exit code thực tế,
   - Bảng **Contract Shape Verification Matrix** (Endpoint, Owning Service: `bff-service` | `identity-service`, Snapshot File & Key, Auth Boundary, Request/Response Assertions),
   - Link ảnh tương đối (`./imgs/*.png`) và ghi rõ giới hạn phạm vi chứng minh của mode đã chạy.
3. **Phân loại khi test fail:**
   - Lỗi implementation / wiring trong spec → debug theo `debugging-services`, sửa tại owning surface, chạy lại `make verify SERVICE=<surface>` và E2E.
   - Lỗi do contract sai/breaking → dừng ngay và chuyển sang `contract-change`.
