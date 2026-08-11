# Self-Heal Protocol

Áp dụng khi một verification gate thất bại sau thay đổi feature.

## 1. Thu thập failure record

Ghi lại:

- command/gate đã chạy;
- surface và owner liên quan;
- exit code hoặc trạng thái;
- phần log nhỏ nhất đủ chẩn đoán, tối đa khoảng 100 dòng;
- failure có tồn tại ở baseline hay do thay đổi mới gây ra;
- số lần sửa cho cùng root cause.

Không gửi toàn bộ log cho assignee nếu một trích đoạn đã đủ tái hiện lỗi.

## 2. Phân loại trước khi route

| Failure class | Route mặc định |
|---|---|
| Compile, lint, unit hoặc component test | Owner của surface chứa lỗi |
| Contract/schema invalid | Owner của durable interface |
| Provider không khớp interface | Owner của provider |
| Consumer không khớp interface | Owner của consumer |
| Integration/E2E assertion sai behavior | Owner của implementation gây sai behavior |
| Test compile/setup/fixture lỗi | Test/QA owner |
| Spec, acceptance criteria hoặc contract mâu thuẫn | Primary agent; hỏi người dùng nếu là quyết định high-risk |
| Toolchain, credential, environment hoặc infrastructure thiếu | Primary agent; không ép implementation owner sửa product code |
| Flaky/timeout chưa rõ owner | Primary agent tái hiện và thu hẹp trước khi route |

Route theo root cause, không theo tên gate. Một E2E failure có thể thuộc UI, API,
contract, fixture hoặc environment.

## 3. Giao vòng sửa

Dùng payload tương đương:

```text
FAILED: <command/gate> — attempt <n>/<limit>
Surface: <surface>
Observed: <concise failure>
Relevant log: <bounded excerpt>

Fix the root cause within your ownership, follow applicable local instructions,
run the focused verification, and report changed files plus exact results.
Do not weaken, delete, or rewrite tests merely to make the gate pass. If the test,
spec, or contract is wrong, return the conflict to the primary agent.
```

Sau khi owner báo sửa xong, primary agent phải chạy lại focused gate. Chỉ chạy lại
gate rộng hơn sau khi focused gate đã pass.

## 4. Bảo vệ test integrity

- Không xóa, skip hoặc hạ assertion để làm xanh.
- Implementation owner không tự thay đổi expected behavior do QA/test owner viết
  nếu acceptance criteria và contract chưa đổi.
- Cho phép sửa test khi test sai, nhưng phải route về test owner hoặc được primary
  xác nhận bằng spec/contract; ghi rõ lý do trong evidence.
- Khi durable interface thay đổi có chủ ý, cập nhật provider, consumer và tests theo
  cùng một quyết định; không để mỗi owner tự diễn giải khác nhau.

## 5. Giới hạn và dừng

Mặc định tối đa 3 vòng cho cùng root cause trên cùng surface, trừ khi workspace quy
định giới hạn thấp hơn. Không reset bộ đếm chỉ vì lỗi biểu hiện ở gate khác.

Dừng self-heal và báo người dùng khi:

- đã hết retry budget;
- cần authority, credential hoặc external-state change mới;
- phải chọn giữa các behavior nghiệp vụ không thể suy ra an toàn;
- fix yêu cầu mở rộng scope hoặc ownership đáng kể;
- cùng interface bị đảo qua lại mà không hội tụ.

Báo blocker kèm command, failure cô đọng, attempts đã thử, root cause tốt nhất hiện
có và quyết định cụ thể cần từ người dùng.
