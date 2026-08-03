# Roadmap Pet Project E-commerce Microservices (8–12 tuần)

> Mục tiêu của tài liệu này là xây một portfolio có chiều sâu kỹ thuật: hoàn thành được một luồng mua hàng đáng tin cậy, giải thích được các trade-off, và có thể demo lỗi/phục hồi. Không phải cố đưa thật nhiều công nghệ vào cùng lúc.

## 1. Kết luận nhanh

Kế hoạch ban đầu có nền tảng tốt: lựa chọn Java + Go, gRPC, RabbitMQ, Redis, Kong và observability đều phù hợp để học kiến trúc phân tán. Tuy nhiên, nếu triển khai tất cả service và nâng cấp cùng lúc thì 8–12 tuần sẽ dễ thành “demo nhiều công nghệ” nhưng thiếu tính đúng đắn.

**Quyết định đề xuất:** bắt đầu bằng **5 service tối thiểu có chủ đích**, hoàn thiện login/register và một checkout flow end-to-end, sau đó mới thêm K8s và Elasticsearch.

| Thành phần | Công nghệ | Trách nhiệm chính |
|---|---|---|
| BFF | Go | REST/JSON cho client, tổng hợp dữ liệu, không chứa nghiệp vụ đơn hàng cốt lõi |
| Identity Service | Java + Spring Boot/Spring Security | Đăng ký, đăng nhập, profile, role, JWT và refresh-token session |
| Catalog Service | Go | Sản phẩm, giá niêm yết, Redis cache |
| Order Service | Java + Spring Boot | Đơn hàng, idempotency, Saga state, transactional outbox |
| Inventory Service | Go | Reservation tồn kho, chống oversell, xử lý event |
| Edge & infra | Kong, PostgreSQL, Redis, RabbitMQ | Ingress, data, cache/utility, message broker |

Không xây ngay: payment thật, review, notification hoàn chỉnh, recommendation, nhiều loại client, service discovery riêng, hay 10+ microservice.

## 2. Mục tiêu học tập và tiêu chí hoàn thành

Pet project này nên chứng minh năng lực ở cấp **middle hướng senior** qua cách ra quyết định và vận hành hệ thống, không chỉ qua danh sách framework.

Khi hoàn thành MVP, cần demo được các tình huống sau:

1. Client gọi một REST endpoint qua Kong và BFF để xem sản phẩm hoặc đặt hàng.
2. User đăng ký, đăng nhập, refresh/logout và đọc/sửa profile qua Identity Service; credential/session không bị leak.
3. BFF gọi gRPC nội bộ; request có deadline, correlation/trace context và không đi vòng qua BFF giữa các service.
4. Đặt hàng lặp lại cùng `Idempotency-Key` chỉ tạo một order.
5. Hai request cạnh tranh số hàng cuối cùng chỉ có một reservation thành công; tồn kho không âm.
6. RabbitMQ có retry hữu hạn, DLQ và consumer idempotent; event không mất khi service chết ngay sau DB commit.
7. Có thể nhìn một request từ BFF → Order → RabbitMQ → Inventory trên trace; metrics và log cho biết service nào lỗi/chậm.
8. Tắt Catalog hoặc Inventory có kiểm soát: timeout/circuit breaker/bulkhead không làm cạn tài nguyên của service gọi.
9. Lỗi nghiệp vụ và dependency trả về `code`, HTTP/gRPC status và `trace_id` nhất quán; client không nhận stack trace.

## 3. Kiến trúc đích của MVP

```text
Browser / API client
        │ HTTPS + Bearer JWT + Idempotency-Key
        ▼
Kong Gateway ─────────────── public ingress: route, rate limit, JWT validation for protected routes
   │                         │
   │ /api/v1/auth/*, /api/v1/users/me      └─ /api/v1/storefront/*
   ▼                              ▼
Identity (Java) ──► identity_db   BFF (Go, REST) ── gRPC ─► Catalog (Go) ─► catalog_db + Redis
   │ JWT + JWKS                      │
   └─────────────────────────────────└───────────── gRPC ─► Order (Java) ─► order_db + outbox
                                                                   │
                                                                   ▼
                                                             RabbitMQ exchanges
                                                                   │
                                                                   ▼
                                                    Inventory (Go) ─► inventory_db + Redis
```

