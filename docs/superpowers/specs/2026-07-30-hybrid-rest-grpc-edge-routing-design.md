# Thiết kế hybrid REST, BFF và gRPC Gateway

## 1. Trạng thái và mục tiêu

**Trạng thái:** Đã thống nhất ở mức thiết kế.

Quyết định này được ghi nhận ngắn gọn tại ADR-008. Nó thay thế phần route public trong ADR-001 vốn giới hạn commerce traffic ở BFF, nhưng không thay đổi service ownership, database boundary hoặc nguyên tắc các service chỉ nằm trên private network.

Tài liệu này quyết định cách định tuyến request trong hệ thống e-commerce khi:

- client bên ngoài sử dụng REST/JSON;
- các microservice nội bộ sử dụng gRPC/Protobuf;
- một số API chỉ chuyển tiếp đến một service;
- một số API phải tổng hợp dữ liệu hoặc tạo response riêng cho frontend;
- cùng một capability có thể được client và một microservice khác gọi.

Mục tiêu là giữ một public ingress qua Kong, tránh biến BFF thành internal service bus, không bắt mỗi service viết hai implementation nghiệp vụ REST và gRPC, đồng thời không dùng API gateway như một workflow engine.

## 2. Kết luận

Áp dụng kiến trúc hybrid theo loại endpoint:

1. **Kong là public ingress duy nhất.** Client không truy cập trực tiếp địa chỉ của BFF, gRPC Gateway hoặc domain service.
2. **API REST ánh xạ 1:1 vào một unary RPC đơn giản** có thể đi qua REST/gRPC transcoding mà không qua BFF.
3. **API cần tổng hợp, response riêng cho UI hoặc mapping nghiệp vụ phức tạp** đi qua BFF.
4. **Microservice nội bộ gọi trực tiếp gRPC endpoint của service đích** qua private network/service discovery; không gọi vòng qua Kong, BFF hoặc REST/gRPC transcoder.
5. **Identity tiếp tục expose REST qua Kong** vì đây là API edge-native hiện có, không cần ép chuyển thành gRPC trong MVP.
6. Chỉ những RPC được chủ đích đánh dấu public mới được transcoding. Không tự động public toàn bộ service hoặc toàn bộ proto.

```text
                                  ┌──────── gRPC trực tiếp ────────┐
                                  │                                ▼
Internal service ─────────────────┘                         Domain service

Client REST/JSON
       │
       ▼
     Kong
       ├──► Identity REST
       │
       ├──► REST/gRPC transcoding ──► một public unary RPC đơn giản
       │
       └──► BFF ──gRPC──► một hoặc nhiều domain service
```

## 3. Vai trò của từng thành phần

### 3.1 Kong

Kong chịu trách nhiệm cho các concern ở ingress:

- TLS termination và public routing;
- authentication/rate limit/request-size limit theo route;
- access log, metrics và trace propagation;
- chọn upstream là Identity, BFF hoặc transcoder;
- xóa hoặc ghi đè các identity header mà client có thể giả mạo.

Kong không chứa business workflow, không tổng hợp order và inventory, không quyết định domain state, và không thực hiện retry tùy tiện cho command không idempotent.

### 3.2 BFF

BFF là **client-facing adapter và composition layer**, không chỉ là “service tổng hợp thông tin”. BFF được dùng khi REST contract khác đáng kể so với internal RPC hoặc khi một use case cần nhiều downstream call.

BFF chịu trách nhiệm:

- thiết kế request/response phù hợp với frontend;
- gọi song song nhiều service khi các dependency độc lập;
- quản lý request budget/deadline cho các downstream call;
- chuyển caller context đáng tin cậy sang gRPC metadata;
- map gRPC status/domain reason sang REST error envelope ổn định;
- thực hiện orchestration ở mức trải nghiệm client, nhưng không sở hữu state machine nghiệp vụ của Order hay Inventory.

BFF không phải đường đi của service-to-service traffic.

### 3.3 REST/gRPC transcoder

Transcoder chuyển HTTP/JSON thành gRPC/Protobuf theo `google.api.http` annotation trong proto. Nó phù hợp khi public REST resource gần như tương ứng trực tiếp với một RPC.

Trong MVP, ưu tiên plugin gRPC-Gateway của Kong cho các route unary/read-only đơn giản vì Kong đã có trong hệ thống và không cần thêm một deployment. Nếu một route cần custom error handler, JSON shaping hoặc middleware vượt khả năng plugin, route đó phải chuyển sang BFF hoặc một generated `grpc-gateway` process riêng; không nhồi business logic vào cấu hình Kong.

