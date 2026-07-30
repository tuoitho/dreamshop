# Roadmap Pet Project E-commerce Microservices (8–12 tuần)

> Mục tiêu của tài liệu này là xây một portfolio có chiều sâu kỹ thuật: hoàn thành được một luồng mua hàng đáng tin cậy, giải thích được các trade-off, và có thể demo lỗi/phục hồi. Không phải cố đưa thật nhiều công nghệ vào cùng lúc.

## 1. Kết luận nhanh

Kế hoạch ban đầu có nền tảng tốt: lựa chọn Java + Go, gRPC, RabbitMQ, Redis, Kong và observability đều phù hợp để học kiến trúc phân tán. Tuy nhiên, nếu triển khai tất cả service và nâng cấp cùng lúc thì 8–12 tuần sẽ dễ thành “demo nhiều công nghệ” nhưng thiếu tính đúng đắn.

**Quyết định đề xuất:** bắt đầu bằng **4 service tối thiểu có chủ đích**, hoàn thiện một checkout flow end-to-end, sau đó mới thêm K8s và Elasticsearch.

| Thành phần | Công nghệ | Trách nhiệm chính |
|---|---|---|
| BFF | Go | REST/JSON cho client, tổng hợp dữ liệu, không chứa nghiệp vụ đơn hàng cốt lõi |
| Catalog Service | Go | Sản phẩm, giá niêm yết, Redis cache |
| Order Service | Java + Spring Boot | Đơn hàng, idempotency, Saga state, transactional outbox |
| Inventory Service | Go | Reservation tồn kho, chống oversell, xử lý event |
| Edge & infra | Kong, PostgreSQL, Redis, RabbitMQ | Ingress, data, cache/utility, message broker |

Không xây ngay: payment thật, review, notification hoàn chỉnh, recommendation, nhiều loại client, service discovery riêng, hay 10+ microservice.

## 2. Mục tiêu học tập và tiêu chí hoàn thành

Pet project này nên chứng minh năng lực ở cấp **middle hướng senior** qua cách ra quyết định và vận hành hệ thống, không chỉ qua danh sách framework.

Khi hoàn thành MVP, cần demo được các tình huống sau:

1. Client gọi một REST endpoint qua Kong và BFF để xem sản phẩm hoặc đặt hàng.
2. BFF gọi gRPC nội bộ; request có deadline, correlation/trace context và không đi vòng qua BFF giữa các service.
3. Đặt hàng lặp lại cùng `Idempotency-Key` chỉ tạo một order.
4. Hai request cạnh tranh số hàng cuối cùng chỉ có một reservation thành công; tồn kho không âm.
5. RabbitMQ có retry hữu hạn, DLQ và consumer idempotent; event không mất khi service chết ngay sau DB commit.
6. Có thể nhìn một request từ BFF → Order → RabbitMQ → Inventory trên trace; metrics và log cho biết service nào lỗi/chậm.
7. Tắt Catalog hoặc Inventory có kiểm soát: timeout/circuit breaker/bulkhead không làm cạn tài nguyên của service gọi.

## 3. Kiến trúc đích của MVP

```text
Browser / API client
        │ HTTPS + Bearer JWT + Idempotency-Key
        ▼
Kong Gateway ──────────────── public ingress: route, rate limit, JWT/OIDC mode
        │
        ▼
BFF (Go, REST) ─────── gRPC ──────► Catalog (Go) ──► catalog_db + Redis cache
        │
        └────────────── gRPC ──────► Order (Java) ──► order_db + outbox
                                               │
                                               ▼
                                         RabbitMQ exchanges
                                               │
                                               ▼
                                Inventory (Go) ──► inventory_db + Redis
```

### Nguyên tắc biên service

- Mỗi service sở hữu database/schema của mình. Trong local có thể dùng cùng một PostgreSQL instance, nhưng **không query hay foreign key xuyên service**.
- BFF là edge adapter cho client; BFF không là “bus” để microservice gọi nhau.
- Chỉ Kong/BFF/Auth endpoint là public. Catalog, Order, Inventory chỉ nghe trên network nội bộ.
- gRPC dùng cho truy vấn/command đồng bộ có phản hồi cần ngay; RabbitMQ dùng cho event sau khi state đã được commit.
- RabbitMQ giao hàng **at-least-once**, do đó mọi consumer phải chịu được bản tin trùng.

## 4. Luồng nghiệp vụ trọng tâm: đặt hàng và reserve tồn kho

Đây là phần đáng đầu tư nhất của dự án.