### Nguyên tắc biên service

- Mỗi service sở hữu database/schema của mình. Trong local có thể dùng cùng một PostgreSQL instance, nhưng **không query hay foreign key xuyên service**.
- BFF là edge adapter cho client; BFF không là “bus” để microservice gọi nhau.
- Kong chỉ expose BFF và Identity Service qua namespace chung `/api/v1/`: `/api/v1/auth/*` là ngoại lệ public; `/api/v1/users/me` cần JWT. Catalog, Order và Inventory chỉ nghe trên network nội bộ.
- gRPC dùng cho truy vấn/command đồng bộ có phản hồi cần ngay; RabbitMQ dùng cho event sau khi state đã được commit.
- RabbitMQ giao hàng **at-least-once**, do đó mọi consumer phải chịu được bản tin trùng.
- `identity_user_id` là UUID từ Identity Service; Order lưu nó như external identifier, không tạo foreign key xuyên database.

## 4. Luồng nghiệp vụ trọng tâm: đặt hàng và reserve tồn kho

Đây là phần đáng đầu tư nhất của dự án.

1. Client gửi `POST /orders` qua Kong/BFF, kèm JWT và `Idempotency-Key`.
2. BFF chuyển identity đã được xác thực và command sang Order Service. Order không tin giá/total từ client; nó lấy và snapshot thông tin giá sản phẩm từ Catalog qua gRPC.
3. Trong **một transaction**, Order lưu `PENDING_STOCK` order, dòng order item, bản ghi idempotency, và bản ghi outbox `OrderCreated`.
4. Outbox publisher phát `OrderCreated` sau commit, có publisher confirm. Nếu phát thất bại, job retry sẽ phát lại.
5. Inventory consume event, deduplicate theo `event_id`, rồi dùng transaction/SQL conditional update để tạo reservation.
6. Inventory phát một trong hai event: `InventoryReserved` hoặc `InventoryReservationRejected`.
7. Order consume event và chuyển order thành `CONFIRMED` hoặc `CANCELLED`. Event trùng không được đổi state lần hai. Nếu `InventoryReserved` đến sau khi Order đã `CANCELLED`, Order phải phát compensating event `ReservationReleaseRequested`; Inventory release idempotent và phát `InventoryReleased`.

`PENDING_STOCK → CONFIRMED/CANCELLED` là Saga choreography nhỏ, đủ để học eventual consistency. Chưa cần payment thật; nếu muốn mở rộng sau này, thêm `STOCK_RESERVED → AWAITING_PAYMENT → PAID` thay vì đổi nghĩa của `CONFIRMED`. Event phải mang `event_id`, `event_type`, `schema_version`, `aggregate_id`, `occurred_at`, `correlation_id`, `causation_id` và W3C trace context.

### 4.1 Luồng Identity: đăng ký, đăng nhập và profile

Identity Service là **first-party authentication service**, không phải OAuth/OIDC provider tổng quát. Nó tự quản lý credential của e-commerce và phát JWT cho chính client của hệ thống.

1. Client gọi `POST /api/v1/auth/register` qua Kong đến Identity Service. Service validate input, hash password, tạo user/profile/role mặc định `CUSTOMER` và không log password.
2. Client gọi `POST /api/v1/auth/login`; nếu credential hợp lệ, service trả access token ngắn hạn và refresh token. Login fail trả lỗi chung để không tiết lộ email có tồn tại hay không.
3. Các request protected mang access JWT. Kong chặn token sai ở ingress; BFF/Identity xác minh signature/claim qua public key của Identity Service trước khi tạo caller context.
4. `POST /api/v1/auth/refresh` xoay refresh token: token cũ bị revoke, token mới được tạo cùng token family. Reuse refresh token đã revoke làm revoke toàn bộ session family.
5. `POST /api/v1/auth/logout` revoke session/refresh token. `GET/PATCH /api/v1/users/me` đi thẳng Identity Service qua Kong; user chỉ được đọc/sửa profile của mình.

