# Quy ước viết contract (OpenAPI 3.1) — hướng C

`openapi.yaml` là source of truth của API cho pipeline `fullstack-feature`. Chỉ architect agent được sửa.

## Vai trò của file này
`openapi.yaml` là **tài liệu thuần**: KHÔNG sinh code. Không có `gen.go`, không có `schema.d.ts`.
- Backend đọc contract → tự viết Gin handler + DTO struct khớp shape (theo `.docs/steering/api-conventions.md`).
- Frontend đọc contract → tự viết type + gọi qua lớp services của repo (`services/*Api.ts`).
- Enforcement conformance nằm ở **QA integration test** (black-box HTTP trong `tests/integration/`), KHÔNG ở compiler. Vì vậy `examples` phải đầy đủ và đúng ngữ nghĩa — chúng là baseline test vector; contract mơ hồ = test yếu = drift lọt lưới.
- Chỉ `redocly lint` chạy tự động (gate `contract`) để bắt lỗi cú pháp OpenAPI.

## Bắt buộc với mọi operation
1. `operationId`: camelCase, duy nhất (ví dụ `createSalesRequest`). KHÔNG còn sinh code từ nó — backend/frontend tự đặt tên hàm/handler theo nó; QA dùng đặt tên test; spec.md map AC → operationId. Vì không có compiler ràng buộc, operationId + path + shape phải được mô tả ĐỦ RÕ để dev implement đúng mà không đoán.
2. `examples` cho request body và response chính — đây là **baseline semantic** cho test, KHÔNG phải toàn bộ test vector. QA dùng làm điểm xuất phát cho happy path; với dữ liệu động (ID sinh từ sequence, token) QA tự sinh/tự lấy. Ngoài happy path, QA bổ sung boundary/field thiếu-sai/stateful (trùng lặp)/authorization và validate response theo schema.
3. Response 2xx nghiệp vụ mô tả theo envelope chuẩn `ResponseEnvelope` (`{code, message, data}`); mô tả rõ shape của `data`. Mọi response 4xx/5xx dùng schema `#/components/schemas/Error` (`{code, message, error?}`) kèm business error code cụ thể — khớp `pkg/response`.
4. `tags` theo resource/module; thêm extension `x-feature: <NNN-slug>` vào operation để trace về feature spec.
5. Endpoint cần auth: khai `security` + ghi RBAC permission code trong `description`. Token test theo role có sẵn ở `.docs/contracts/accesstokenfortestonly.md` — QA dùng khi viết integration test.
6. Endpoint dạng list: dùng đúng pagination chuẩn của repo — `page`, `pageSize`, `sortField`, `sortOrder` (xem `.docs/steering/api-conventions.md`).

## Schema
- Tên PascalCase (`SalesRequest`, `CreateSalesRequestRequest`).
- Khai báo `required` tường minh — dev đọc để biết field nào bắt buộc. Đây là hợp đồng đọc-bằng-mắt, QA integration test kiểm chứng.

## Quan hệ với `.docs/contracts/` (flow fico-fullstack có sẵn)
Repo có sẵn convention contract `.docs/contracts/<feature>.md` + `.http` (viết SAU backend, từ DTO đã implement) dùng cho skill `fico-fullstack`. Hai flow tồn tại song song:
- Pipeline `fullstack-feature` (contract-FIRST): dùng `contracts/openapi.yaml` — architect viết TRƯỚC khi có code.
- Skill `fico-fullstack` (backend-first): dùng `.docs/contracts/` như cũ.
Đừng trộn: một feature đi theo một flow, contract của flow nào nằm ở chỗ flow đó.
