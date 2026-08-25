# Quy ước viết contract

Thư mục này chứa contract dùng chung giữa các bounded context. Domain logic vẫn
thuộc service sở hữu và không được đưa vào module contract.

## Protobuf và gRPC

Các file dưới `proto/` là source of truth cho gRPC:

- `catalog.v1`: lấy chi tiết, tìm kiếm và soft-delete sản phẩm.
- `inventory.v1`: đọc tồn kho khả dụng.
- `cart.v1`: đọc và thay đổi cart line.
- `identity.v1` và `order.v1`: contract đã định nghĩa nhưng server gRPC tương
  ứng chưa được application triển khai.
- `common.v1`: value dùng chung như `Money`.

`buf generate` sinh client/server types vào `go/` và
`java/src/main/generated/`. Không sửa generated source trực tiếp. Thay đổi
contract công khai phải giữ tương thích; ưu tiên thêm field/RPC và không tái sử
dụng field number đã phát hành.

## REST và OpenAPI

`openapi.yaml` là tài liệu OpenAPI 3.1 và hiện mới mô tả health foundation; nó
chưa bao phủ toàn bộ Identity/BFF REST surface. REST behavior hiện được kiểm
chứng bởi controller/handler tests và frontend API adapter. Khi mở rộng OpenAPI,
giữ envelope chuẩn của repository:

- Success: `{ "data": <payload>, "message": "..." }`.
- Error: `{ "code": "STABLE_ERROR_CODE", "message": "...", "errors": {}, "traceId": "..." }`.
- Pagination nằm trong `data` với `items`, `page`, `pageSize`, `totalItems`, và
  `totalPages`.

## Verification

```powershell
$gitRoot = Split-Path (Split-Path (Get-Command git).Source -Parent) -Parent
$env:PATH = "$gitRoot\usr\bin;$env:PATH"
buf format --diff --exit-code
buf lint
buf generate
git diff --exit-code -- contracts/go contracts/java/src/main/generated
```

Sau khi thay đổi protobuf, chạy build/test của generated Java module và mọi Go
hoặc Java consumer bị ảnh hưởng. Xem thêm
[`docs/steering/api-contracts.md`](../docs/steering/api-contracts.md) và
[`docs/instructions/build-and-test.md`](../docs/instructions/build-and-test.md).