Access token chứa tối thiểu `sub` (UUID user), `roles`, `iss`, `aud`, `exp`, `iat`, `jti` và `kid`. Identity Service ký bằng key bất đối xứng, public key được publish dưới JWKS endpoint để Kong/BFF cache và verify. Password reset, email verification, MFA, social login và third-party OAuth/OIDC được hoãn sau MVP.

## 5. Nhận xét và chỉnh sửa các điểm quan trọng trong bản gốc

| Nội dung gốc | Nhận xét / quyết định hoàn thiện |
|---|---|
| Product Go, Order Java, Inventory Go | Hợp lý. Đừng chọn ngôn ngữ chỉ vì “Go nhanh”; chọn vì bạn muốn học concurrency/gRPC trong Go và domain workflow trong Spring Boot. |
| Redis Lock để chống oversell | **Không xem Redis lock là nguồn đảm bảo đúng đắn.** Đảm bảo chính phải nằm ở inventory DB: conditional update `available >= quantity`, transaction và unique reservation theo order. Redis lock chỉ là bài học/biện pháp bổ sung cho hot SKU, có TTL, fencing/timeout và không thay DB invariant. |
| Order tạo `PENDING_STOCK` rồi Inventory reserve kho | Đúng hướng nhưng thiếu nhánh thất bại và compensation muộn. Bắt buộc thêm `InventoryReservationRejected → Order CANCELLED`, retry/DLQ, idempotent consumer và `ReservationReleaseRequested` khi reservation đến sau cancel. |
| Publish RabbitMQ sau khi lưu DB | Cần **transactional outbox**. Nếu không, process có thể chết giữa DB commit và publish khiến order kẹt mãi. |
| JWT chỉ check ở Kong, service “ngây thơ” | Identity Service là issuer JWT. Kong nên chặn request ở ingress, nhưng downstream không được tin header tùy ý từ public network. Giữ service private, strip/overwrite header tại gateway, BFF verify claim trước khi tạo caller context, và kiểm tra authorization/ownership ở service xử lý dữ liệu nhạy cảm. |
| Kong route thẳng service hoặc BFF | Về kỹ thuật làm được; với dự án này mặc định dùng chung namespace `/api/v1/*` nhưng có route thẳng. Ngoại lệ hẹp đi thẳng: Identity endpoint (`/api/v1/auth/*`, `/api/v1/users/me`) và payment webhook sau này. Không công khai generic internal service endpoint chỉ để “cho đủ microservice”. |
| gRPC call mặc định timeout 30 giây | Không nên giả định vậy. Java gRPC thường không có deadline mặc định nếu app không đặt. Mọi outbound call phải đặt deadline rõ ràng; timeout ngắn hơn request budget. |
| Circuit breaker trả giá `0` khi Catalog down | Không an toàn cho checkout. Với read-only product page có thể trả cached/stale data; với tạo order phải fail rõ `PRODUCT_UNAVAILABLE`, không dùng giá fallback. |
| OpenTelemetry “bắn logs/metrics/traces” thẳng mọi nơi | Tách pipeline: traces qua OTLP Collector → Jaeger; Prometheus scrape metrics endpoint; logs JSON có `trace_id` (Loki là tùy chọn). Message headers phải propagate W3C Trace Context qua RabbitMQ. |
| K8s + Service Discovery | K8s Service + CoreDNS đã là service discovery. Không cần thêm Eureka/Consul trong phạm vi này. Docker Compose cũng resolve bằng service name trên network chung. |
| 3 Docker Compose file chạy độc lập | Ý tưởng hybrid đúng, nhưng các file độc lập cần cùng network/project. Cách đơn giản là chạy `docker compose -f infra.yml -f services.yml up`, hoặc tạo external shared network. Local process dùng `localhost`; container dùng DNS service name. |
| Index | Nhận xét ban đầu đúng: PostgreSQL tự tạo index cho `PRIMARY KEY` và `UNIQUE`, không tự index FK. Chỉ thêm index theo query thực tế, kiểm chứng bằng `EXPLAIN (ANALYZE, BUFFERS)`. |
| Proto chung monorepo | Đúng cho pet project. Dùng một `proto/` canonical, không copy. Ưu tiên Buf để lint, breaking-change check và generate; `protoc` script vẫn là fallback hợp lệ. |