Generated `grpc-gateway` là một reverse proxy Go độc lập, nên vẫn có thể đứng trước gRPC service viết bằng Java. Không yêu cầu domain service phải viết bằng Go.

### 3.4 Domain service

Catalog, Order và Inventory giữ gRPC là interface đồng bộ canonical cho nội bộ. Business validation và authorization trên resource vẫn nằm ở service sở hữu domain; gateway validation không thay thế domain invariant.

Một capability được client và server cùng gọi phải hội tụ vào cùng gRPC handler/use case. Không viết một implementation cho REST và một implementation khác cho gRPC.

## 4. Ma trận định tuyến đề xuất

| Nhóm route | Public path minh họa | Đường đi | Lý do |
|---|---|---|---|
| Authentication/profile | `/api/v1/auth/*`, `/api/v1/users/me` | Kong → Identity REST | Identity đã là edge-native HTTP API; auth flow cần kiểm soát cookie/token/header cụ thể. |
| Product lookup đơn giản | `/api/v1/products/{id}` | Kong transcoding → Catalog gRPC | Một REST GET ánh xạ 1:1 vào `CatalogService.GetProduct`. |
| Storefront/product page | `/api/v1/storefront/*` | Kong → BFF → Catalog/Inventory gRPC | Response phục vụ màn hình và có thể tổng hợp product với availability. |
| Create/read order cho client | `/api/v1/orders/*` | Kong → BFF → Order gRPC | Cần caller context, `Idempotency-Key`, REST error envelope và response ổn định theo client. |
| Inventory/reservation | Không public | Service → Inventory gRPC hoặc RabbitMQ | Đây là domain capability nội bộ, không phải public resource. |
| Service-to-service | Không có public path | Service → service gRPC trực tiếp | Tránh thêm hop, coupling và failure domain không cần thiết. |

Route table là allowlist. Một RPC mới mặc định là private cho tới khi có quyết định public API rõ ràng.

## 5. Cùng endpoint được client và server gọi

Ví dụ `CatalogService.GetProduct`:

```text
Client:
GET /api/v1/products/p-123
  → Kong/transcoding
  → CatalogService.GetProduct(id = "p-123")

Order Service:
Catalog gRPC client
  → CatalogService.GetProduct(id = "p-123")
```

Hai caller dùng hai transport path khác nhau nhưng cùng đến một application use case. Khác biệt chỉ nằm ở edge policy và caller context:

- client request chịu public authentication, rate limit và JSON mapping;
- internal request dùng private DNS, gRPC deadline và service identity;
- khi internal service hành động thay user, end-user identity được truyền bằng metadata có nguồn đáng tin cậy và service đích vẫn kiểm authorization cần thiết.

## 6. Quy tắc chọn BFF hay transcoding

Một route chỉ dùng transcoding khi **tất cả** điều kiện sau đúng:

- một HTTP operation ánh xạ vào đúng một unary RPC;
- JSON shape từ proto là public contract chấp nhận được;
- standard gRPC-to-HTTP status mapping đủ dùng hoặc đã được contract-test;
- không cần gọi thêm domain service khác;
- không cần workflow, fallback theo nghiệp vụ hoặc response riêng theo loại client;
- RPC đã được review và đánh dấu public có chủ đích.

Nếu một điều kiện không đúng, route đi qua BFF. Quy tắc ngắn:

```text
1 REST operation = 1 simple RPC          → transcoding
client-specific shape/composition        → BFF
service-to-service                       → direct gRPC
```

## 7. So sánh các phương án

| Tiêu chí | Tất cả commerce REST qua BFF | Hybrid được chọn | Mỗi service tự expose REST + gRPC |
|---|---|---|---|
| Public contract nhất quán | Rất tốt | Tốt nếu có contract test | Phụ thuộc từng team/service |
| Code pass-through | Nhiều | Ít | Có thể trùng adapter/model |
| Composition/client shaping | Tốt | Tốt qua BFF | Dễ phân tán và lặp lại |
| Service autonomy | BFF dễ thành bottleneck | Cân bằng | Cao nhưng public surface lớn |
| Coupling client với proto | Thấp | Trung bình ở route transcoded | Thấp đến trung bình |
| Số hop cho API đơn giản | Kong + BFF + service | Kong/transcoder + service | Kong + service |
| Vận hành | Một BFF dễ hiểu nhưng dễ phình | Nhiều loại route cần governance | Nhiều HTTP stack cần vận hành |
| Phù hợp repo hiện tại | Chấp nhận được | Tốt nhất | Không ưu tiên |

