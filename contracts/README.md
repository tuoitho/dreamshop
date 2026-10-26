# Quy ước viết contract

Thư mục này chứa contract dùng chung giữa các bounded context. Domain logic vẫn
thuộc service sở hữu và không được đưa vào module contract.

## Protobuf và gRPC

Các file dưới `proto/` là source of truth cho gRPC:

- `ecommerce.catalog.v1`: lấy chi tiết, tìm kiếm và soft-delete/restore sản phẩm.
- `ecommerce.inventory.v1`: đọc tồn kho khả dụng.
- `ecommerce.cart.v1`: đọc và thay đổi cart line.
- `ecommerce.identity.v1`: `AuthorizationService` được triển khai cho kiểm tra
  phân quyền riêng tư từ Catalog (`product.delete`) và Cart (`cart.item.add`);
  RPC `IdentityService.GetUser` cũ hiện chưa được nối dây.
- `ecommerce.order.v1`: contract đã định nghĩa nhưng server gRPC tương ứng chưa
  được application triển khai.
- `ecommerce.common.v1`: value dùng chung như `Money`.

`buf generate` sinh client/server types vào `go/` và
`java/src/main/generated/`. Không sửa generated source trực tiếp. Thay đổi
contract công khai phải giữ tương thích; ưu tiên thêm field/RPC và không tái sử
dụng field number đã phát hành.

## REST và OpenAPI

`openapi.yaml` hiện chỉ là file placeholder khung ban đầu và chưa phản ánh REST
surface hay envelope thực tế của Identity/BFF. REST behavior hiện được kiểm
chứng bởi controller/handler tests và frontend API adapter. Khi cập nhật hoặc mở
rộng OpenAPI, phải đồng bộ theo đúng envelope chuẩn của repository:

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