## 6. Scope theo giai đoạn (8–12 tuần)

Mỗi giai đoạn phải tạo ra một demo chạy được; không mở phase kế tiếp nếu exit criteria chưa đạt.

### Phase 0 — Foundation (tuần 1)

**Mục tiêu:** repo có thể chạy một lệnh, contract ổn định và mọi service có skeleton.

- Monorepo: `proto/`, `services/`, `deployments/`, `docs/`, `scripts/`.
- Docker Compose cho PostgreSQL, Redis, RabbitMQ, Kong và Identity Service; cấu hình `.env.example` không chứa secret thật.
- Proto v1 cho Catalog, Order, Inventory; Buf lint/generate hoặc script `protoc` duy nhất.
- Chốt error model v1: gRPC status chuẩn, stable domain error code và REST error envelope tại BFF.
- Identity skeleton: `identity_db`, local asymmetric signing key chỉ cho dev, JWKS endpoint, health check và migration tách riêng.
- Health/readiness endpoint, structured JSON logs và config theo environment cho mỗi service.
- README: sơ đồ hệ thống, cách chạy, port map và happy-path API.

**Exit criteria:** `make up` (hoặc lệnh tương đương) khởi động infra; mỗi service expose health check; generated client/server build được ở Go lẫn Java.

### Phase 1 — Happy-path core (tuần 2–3)

**Mục tiêu:** có dữ liệu thật và một request đi qua BFF/gRPC.

- Catalog CRUD tối thiểu và `GetProduct`/`ListProducts` gRPC.
- Inventory seed SKU và query availability; chưa cần event flow phức tạp ở ngày đầu.
- Order tạo order với item snapshot và state machine tối thiểu.
- Identity Service có register/login/refresh/logout và `GET/PATCH /users/me`; hash password, role `CUSTOMER`, JWT access token và refresh-token rotation hoạt động.
- BFF REST: product list/detail và create order; aggregation chạy song song khi các query độc lập.
- Kong route vào BFF và Identity; chỉ register/login/refresh là anonymous, protected routes bắt buộc JWT; rate limiting cơ bản cho cả auth endpoint.

**Exit criteria:** user đăng ký, đăng nhập và gọi BFF qua Kong tạo được order `PENDING_STOCK`; không truy cập trực tiếp database của service khác.

### Phase 2 — Consistency và asynchronous workflow (tuần 4–6)

**Mục tiêu:** biến happy path thành workflow chịu retry/failure.

- Transactional outbox tại Order và Inventory; publisher có retry/publisher confirm.
- RabbitMQ topology rõ ràng: domain exchange, queue theo consumer, retry có giới hạn, DLQ.
- Inventory reservation với DB invariant, consumer dedup/inbox, event outcome.
- Saga state transitions trong Order; test các event trùng và đến sai thứ tự.
- Idempotency bền vững bằng unique key trong Order DB; Redis có thể hỗ trợ fast-path nhưng không là nguồn duy nhất.
- Reconciliation job quét `PENDING_STOCK` quá hạn; cancel có kiểm soát và phát compensation nếu Inventory đã reserve.

**Test scenarios bắt buộc cho Saga:**

