# Review Prompts

Prompt mẫu để giao review cho subagent. Reviewer là một subagent general-purpose
mới, chưa tham gia implementation; toàn bộ hướng dẫn review nằm trong skill mà
prompt yêu cầu nạp, prompt chỉ mang đầu vào của lần review đó.

| Lần review | Skill reviewer nạp | Model |
|---|---|---|
| Broad review, mọi thay đổi | `reviewing-code` | mạnh nhất hiện có |
| Broad review, thay đổi chạm contract, nhiều service, persistence, routing hoặc communication path | `reviewing-service-boundaries` | mạnh nhất hiện có |
| Scoped re-review | skill của reviewer đã tạo finding | tầm trung trở lên |

Quy tắc:

- Mỗi lần review là một subagent mới; không tái dùng reviewer của vòng trước.
- Hai broad review độc lập với nhau nên dispatch cùng lúc.
- Ghi rõ model khi dispatch; bỏ trống thì subagent thừa hưởng model của phiên.
- Không đưa lời biện minh thiết kế hoặc gợi ý hạ mức độ lỗi vào prompt. Nếu
  prompt đang viết chứa "do not flag", "at most minor" hay "plan đã chọn", đó
  là đang chấm hộ reviewer; xóa nó.
- Base là commit nhánh bắt đầu (`git merge-base main HEAD`), không phải
  `HEAD~1`.

## Broad review

Thay `<skill>` bằng `reviewing-code` hoặc `reviewing-service-boundaries`.

```text
description: "<skill> — <tên feature>"
prompt: |
  Nạp skill `<skill>` và làm đúng theo skill đó cho lần broad review này.
  Bạn không tham gia implementation. Chỉ đọc: không sửa file, không đổi
  index, HEAD hay branch, không commit.

  ## Thay đổi
  <outcome trong 1-3 câu; các surface đã chạm>

  ## Acceptance criteria và out-of-scope
  <danh sách AC, hoặc đường dẫn spec/plan cùng mục chứa AC>

  ## Phạm vi review
  Range: --from <BASE_SHA> --to <HEAD_SHA>

  ## Contract đã đổi và consumer đã biết
  <proto/REST/event + đường dẫn, consumer; hoặc "Không có">

  ## Commands đã chạy
  - `<command>` → exit <code>

  Trả kết quả đúng định dạng "Broad Review" trong skill.
```

## Scoped re-review

```text
description: "<skill> re-review vòng <R> — <tên feature>"
prompt: |
  Nạp skill `<skill>` và làm đúng phần "Scoped Re-review Mode" của skill đó.
  Đây không phải broad review: chỉ chấm các finding dưới đây và kiểm tra
  phần diff của bản sửa. Chỉ đọc: không sửa file, không đổi index, HEAD hay
  branch, không commit.

  ## Finding chưa đóng từ vòng trước
  <dán nguyên văn từng finding: mức, file:line, nội dung>

  ## Bản sửa
  Range: --from <SHA trước bản sửa> --to <HEAD_SHA>

  ## Kết quả chạy lại
  - `<focused test>` → exit <code>
  - `make verify SERVICE=<surface>` → exit <code>

  Trả kết quả đúng định dạng "Scoped Re-review" trong skill.
```
