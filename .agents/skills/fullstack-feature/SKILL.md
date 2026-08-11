---
name: fullstack-feature
description: Điều phối việc xây dựng hoặc thay đổi một feature end-to-end qua một hay nhiều application surface. Dùng khi cần làm rõ requirement và acceptance criteria, xác định phạm vi, chọn single-agent hay selective delegation, ổn định interface khi cần, triển khai kèm test, chạy verification, self-heal có giới hạn và báo cáo evidence. Skill chỉ quy định workflow; kiến trúc, technology, convention và lệnh kiểm tra phải lấy từ workspace và service-local instructions.
---

# Fullstack Feature

Điều phối feature end-to-end nhưng để repository quyết định cách hiện thực.

## Nguồn sự thật

Áp dụng theo thứ tự ưu tiên:

1. Yêu cầu và quyết định hiện tại của người dùng.
2. `AGENTS.md`, `CLAUDE.md`, steering docs và instructions áp dụng cho từng thư mục.
3. Spec, contract/interface và test hiện hữu của repository.
4. Pattern đang được dùng trong code gần nhất.
5. Workflow trong skill này.

Không đưa convention của một stack hoặc project khác vào feature. Không tạo thêm
manifest cấu hình, repo detector hoặc verification harness nếu repository không yêu
cầu chúng.

Khi cần xác định context hoặc repository có nhiều service, đọc
[`references/workspace-context.md`](references/workspace-context.md).

## Nguyên tắc thực thi

- Giữ primary agent chịu trách nhiệm end-to-end và cho phép primary trực tiếp viết
  code trong single-agent mode.
- Ưu tiên single-agent. Chỉ delegate khi phần việc độc lập đủ để giảm thời gian hoặc
  khi người dùng/workspace yêu cầu nhiều agent.
- Phân chia theo implementation surface và ownership thực tế, không mặc định theo
  `backend`/`frontend` hoặc theo technology.
- Chỉ chạy song song sau khi dependency và shared interface liên quan đã ổn định.
- Để mỗi assignee đọc instructions áp dụng cho surface của mình trước khi sửa code.
- Giữ test độc lập với implementation trong multi-agent mode khi có thể. Không cho
  implementation owner sửa hoặc xóa test chỉ để làm gate xanh.
- Xem kết quả command/harness của repository là bằng chứng pass/fail. Không tự suy
  diễn rằng gate đã pass khi chưa chạy.
- Giữ Vision QA là advisory, trừ khi workspace hoặc người dùng quy định nó là gate.

## Workflow

### 1. Chuẩn hóa yêu cầu

- Ghi lại outcome, hành vi mong đợi, ràng buộc và out-of-scope.
- Chuyển chúng thành acceptance criteria có thể kiểm chứng.
- Phân biệt yêu cầu trực tiếp với assumption hoặc derived requirement.
- Hỏi người dùng trước khi thực hiện một quyết định high-risk không thể suy ra an
  toàn, như thay đổi public interface gây breaking change, security boundary,
  destructive migration, dữ liệu nhạy cảm hoặc external side effect khó hoàn tác.
- Với quyết định low-risk và dễ đảo ngược, nêu assumption rồi tiếp tục.

### 2. Xác định affected surfaces

- Đọc root instructions và chỉ các local instructions có khả năng áp dụng.
- Xác định từng service, application, contract, datastore hoặc operational surface
  bị ảnh hưởng.
- Lập dependency order ngắn gọn và gán một owner cho mỗi surface.
- Không xem feature chạm nhiều service là boundary violation mặc định.

Ví dụ surface hợp lệ:

```text
order-service -> inventory-service -> web
```

### 3. Chọn execution strategy

Chọn single-agent khi thay đổi nhỏ, coupling cao, cần nhiều vòng chỉnh chéo, hoặc
chi phí handoff lớn hơn lợi ích song song hóa.

Chọn selective delegation khi có từ hai implementation unit độc lập, ownership rõ,
ít shared state và interface dùng chung đã ổn định. Mỗi task giao đi phải chứa:

- outcome và acceptance criteria liên quan;
- target surface và ownership boundary;
- interface/spec cần tuân thủ;
- instructions cần đọc;
- verification cần chạy;
- format báo cáo files changed, command và result.

Không delegate chỉ để mô phỏng một pipeline cố định. Không tạo agent theo technology
nếu generic assignee có thể đọc local rules và làm đúng surface.

Các agent chuyên biệt có thể dùng nếu hiện diện: `architect`, `surface-dev`, `qa`,
`vision-qa`.

### 4. Quyết định interface/contract

Không tạo contract trung gian chỉ để handoff trong một thay đổi nội bộ do một agent
thực hiện.

Cập nhật durable interface khi feature thay đổi public API, cross-service call,
event/schema hoặc boundary được nhiều implementation unit sử dụng. Dùng artifact mà
repository đã chọn, ví dụ OpenAPI, Protobuf, event schema hoặc typed interface; không
mặc định mọi boundary đều là một `openapi.yaml`.

Trong multi-agent mode, ổn định phần shared interface tối thiểu trước khi dispatch
các consumer/provider phụ thuộc vào nó. Nếu contract còn tranh chấp, giữ các phần
phụ thuộc tuần tự.

### 5. Lập test và triển khai

- Bám acceptance criteria và testing policy của repository.
- Thêm regression coverage cho mọi behavior path thay đổi theo local convention.
- Nếu dùng multi-agent, giao test/QA độc lập với implementation khi việc đó mang lại
  kiểm chứng thực sự; không bắt buộc tạo QA agent cho thay đổi nhỏ.
- Triển khai theo dependency order trong single-agent mode, hoặc song song chỉ với
  các unit đã độc lập.
- Không sửa file generated thủ công và không mở rộng ownership ngoài scope nếu chưa
  cập nhật kế hoạch.

### 6. Verify theo lớp

Chạy bằng commands do repository quy định:

1. Focused checks cho phần code vừa đổi.
2. Surface-level build, lint và test liên quan.
3. Contract/integration checks qua boundary đã thay đổi.
4. E2E hoặc repository-wide gates khi scope/risk yêu cầu.

Nếu repository có canonical CI script hoặc harness, dùng nó làm gate chính. Nếu
không có, chạy trực tiếp các command đã được instructions/build files xác nhận;
không dựng một universal harness trong feature task.

Khi fail, áp dụng [`references/self-heal-protocol.md`](references/self-heal-protocol.md).

### 7. Kết thúc bằng evidence

Chỉ tuyên bố hoàn thành khi các acceptance criteria trong scope đã có evidence.
Báo cáo ngắn gọn:

- surfaces và behavior đã thay đổi;
- contract/interface đã thêm hoặc cập nhật;
- commands thực tế đã chạy và kết quả;
- checks bị skip cùng lý do;
- assumptions hoặc high-risk decisions đã được chốt;
- blocker hoặc residual risk còn lại.

Nếu UI thay đổi, có thể đề xuất hoặc thực hiện Vision QA sau functional gates và ghi
rõ kết quả advisory hay blocking theo rules của workspace.