| Tình huống | Kỳ vọng kiểm chứng |
|---|---|
| Happy path: `OrderCreated → InventoryReserved` | Order đi từ `PENDING_STOCK` sang `CONFIRMED`; đúng một reservation và đúng một outcome event. |
| Hết hàng: `OrderCreated → InventoryReservationRejected` | Order thành `CANCELLED`; stock/reservation không âm và không có side effect thành công. |
| `OrderCreated` trùng | Inventory tạo tối đa một reservation cho một `order_id`; event duplicate được acknowledge/dedup. |
| `InventoryReserved` trùng | State machine không đổi lần hai; không tạo reservation/side effect bổ sung. |
| `InventoryReserved` tới sau `CANCELLED` | Order không mở lại state; tạo đúng một `ReservationReleaseRequested`, Inventory release đúng một lần. |
| Outcome event tham chiếu order chưa thấy được | Không tự tạo order. Persist event chờ hoặc retry có backoff hữu hạn; quá ngưỡng vào DLQ và alert để điều tra. Không giả định thứ tự toàn cục giữa queue/consumer. |
| Consumer crash sau DB commit nhưng trước broker ack | Restart/re-delivery không double-reserve vì inbox/dedup và unique constraint. |
| Publisher fail sau DB commit | Outbox relay phát lại sau recovery; order cuối cùng vẫn nhận được workflow event. |

**Exit criteria:** fire 20–100 request cạnh tranh vào SKU tồn kho nhỏ; số reservation không vượt tồn kho, order cuối cùng chỉ ở `CONFIRMED` hoặc `CANCELLED`, và toàn bộ bảng test Saga ở trên chạy xanh.

### Phase 3 — Performance, security, resilience (tuần 6–8)

**Mục tiêu:** chứng minh hệ thống có hành vi có chủ đích khi tải hoặc dependency lỗi.

- Cache-aside cho Catalog, TTL hợp lý và invalidation khi product thay đổi. Cache price/product detail, không dùng cache làm nguồn tồn kho.
- JWT flow từ Identity Service cho ingress; Kong/BFF verify qua JWKS, key rotation có `kid`, rate limit và request-size limit.
- gRPC deadline, bounded retry chỉ cho thao tác idempotent, exponential backoff + jitter, circuit breaker và bulkhead/concurrency limit phía caller.
- gRPC health checking; authorization/ownership checks ở Order.
- Kịch bản chaos nhẹ: dừng Catalog/Inventory/RabbitMQ, quan sát timeout, retry và recovery.

**Exit criteria:** không có unbounded retry; create-order không rơi vào fallback sai giá; resource của BFF/Order không cạn khi dependency down.

### Phase 4 — Observability, tests và portfolio polish (tuần 8–9)

**Mục tiêu:** có bằng chứng để giải thích hệ thống hoạt động ra sao.

- OpenTelemetry SDK/auto-instrumentation cho HTTP, gRPC và RabbitMQ; OTel Collector làm điểm nhận chung.
- Jaeger cho trace, Prometheus scrape metrics, Grafana dashboard; logs JSON chứa trace/span ID. Loki là nâng cấp tùy chọn.
- Test pyramid: unit test state/invariant, integration test với Testcontainers hoặc Compose, contract test proto, smoke/E2E test qua Kong.
- Dashboard tối thiểu: request rate/error/latency, gRPC client errors, RabbitMQ backlog/DLQ, cache hit rate, reservation success rate.
- Runbook ngắn: cách trace một order, xử lý DLQ, rollback local, known limitations.

**Exit criteria:** từ `order_id` hoặc `trace_id` xem được luồng end-to-end; CI chạy test/lint/build image; README có demo script.

### Phase 5 — Vận hành nâng cao (tuần 10–12, chọn theo thời gian)

Ưu tiên theo thứ tự sau; không bắt buộc hoàn thành cả hai.

1. **Kubernetes trước:** Kind/Minikube/K3d, Deployment/Service/ConfigMap/Secret, readiness/liveness probes, resource requests/limits, HPA demo. K8s DNS thay Docker DNS.
2. **Elasticsearch sau:** Product Service phát `ProductChanged`; Search Indexer consume và upsert Elasticsearch. BFF gọi Search API khi tìm kiếm/filter. Chấp nhận index eventually consistent; không dual-write DB + ES trong cùng request.
3. **CI/CD nếu còn thời gian:** GitHub Actions lint/test/build/scan/push image. Không cần auto-deploy production thật.

## 7. Quyết định kỹ thuật cần giữ nhất quán khi code

### Data và index