1. Client gửi `POST /orders` qua Kong/BFF, kèm JWT và `Idempotency-Key`.
2. BFF chuyển identity đã được xác thực và command sang Order Service. Order không tin giá/total từ client; nó lấy và snapshot thông tin giá sản phẩm từ Catalog qua gRPC.
3. Trong **một transaction**, Order lưu `PENDING` order, dòng order item, bản ghi idempotency, và bản ghi outbox `OrderCreated`.
4. Outbox publisher phát `OrderCreated` sau commit, có publisher confirm. Nếu phát thất bại, job retry sẽ phát lại.
5. Inventory consume event, deduplicate theo `event_id`, rồi dùng transaction/SQL conditional update để tạo reservation.
6. Inventory phát một trong hai event: `InventoryReserved` hoặc `InventoryReservationRejected`.
7. Order consume event và chuyển order thành `CONFIRMED` hoặc `CANCELLED`. Một order đã terminal không được chuyển lại bởi event đến muộn/trùng.

`PENDING → CONFIRMED/CANCELLED` là Saga choreography nhỏ, đủ để học eventual consistency. Chưa cần payment thật; nếu muốn mở rộng sau này, thêm `PAYMENT_PENDING` và payment adapter giả lập.

## 5. Nhận xét và chỉnh sửa các điểm quan trọng trong bản gốc

| Nội dung gốc | Nhận xét / quyết định hoàn thiện |
|---|---|
| Product Go, Order Java, Inventory Go | Hợp lý. Đừng chọn ngôn ngữ chỉ vì “Go nhanh”; chọn vì bạn muốn học concurrency/gRPC trong Go và domain workflow trong Spring Boot. |
| Redis Lock để chống oversell | **Không xem Redis lock là nguồn đảm bảo đúng đắn.** Đảm bảo chính phải nằm ở inventory DB: conditional update `available >= quantity`, transaction và unique reservation theo order. Redis lock chỉ là bài học/biện pháp bổ sung cho hot SKU, có TTL, fencing/timeout và không thay DB invariant. |
| Order tạo `PENDING` rồi Inventory trừ kho | Đúng hướng nhưng thiếu nhánh thất bại. Bắt buộc thêm `InventoryReservationRejected → Order CANCELLED`, retry/DLQ và idempotent consumer. |
| Publish RabbitMQ sau khi lưu DB | Cần **transactional outbox**. Nếu không, process có thể chết giữa DB commit và publish khiến order kẹt mãi. |
| JWT chỉ check ở Kong, service “ngây thơ” | Kong nên chặn request ở ingress, nhưng downstream không được tin header tùy ý từ public network. Giữ service private, strip/overwrite header tại gateway, và kiểm tra authorization/ownership ở service xử lý dữ liệu nhạy cảm. |
| Kong route thẳng service hoặc BFF | Về kỹ thuật làm được; với dự án này mặc định route vào BFF. Ngoại lệ hẹp: auth callback hoặc payment webhook. Không công khai generic internal service endpoint chỉ để “cho đủ microservice”. |
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
- Docker Compose cho PostgreSQL, Redis, RabbitMQ, Kong; cấu hình `.env.example` không chứa secret thật.
- Proto v1 cho Catalog, Order, Inventory; Buf lint/generate hoặc script `protoc` duy nhất.
- Health/readiness endpoint, structured JSON logs và config theo environment cho mỗi service.
- README: sơ đồ hệ thống, cách chạy, port map và happy-path API.

**Exit criteria:** `make up` (hoặc lệnh tương đương) khởi động infra; mỗi service expose health check; generated client/server build được ở Go lẫn Java.

### Phase 1 — Happy-path core (tuần 2–3)

**Mục tiêu:** có dữ liệu thật và một request đi qua BFF/gRPC.

- Catalog CRUD tối thiểu và `GetProduct`/`ListProducts` gRPC.
- Inventory seed SKU và query availability; chưa cần event flow phức tạp ở ngày đầu.
- Order tạo order với item snapshot và state machine tối thiểu.
- BFF REST: product list/detail và create order; aggregation chạy song song khi các query độc lập.
- Kong route chỉ vào BFF; rate limiting cơ bản.

**Exit criteria:** gọi BFF qua Kong tạo được order `PENDING`, không truy cập trực tiếp database của service khác.

### Phase 2 — Consistency và asynchronous workflow (tuần 4–6)

**Mục tiêu:** biến happy path thành workflow chịu retry/failure.

