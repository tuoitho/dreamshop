# ADR-008: Hybrid routing giữa REST, BFF và gRPC transcoding

## Status

Accepted

## Context

Client bên ngoài sử dụng REST/JSON trong khi Catalog, Order và Inventory dùng gRPC/Protobuf cho giao tiếp đồng bộ nội bộ. Một số client endpoint cần tổng hợp hoặc response riêng cho UI, nhưng những endpoint khác chỉ ánh xạ 1:1 tới một RPC. Nếu mọi commerce request đều qua BFF, BFF sẽ có nhiều pass-through handler và dễ trở thành bottleneck. Nếu mỗi service tự viết cả REST và gRPC, public surface và chi phí duy trì contract tăng. Service-to-service traffic cũng không nên đi vòng qua edge components.

ADR này thay thế riêng phần public routing của ADR-001. Các quyết định về service ownership, database boundary và private service network trong ADR-001 vẫn giữ nguyên.

## Decision

Áp dụng hybrid routing theo loại endpoint:

- Kong là public ingress duy nhất.
- Identity REST tiếp tục đi trực tiếp từ Kong tới Identity Service.
- REST operation ánh xạ 1:1 vào một unary RPC đơn giản có thể dùng gRPC transcoding tại Kong. Chỉ RPC có `google.api.http` annotation được review mới được public.
- Endpoint cần composition, client-specific response, custom error mapping hoặc orchestration đi qua BFF; BFF gọi các domain service bằng gRPC trực tiếp.
- Microservice gọi nhau bằng gRPC trực tiếp trên private network hoặc bằng RabbitMQ cho event flow; không gọi qua Kong, BFF hay transcoder.
- Kong/transcoder không chứa business workflow hoặc fallback làm thay đổi domain semantics.
- Route transcoded phải có REST contract và security test. Nếu mapping của Kong không đáp ứng public contract, route chuyển sang BFF hoặc một generated `grpc-gateway` adapter riêng. (Hiện tại các route storefront, cart và admin products đều đi qua BFF; chưa có `.proto` nào bật `google.api.http` transcoding.)

## Consequences

- Được: giảm pass-through code trong BFF cho API đơn giản, nhưng vẫn giữ BFF cho composition và client contract phức tạp.
- Được: cùng một gRPC application use case phục vụ được REST client qua transcoding và internal caller qua direct gRPC.
- Được: service-to-service traffic không phụ thuộc availability hoặc latency của edge layer.
- Mất: route governance phức tạp hơn vì Kong có nhiều loại upstream.
- Mất: REST client có thể coupling với proto JSON shape ở route transcoded; proto compatibility không tự đảm bảo REST compatibility.
- Trade-off: phải duy trì contract test, public RPC allowlist và observability tách theo BFF/transcoded route.
- Xem xét lại: chuyển route về BFF/generated gateway nếu cần JSON shape, error envelope hoặc middleware khác đáng kể so với RPC; bỏ BFF khỏi route nếu nó chỉ pass-through ổn định và đáp ứng đầy đủ điều kiện transcoding.