- Mỗi service có migration riêng (Flyway/Liquibase cho Java; Goose/Atlas/Migrate cho Go — chọn một tool phù hợp mỗi service).
- `orders`: unique `(identity_user_id, idempotency_key)`; index các query vận hành thực sự dùng, ví dụ `(identity_user_id, created_at DESC)` và status khi dashboard/filter cần.
- `inventory_reservations`: unique `order_id`/`reservation_id` để retry không double-reserve.
- Identity Service có unique email (normalized), user/profile tách credential nếu cần, và bảng session/refresh-token chỉ lưu token hash, token family, expiry và revoked state.
- Đo query bằng `EXPLAIN ANALYZE` trước khi thêm index; index tăng chi phí write và disk.

### Proto và compatibility

- Tổ chức `proto/catalog/v1`, `proto/order/v1`, `proto/inventory/v1`; producer sở hữu contract của domain mình.
- Thay đổi ưu tiên additive: thêm field mới, không đổi/reuse field number; dùng `reserved` cho field đã xóa.
- BFF chỉ generate/import proto mà nó gọi. Không dùng shared `common.proto` làm “god object”.
- Buf là tùy chọn được khuyến nghị; generated code không phải source of truth. CI phải có bước generate + kiểm tra clean diff hoặc build từ generated artifact rõ ràng.

### Error model và API contract

- Service gRPC trả **standard gRPC status code** trước, sau đó mới dùng domain `reason` để client/BFF phân loại. Không dùng `INTERNAL` cho lỗi mà caller có thể xử lý.
- Dùng `google.rpc.ErrorInfo` (hoặc envelope tương đương đã versioned) cho `reason`, `domain` và metadata không nhạy cảm. Ví dụ: `INVENTORY_INSUFFICIENT`, `PRODUCT_UNAVAILABLE`, `IDEMPOTENCY_KEY_REUSED`.
- BFF là nơi tạo REST contract ổn định. Không expose trực tiếp message/exception của Java, Go hoặc downstream service cho client.

| gRPC status | REST tại BFF | Khi dùng |
|---|---:|---|
| `INVALID_ARGUMENT` | 400 | Request/field không hợp lệ. |
| `UNAUTHENTICATED` | 401 | Thiếu hoặc token không hợp lệ. |
| `PERMISSION_DENIED` | 403 | User không có quyền/không sở hữu resource. |
| `NOT_FOUND` | 404 | Product/order không tồn tại. |
| `ALREADY_EXISTS` | 409 | Xung đột thực sự, ví dụ email đã đăng ký hoặc một idempotency key bị tái sử dụng với payload khác. |
| `FAILED_PRECONDITION` | 422 | Domain state không cho phép thao tác, ví dụ `INVENTORY_INSUFFICIENT`. |
| `RESOURCE_EXHAUSTED` | 429 | Quota/rate limit bị vượt. |
| `UNAVAILABLE` | 503 | Dependency tạm không phục vụ. |
| `DEADLINE_EXCEEDED` | 504 | Hết request budget khi chờ downstream. |
| `INTERNAL`/không phân loại được | 500 | Lỗi không mong đợi; log chi tiết chỉ ở server. |

Response REST lỗi thống nhất:

```json
{
  "code": "INVENTORY_INSUFFICIENT",
  "message": "Số lượng còn lại không đủ để đặt hàng.",
  "trace_id": "01J..."
}
```

- `message` phải an toàn để hiển thị; không trả stack trace, SQL, hostname, token hoặc broker detail. Chi tiết kỹ thuật chỉ nằm trong structured log/trace theo `trace_id`.
- Cùng `Idempotency-Key` và **cùng request fingerprint** phải trả lại status/body đã lưu của request đầu tiên (idempotent replay), không trả `409`. Cùng key nhưng payload khác mới là `ALREADY_EXISTS → 409` với `IDEMPOTENCY_KEY_REUSED`.
- Lưu request fingerprint, response/status và thời hạn replay rõ ràng (ví dụ 24 giờ). Event failure cũng mang `event_id`, `order_id`, stable `reason` và version; không serialize stack trace vào RabbitMQ payload.

### Redis