- Transactional outbox tại Order và Inventory; publisher có retry/publisher confirm.
- RabbitMQ topology rõ ràng: domain exchange, queue theo consumer, retry có giới hạn, DLQ.
- Inventory reservation với DB invariant, consumer dedup/inbox, event outcome.
- Saga state transitions trong Order; test các event trùng và đến sai thứ tự.
- Idempotency bền vững bằng unique key trong Order DB; Redis có thể hỗ trợ fast-path nhưng không là nguồn duy nhất.

**Exit criteria:** fire 20–100 request cạnh tranh vào SKU tồn kho nhỏ; số reservation không vượt tồn kho và order cuối cùng chỉ ở `CONFIRMED` hoặc `CANCELLED`.

### Phase 3 — Performance, security, resilience (tuần 6–8)

**Mục tiêu:** chứng minh hệ thống có hành vi có chủ đích khi tải hoặc dependency lỗi.

- Cache-aside cho Catalog, TTL hợp lý và invalidation khi product thay đổi. Cache price/product detail, không dùng cache làm nguồn tồn kho.
- JWT/OIDC flow cho ingress phù hợp bản Kong đang dùng; rate limit và request-size limit.
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
- `orders`: unique `(user_id, idempotency_key)`; index các query vận hành thực sự dùng, ví dụ `(user_id, created_at DESC)` và status khi dashboard/filter cần.
- `inventory_reservations`: unique `order_id`/`reservation_id` để retry không double-reserve.
- Đo query bằng `EXPLAIN ANALYZE` trước khi thêm index; index tăng chi phí write và disk.

### Proto và compatibility

- Tổ chức `proto/catalog/v1`, `proto/order/v1`, `proto/inventory/v1`; producer sở hữu contract của domain mình.
- Thay đổi ưu tiên additive: thêm field mới, không đổi/reuse field number; dùng `reserved` cho field đã xóa.
- BFF chỉ generate/import proto mà nó gọi. Không dùng shared `common.proto` làm “god object”.
- Buf là tùy chọn được khuyến nghị; generated code không phải source of truth. CI phải có bước generate + kiểm tra clean diff hoặc build từ generated artifact rõ ràng.

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
- Kong xác thực ingress theo plugin/chế độ mà bản cài đặt hỗ trợ. BFF/service chỉ nhận traffic private; gateway phải xóa/ghi đè identity headers từ client.
- Phân biệt authentication (ai gọi) và authorization (được làm gì). Order vẫn kiểm ownership của order.
- Làm mTLS/network policy là stretch goal sau khi K8s chạy ổn.

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

1. Vì sao service boundary là Catalog/Order/Inventory, và vì sao chưa tách Cart/Payment/Notification?
2. Điều gì đảm bảo stock không âm, nếu Redis/RabbitMQ/service crash thì sao?
3. Vì sao dùng outbox + idempotent consumer thay vì “save DB rồi publish”?
4. Timeout/retry/circuit breaker được đặt thế nào và fallback nào là an toàn?
5. Một order lỗi được trace, debug và replay DLQ bằng cách nào?
6. K8s/Elasticsearch là upgrade nào, dữ liệu search nhất quán đến mức nào?

Nếu trả lời được bằng test, dashboard, trace và ADR ngắn thay vì chỉ bằng lời, dự án sẽ thể hiện năng lực senior-oriented rõ hơn nhiều.

## 12. Definition of Done cuối roadmap

- [ ] Một lệnh khởi động local và một lệnh chạy smoke test.
- [ ] Checkout Saga có outbox, idempotency, retry/DLQ và inventory invariant được test.
- [ ] Kong/BFF/public-private boundary, JWT và rate limit hoạt động.
- [ ] gRPC contracts linted/versioned; Go và Java build lặp lại được.
- [ ] Trace end-to-end, metrics dashboard và structured logs hoạt động.
- [ ] README, architecture diagram, ADR và runbook đủ để người khác chạy/demo.
- [ ] Ít nhất một chaos/failure demo và một load/concurrency demo có kết quả ghi lại.

---

## Phụ lục: thứ tự đọc/triển khai chi tiết khi bắt đầu code

1. Foundation + repo/contract/compose.
2. Catalog + Inventory data invariants.
3. Order state machine và BFF/Kong happy path.
4. Outbox/RabbitMQ/Saga + idempotency trước Redis lock.
5. Cache, deadline, resilience, security.
6. Tests + OTel/Prometheus/Jaeger/Grafana.
7. K8s, rồi Elasticsearch nếu còn thời gian.

Đây là thứ tự chủ đích: trước hết làm hệ thống **đúng**, sau đó **chịu lỗi**, rồi mới **quan sát, scale và tối ưu**.
