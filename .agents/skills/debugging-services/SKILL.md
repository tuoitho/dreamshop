---
name: debugging-services
description: Dùng trước khi đề xuất hoặc viết fix cho bất kỳ bug, test fail, flaky test hay hành vi sai nào trong monorepo, đặc biệt là gRPC DeadlineExceeded/Unavailable/PermissionDenied, lệch serialization giữa proto và generated code, route Kong/BFF sai, dữ liệu lệch giữa các service, race condition hoặc lỗi chỉ xuất hiện khi chạy full stack. Bắt buộc tìm ra root cause có evidence rồi mới sửa.
---

# Debugging Services

**Luật cứng:** chưa có root cause được evidence xác nhận thì chưa sửa code. Thử-sai
trong hệ phân tán thường chỉ che triệu chứng ở một service trong khi lỗi nằm ở
boundary.

## 1. Tái hiện

- Ghi lại chính xác: request/command, input, kết quả mong đợi, kết quả thực tế,
  service và port liên quan.
- Tái hiện ở scope nhỏ nhất: unit/package test, rồi gRPC/HTTP trực tiếp tới
  provider, rồi qua BFF, cuối cùng mới qua Kong.
- Lỗi không tái hiện được thì thu thập thêm evidence (bước 2); không đoán fix.
- Chạy service theo quy tắc long-running process trong `AGENTS.md`: chạy nền, log ra
  file, health check, ghi PID và dừng khi xong.

## 2. Khoanh vùng boundary

Đi theo đường request và tìm **hop đầu tiên** mà dữ liệu hoặc status bị sai:

```text
ecommerce-fe -> Kong :8000 -> BFF :8081 -> gRPC provider (Catalog :50051, Cart :50053)
                                            Cart -> Catalog :50051, Inventory :50052, Identity :50054
                                            provider -> PostgreSQL <service>_db
```

Ở mỗi hop, so input nhận được với output trả ra. Gọi thẳng hop phía sau để chia đôi
vùng lỗi, ví dụ gọi BFF `:8081` trực tiếp để loại trừ Kong. Health endpoint của từng
service nằm trong `docs/steering/service-catalog.md`.

## 3. Checklist theo loại lỗi

**gRPC timeout hoặc Unavailable**

- Provider có đang lắng nghe đúng port không (`Get-NetTCPConnection -LocalPort <port>`
  hoặc `ss -ltnp`)? Health endpoint trả gì?
- Caller dùng deadline nào? Cart đọc `CATALOG_GRPC_TIMEOUT`, `INVENTORY_GRPC_TIMEOUT`
  (mặc định `3s`) và `IDENTITY_GRPC_TIMEOUT` (`500ms`); BFF đọc
  `BFF_CATALOG_GRPC_TIMEOUT` và `BFF_CART_GRPC_TIMEOUT`.
- Thứ tự khởi động: Identity phải chạy trước Cart.
- Caller map status sang lỗi nào? Ví dụ Cart trả `Unavailable` khi authorization
  không phản hồi (REST `503`) và `PermissionDenied` khi bị từ chối (`403`). Xác
  định lỗi sinh ra ở provider hay do caller map sai.

**Serialization hoặc contract mismatch**

- Chạy `make verify SERVICE=contracts`: generated code phải khớp proto.
- Caller và provider có build từ cùng một revision của `contracts/` không? Service
  đang chạy có được build lại sau khi generate không?
- Field bị đổi số, đổi kiểu hoặc enum value không biết bị xử lý như lỗi?
- REST/JSON dùng `camelCase`, proto dùng `lower_snake_case`: kiểm tra mapping ở BFF
  hoặc lớp transcoding.

**Route hoặc auth ở edge**

- So route trong `deployments/kong/kong.yaml` với path thực tế; Kong local proxy tới
  `host.docker.internal:8081`.
- Phân biệt lỗi do Kong plugin (JWT, caller-context) với lỗi do service bằng cách
  gọi thẳng service.

**Dữ liệu lệch giữa service**

- Mỗi service chỉ đọc database của chính nó. Khi chẩn đoán, dùng `psql` chỉ đọc
  trên đúng `<service>_db`, lấy credential từ `start-local` script hoặc `.env`.
- Dữ liệu projection (ví dụ Cart lấy product từ Catalog) được lấy qua gRPC, không
  qua join database. Tìm hop trả dữ liệu cũ hoặc sai.

**Async, outbox, event**

- Hiện chưa service nào publish hoặc consume RabbitMQ. Khi đã có, lần theo chuỗi:
  bản ghi outbox → exchange/queue (management `:15672`) → bản ghi dedupe ở consumer,
  theo ADR-002.

**Race condition hoặc flaky**

- Lặp lại test để đo tần suất: `go test -race -count=50 -run <Test> ./<pkg>` (Go);
  lặp test class nhiều lần với Maven.
- Thay `sleep` bằng chờ theo điều kiện có timeout. Không tăng timeout để test xanh
  khi chưa biết vì sao chậm.

## 4. Giả thuyết và kiểm chứng

- Viết một giả thuyết cụ thể: "X sai vì Y, evidence là Z".
- Kiểm chứng bằng thay đổi nhỏ nhất hoặc log tạm thời, mỗi lần một biến. Xóa log
  tạm thời khi xong.
- Giả thuyết sai thì quay lại bước 2 với evidence mới; không chồng thêm fix.

## 5. Sửa

1. Viết test fail tái hiện đúng root cause, ở level thấp nhất có thể
   (`docs/steering/testing.md`). Chạy và xác nhận nó fail đúng lý do.
2. Sửa tại service sở hữu root cause, không vá triệu chứng ở consumer.
3. Chạy focused test, rồi `make verify SERVICE=<service>` cho service đã sửa và
   mọi consumer liên quan.

Nếu 3 lần sửa cho cùng root cause đều thất bại, dừng lại: vấn đề có thể nằm ở thiết
kế hoặc contract. Báo người dùng các giả thuyết đã loại trừ, evidence và quyết định
cần họ đưa ra.

## Báo cáo

- triệu chứng và cách tái hiện;
- hop lỗi và root cause kèm evidence;
- test tái hiện, fix, và commands verify cùng kết quả;
- giả thuyết đã loại trừ và rủi ro còn lại.