- Catalog: cache-aside, key versioned, TTL và invalidate khi ghi thành công.
- Idempotency: DB unique constraint là canonical; Redis chỉ tăng tốc hoặc chống duplicate sát nhau.
- Lock: chỉ thử sau khi test DB reservation đúng; lock có owner token, TTL và telemetry. Không dựa vào Redis lock để sửa một invariant thiếu trong database.

### Resilience

- Deadline trước retry; retry trước circuit breaker chỉ khi operation an toàn; luôn có retry budget.
- Circuit breaker đặt ở **client** gRPC/HTTP. Fallback phụ thuộc use case: read có thể cache/stale; write/payment/price phải fail an toàn.
- Blocking stub Java chiếm application thread trong lúc chờ. Async API giảm kiểu block đó nhưng không thay thế deadline, queue bound hay backpressure.
- Ghi rõ timeout budget giữa Kong → BFF → downstream để downstream timeout trước upstream.

### Security

- Không commit secret; local dùng `.env`, K8s dùng Secret (demo secret vẫn không phải production secret).
- Identity Service là first-party auth service, không triển khai authorization code flow, third-party client, MFA hay social login trong MVP.
- Password hash ưu tiên Argon2id; chỉ chấp nhận bcrypt nếu thư viện/vận hành Java đơn giản hơn trong phase đầu. Không log password, access token, refresh token hay JWT signing key.
- Access JWT có hạn ngắn; refresh token lưu hash, được rotate/revoke theo token family. Refresh-token reuse sau revoke phải revoke toàn family và emit security log/metric.
- Signing key bất đối xứng có `kid`; private key chỉ trong secret store, JWKS public có cache/rotation. Không hard-code key hay để key development chạy ở production.
- Kong xác thực JWT tại ingress theo plugin/chế độ hỗ trợ JWKS của bản cài đặt; BFF và Identity Service cũng verify signature/issuer/audience trước khi tạo caller context. BFF/service chỉ nhận traffic private; gateway phải xóa/ghi đè identity headers từ client.
- Phân biệt authentication (ai gọi) và authorization (được làm gì). Identity chỉ cho sửa profile của `sub`; Order vẫn kiểm ownership của order.
- Login/register/refresh cần rate limit riêng và login failure trả response chung để tránh account enumeration.
- Làm mTLS/network policy là stretch goal sau khi K8s chạy ổn.

### Architecture Decision Records (ADR)

- Lưu quyết định có trade-off dài hạn ở `docs/adr/`, không giấu trong commit message hoặc README.
- Bắt đầu với `TEMPLATE.md` và bảy ADR: service boundaries, transactional outbox, DB invariant vs Redis lock, circuit breaker/fallback, Saga compensation/reservation lifecycle, error model/idempotency, first-party identity/token security.
- Mỗi ADR phải nêu context, quyết định, consequence tích cực/tiêu cực và trạng thái. Khi đổi quyết định, tạo ADR mới và supersede ADR cũ, không sửa lịch sử để làm mất lý do ban đầu.

## 8. Workflow phát triển địa phương

Hybrid Docker + local là lựa chọn tốt:

- Chạy PostgreSQL, Redis, RabbitMQ, Kong bằng Docker.
- Chạy service đang sửa bằng local debug (`mvn spring-boot:run` / IDE cho Java; `air` hoặc `go run` cho Go).
- Chạy các service còn lại bằng Docker khi cần integration.
- Với local process gọi container: dùng `localhost:<published-port>`; với container gọi container: dùng `<service-name>:<container-port>`.
- Nếu tách Compose file, chạy chúng trong cùng compose invocation hoặc chuẩn bị một external network. `depends_on` trong file services không tự quản lý infra ở project Compose khác.

Không nên dùng `docker compose down -v` như lệnh dev mặc định vì nó xóa dữ liệu database; dùng `down` thường, chỉ xóa volume khi chủ động reset môi trường.

## 9. Mô hình observability

```text
Application SDKs
  ├─ traces (OTLP) ─────────► OTel Collector ─► Jaeger
  ├─ metrics endpoint ──────► Prometheus ─────► Grafana
  └─ JSON logs + trace_id ──► stdout / Loki (optional) ─► Grafana
```