### Vì sao không chọn tất cả qua BFF

- BFF chứa nhiều handler chỉ copy request/response mà không tạo giá trị.
- Mọi thay đổi public API commerce đều buộc deploy BFF.
- BFF có nguy cơ trở thành monolithic gateway và bottleneck tổ chức.

### Vì sao không để mỗi service tự viết cả REST và gRPC

- Tăng surface cần bảo mật, documentation và contract testing.
- Dễ lặp validation/mapping hoặc làm hai transport có hành vi khác nhau.
- Các Go/Java service phải duy trì thêm HTTP framework và public concerns dù chỉ có một RPC đơn giản.

### Chi phí chấp nhận của hybrid

- Route ownership không còn suy ra chỉ từ URL; cần route catalog rõ ràng.
- Client có thể bị coupling với proto JSON shape ở route transcoded.
- Phải kiểm thử cả REST contract và gRPC contract.
- Team phải hiểu ranh giới giữa edge mapping, client composition và domain logic.

## 8. REST contract và proto evolution

- Dùng `google.api.http` annotation chỉ trên RPC public; RPC không annotation mặc định là internal.
- Public JSON contract phải được sinh hoặc mô tả bằng OpenAPI và kiểm thử độc lập với gRPC unit test.
- Proto evolution vẫn additive: không reuse field number, dùng `reserved` cho field đã xóa và chạy Buf breaking check.
- Không coi việc proto tương thích binary là đủ để kết luận REST/JSON tương thích. Đổi field name, enum representation, path binding hoặc JSON default có thể phá client.
- Không expose internal fields chỉ vì chúng đang có trong response message. Khi public và internal shape bắt đầu khác đáng kể, tạo public RPC/message rõ ràng hoặc chuyển route sang BFF.
- Transcoding ở MVP chỉ áp dụng cho unary RPC. Streaming cần một quyết định transport riêng như gRPC-Web, SSE hoặc WebSocket.

## 9. Authentication, authorization và metadata

### Client request

1. Kong xác thực token theo policy của route.
2. Kong strip các header như `x-user-id`, `x-user-roles` do client gửi và chỉ tạo/forward caller context từ credential đã xác thực.
3. `Authorization`, W3C `traceparent`, correlation metadata và request deadline/budget được propagate theo allowlist.
4. BFF hoặc domain service kiểm tra issuer/audience/signature khi policy defense-in-depth yêu cầu.
5. Service sở hữu dữ liệu vẫn kiểm authorization/ownership; việc request đã qua Kong không đủ để bỏ kiểm tra domain authorization.

### Internal request

- Service gọi DNS/private gRPC endpoint trực tiếp.
- Mọi call có deadline; retry chỉ áp dụng cho operation idempotent và có retry budget.
- Service identity nên được xác thực bằng workload identity/mTLS khi sang K8s; private network một mình không phải mô hình bảo mật production đầy đủ.
- Không tin metadata end-user từ nguồn public hoặc service không được phép đại diện user.

## 10. Error, timeout và retry

- Domain service trả standard gRPC status cùng stable domain reason.
- BFF map lỗi sang REST envelope `{code, message, trace_id}` như ADR-006.
- Route transcoded chỉ được public khi mapping HTTP status/body đã được contract-test. Nếu Kong plugin không thể tạo public error contract yêu cầu, route phải đi qua BFF hoặc generated gateway có custom error handler.
- Kong timeout phải lớn hơn budget của adapter; BFF/transcoder timeout phải lớn hơn deadline downstream một khoảng nhỏ để downstream fail trước upstream.
- Không retry tự động `CreateOrder` ở gateway nếu chưa xác nhận `Idempotency-Key` và replay semantics. Read-only `GetProduct` có thể retry hữu hạn với backoff/jitter trong request budget.
- Cancellation từ client nên được propagate đến gRPC call để tránh tiếp tục xử lý không cần thiết, nhưng service không được giả định cancellation sẽ rollback side effect đã commit.

## 11. Observability

Một request phải giữ cùng trace qua HTTP và gRPC:

