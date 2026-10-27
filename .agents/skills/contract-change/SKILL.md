---
name: contract-change
description: Dùng khi thêm, sửa hoặc xóa một contract dùng chung giữa các surface trong monorepo, gồm protobuf message/RPC/enum trong contracts/proto, generated Go/Java module, google.api.http annotation, Kong route, REST envelope/field mà frontend hoặc BFF dùng, hoặc event schema. Skill dẫn qua phân loại compatibility, tìm consumer, buf format/lint/breaking/generate, triển khai provider trước consumer và báo cáo rollout.
---

# Contract Change

Đổi contract như một boundary giữa các service deploy độc lập, không như code dùng
chung. Luật chi tiết nằm ở `docs/steering/api-contracts.md` và
`docs/steering/change-impact.md`; skill này là trình tự thực hiện.

## 1. Xác định contract và provider

| Contract | Nguồn | Provider (owner) |
|---|---|---|
| gRPC/protobuf | `contracts/proto/<domain>/v1/*.proto` | `services/<domain>-service` |
| Kiểu dùng chung | `contracts/proto/common/v1/common.proto` | Mọi service import nó; xem như contract của tất cả |
| Generated code | `contracts/go/`, `contracts/java/src/main/generated/` | Sinh bằng `buf generate`; không sửa tay |
| Public REST | Controller của Identity, handler của BFF, adapter Go `grpc-gateway` ở service (khi được duyệt), `deployments/kong/*.yaml` | Service phục vụ route |
| Event | Chưa có producer/consumer RabbitMQ; khi thêm, theo mục Events của `api-contracts.md` và ADR-002 | Service phát event |

## 2. Phân loại compatibility

Phân loại theo tác động lên consumer hiện có, không theo kích thước diff. Phân vân
giữa hai loại thì chọn loại chặt hơn.

- `additive`: consumer hiện có chạy nguyên vẹn. Ví dụ: field optional mới, RPC mới,
  enum value mới mà consumer xử lý an toàn khi không biết, error reason mới.
- `breaking`: consumer phải đổi. Ví dụ: xóa, đổi tên, đổi kiểu hoặc đánh số lại
  field; input optional thành required; xóa RPC/route; đổi status hoặc error `code`.
- `semantic`: shape giữ nguyên, ý nghĩa đổi. Ví dụ: default, ordering, filtering,
  authorization, state transition, đơn vị, idempotency scope.

`breaking` và `semantic` là quyết định high-risk: dừng lại, trình bày impact và đề
xuất staged migration (field/RPC mới hoặc package `v2`, cửa sổ tương thích, điều
kiện gỡ bản cũ), rồi chờ người dùng duyệt. Đưa về dạng additive khi có thể.

## 3. Tìm mọi consumer

Code là nguồn sự thật; `docs/steering/service-catalog.md` (cột Inbound/Outbound)
dùng để đối chiếu. Nếu hai nguồn lệch nhau, báo lại.

```powershell
# Go consumers của một proto package
git grep -n "ecommerce.local/contracts/go/<domain>/v1" -- services
# Java consumers
git grep -n "com.ecommerce.contracts.<domain>.v1" -- services
# Public exposure qua transcoding hoặc route
git grep -n "google.api.http" -- contracts/proto/<domain>
git grep -n "<route hoặc tên RPC>" -- deployments/kong
# REST consumers
git grep -n "<path hoặc field>" -- ecommerce-fe/src services/bff-service
```

Ghi inventory theo `change-impact.md`: CONTRACT đã đổi, DEPENDENT phải sửa, và
DEPENDENT chỉ inspect rồi giữ nguyên (kèm lý do).

## 4. Sửa nguồn contract

- Protobuf field giữ `lower_snake_case`. Không đánh số lại field. Khi xóa field hoặc
  enum value, thêm `reserved` cho cả số và tên.
- Consumer phải chịu được field không biết, field optional vắng mặt và enum value
  không biết.
- Không thêm `google.api.http`, service-side Go `grpc-gateway` hoặc Kong route cho RPC
  nội bộ nếu chưa được duyệt (ADR-011); không dùng plugin `grpc-gateway` của Kong. Khi
  expose direct domain REST bằng Go `grpc-gateway`, phải giữ chuẩn `camelCase`,
  envelope `{ "data", "message" }` và error envelope của repo. Internal caller gọi
  gRPC trực tiếp, không qua Kong, BFF hay cổng HTTP của service khác.
- Error dùng gRPC status chuẩn cộng domain reason ổn định; REST dùng error envelope
  trong `AGENTS.md`.

Gate contract:

```powershell
make verify SERVICE=contracts
```

Lệnh này chạy boundary check, `buf format`, `buf lint`, `buf breaking` so với
`main`, `buf generate` và kiểm tra generated output đã đồng bộ và được track. Commit
generated output cùng commit với thay đổi proto.

`buf breaking` fail nghĩa là thay đổi đang breaking. Không tắt rule để qua gate;
quay lại bước 2.

## 5. Triển khai theo thứ tự provider trước

1. Provider: implement và test behavior mới, rồi chạy
   `make verify SERVICE=<provider>-service`.
2. Consumer: chỉ sửa consumer cần dùng behavior mới. Chạy
   `make verify SERVICE=<consumer>` cho mọi consumer import package đã đổi, kể cả
   consumer không sửa, vì generated code của chúng đã thay đổi.
3. Frontend consumer của REST: `make verify SERVICE=ecommerce-fe`.

Thứ tự rollout:

- `additive`: provider deploy trước, consumer bắt đầu dùng sau. Ví dụ chuỗi
  authorization: Identity, rồi Cart, rồi BFF.
- Gỡ field hoặc RPC: consumer ngừng dùng trước, provider gỡ sau và `reserved` số
  field.
- `semantic`: giới thiệu field/RPC/version mới, chuyển consumer, rồi mới gỡ bản cũ.

Không để correctness phụ thuộc vào việc mọi service deploy cùng lúc.

## 6. Review và báo cáo

Giao diff cho `boundary-reviewer`, kèm contract đã đổi và danh sách consumer.

Báo cáo gồm:

- contract, file nguồn và compatibility class;
- consumer đã sửa, consumer đã inspect nhưng giữ nguyên (kèm lý do);
- thứ tự rollout, cửa sổ tương thích và điều kiện gỡ bản cũ nếu có;
- commands `make verify` đã chạy cùng kết quả;
- quyết định high-risk đã được người dùng duyệt và rủi ro còn lại.