- Propagate W3C `traceparent` qua HTTP/gRPC metadata và RabbitMQ headers; không tự phát minh header `trace_id` riêng nếu instrumentation đã làm được.
- Đặt service name, deployment environment, version và order ID (không chứa PII) làm attributes có kiểm soát cardinality.
- Alert/dashboard chỉ cần phục vụ demo trước: high error rate, p95 latency, queue depth/DLQ count, consumer lag, DB/Redis error.

## 10. Danh sách “không làm ngay” để tránh over-engineering

- Distributed cache cluster, Redis Sentinel/Cluster, multi-region, sharding.
- Payment provider thật, PCI compliance, refund hoàn chỉnh.
- CQRS/Event Sourcing đầy đủ, API Gateway custom, service mesh.
- Consul/Eureka ngoài K8s DNS, Kafka song song với RabbitMQ, nhiều broker.
- CDC/Debezium trước khi ProductChanged event + indexer chạy ổn.
- Tối ưu performance khi chưa có metrics/load test chứng minh bottleneck.

## 11. Cách trình bày dự án trong portfolio/phỏng vấn

README và demo video nên trả lời ngắn gọn các câu hỏi sau:

1. Vì sao service boundary gồm Identity/Catalog/Order/Inventory, và vì sao chưa tách Cart/Payment/Notification?
2. Điều gì đảm bảo stock không âm, nếu Redis/RabbitMQ/service crash thì sao?
3. Vì sao dùng outbox + idempotent consumer thay vì “save DB rồi publish”?
4. Timeout/retry/circuit breaker được đặt thế nào và fallback nào là an toàn?
5. Một order lỗi được trace, debug và replay DLQ bằng cách nào?
6. K8s/Elasticsearch là upgrade nào, dữ liệu search nhất quán đến mức nào?
7. Vì sao tự viết first-party Identity Service nhưng không tự xây generic OAuth/OIDC provider?

Nếu trả lời được bằng test, dashboard, trace và ADR ngắn thay vì chỉ bằng lời, dự án sẽ thể hiện năng lực senior-oriented rõ hơn nhiều.

## 12. Definition of Done cuối roadmap

- [ ] Một lệnh khởi động local và một lệnh chạy smoke test.
- [ ] Identity Service tự viết có register/login/refresh/logout/profile, JWT/JWKS, password hash và refresh-token rotation được test.
- [ ] Checkout Saga có outbox, idempotency, retry/DLQ và inventory invariant được test.
- [ ] Error model gRPC/REST có stable code, trace ID, idempotent replay và không leak chi tiết nội bộ.
- [ ] Ma trận test Saga bao phủ duplicate, late/out-of-order, consumer crash, compensation và outbox publish failure.
- [ ] Kong/BFF/public-private boundary, JWT và rate limit hoạt động.
- [ ] gRPC contracts linted/versioned; Go và Java build lặp lại được.
- [ ] Trace end-to-end, metrics dashboard và structured logs hoạt động.
- [ ] ADR cho service boundary, outbox, stock invariant, fallback, compensation, error model và Identity security được cập nhật.
- [ ] README, architecture diagram, ADR và runbook đủ để người khác chạy/demo.
- [ ] Ít nhất một chaos/failure demo và một load/concurrency demo có kết quả ghi lại.

---

## Phụ lục: thứ tự đọc/triển khai chi tiết khi bắt đầu code

1. Foundation + repo/contract/compose + Identity key/config.
2. Identity register/login/refresh/profile, rồi Catalog + Inventory data invariants.
3. Order state machine và BFF/Kong authenticated happy path.
4. Outbox/RabbitMQ/Saga + idempotency trước Redis lock.
5. Cache, deadline, resilience, security hardening.
6. Tests + OTel/Prometheus/Jaeger/Grafana.
7. K8s, rồi Elasticsearch nếu còn thời gian.

Đây là thứ tự chủ đích: trước hết làm hệ thống **đúng**, sau đó **chịu lỗi**, rồi mới **quan sát, scale và tối ưu**.