```text
Client → Kong → BFF/transcoder → gRPC service → downstream
```

Các thành phần ghi nhận tối thiểu:

- route name và upstream type (`identity`, `bff`, `transcoded-grpc`);
- HTTP method/path/status ở edge;
- gRPC service/method/status ở adapter và service;
- latency từng hop, deadline exceeded và cancellation;
- `trace_id`, không log access token, refresh token hoặc payload nhạy cảm.

Dashboard cần tách latency/error của BFF route và transcoded route để đánh giá lựa chọn hybrid bằng dữ liệu thay vì cảm tính.

## 12. Testing bắt buộc

### Contract test

- REST path/query/body ánh xạ đúng proto field.
- gRPC status/domain reason ánh xạ đúng HTTP status/error body.
- JSON field name, enum, 64-bit number, empty/default value và unknown field có hành vi đã định nghĩa.
- OpenAPI và annotated proto không drift khỏi route Kong.

### Security test

- Không thể bypass Kong để truy cập public service từ external network.
- Spoofed identity header bị strip/overwrite.
- Protected route từ chối token thiếu, sai issuer/audience hoặc hết hạn.
- Internal-only RPC không có public route.

### Resilience test

- Deadline/cancellation được propagate.
- Catalog down trả lỗi hữu hạn, không retry vô hạn.
- Create order lặp với cùng idempotency semantics không tạo order thứ hai.
- BFF aggregation xử lý partial dependency failure theo policy đã định nghĩa; transcoder không tự tạo fallback nghiệp vụ.

## 13. Triển khai theo giai đoạn

1. Giữ route Identity và Storefront hiện tại.
2. Thêm `google.api.http` dependency/annotation cho đúng một RPC thử nghiệm: `CatalogService.GetProduct`.
3. Cấu hình Kong gRPC-Gateway plugin cho `/api/v1/products/{id}`; Catalog vẫn chỉ expose gRPC trong private network.
4. Sinh OpenAPI từ proto hoặc duy trì contract artifact tương đương; thêm smoke/contract test qua Kong.
5. Thêm BFF route cho order và storefront; BFF gọi direct gRPC, không gọi lại Kong.
6. Đo latency/error và kiểm tra độ phức tạp mapping trước khi public thêm RPC transcoded.

Không chuyển hàng loạt mọi RPC sang public route trong cùng một bước.

## 14. Tiêu chí hoàn thành

- Client chỉ có một public base URL tại Kong.
- `GetProduct` minh họa được REST client và internal gRPC caller cùng dùng một domain implementation.
- Storefront minh họa được BFF aggregation qua gRPC.
- Service-to-service traffic không đi qua BFF/Kong/transcoder.
- Inventory RPC không thể gọi từ public network.
- REST và gRPC contract test chạy tự động.
- Trace thể hiện đúng từng hop và giữ nguyên trace context.
- Error, deadline, retry và identity propagation tuân theo ADR-006 và ADR-007.

## 15. Khi nào cần xem xét lại quyết định

Xem xét thay route transcoded bằng BFF hoặc generated gateway riêng khi có một trong các dấu hiệu:

- public JSON model thay đổi độc lập với proto model;
- cần custom error envelope/middleware mà Kong plugin không đáp ứng sạch;
- một endpoint bắt đầu gọi nhiều service hoặc có client-specific logic;
- route transcoding làm lộ internal field hoặc buộc client chạy theo nhịp thay đổi nội bộ;
- governance proto annotation và Kong config trở thành bottleneck.

Xem xét bỏ BFF khỏi một route khi handler chỉ pass-through ổn định trong thời gian dài và đáp ứng đầy đủ điều kiện transcoding ở mục 6.

## 16. Tài liệu tham khảo

- [Microsoft: Backends for Frontends pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/backends-for-frontends)
- [Microsoft: API gateway pattern và direct client-to-microservice](https://learn.microsoft.com/en-us/dotnet/architecture/microservices/architect-microservice-container-applications/direct-client-to-microservice-communication-versus-the-api-gateway-pattern)
- [gRPC-Gateway: Introduction](https://grpc-ecosystem.github.io/grpc-gateway/docs/tutorials/introduction/)
- [gRPC-Gateway: FAQ và giới hạn mapping](https://grpc-ecosystem.github.io/grpc-gateway/docs/faq/)
- [Kong: gRPC-Gateway plugin](https://developer.konghq.com/plugins/grpc-gateway/)
