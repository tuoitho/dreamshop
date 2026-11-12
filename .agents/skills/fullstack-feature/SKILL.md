---
name: fullstack-feature
description: Nghĩa vụ riêng của repository này cho mọi feature hoặc behavior change, áp dụng chồng lên bất kỳ workflow spec/plan/thực thi nào (superpowers brainstorming → writing-plans → subagent-driven-development hoặc executing-plans, hay làm trực tiếp). Dùng khi viết spec hoặc plan cho thay đổi chạm service, frontend, contract hoặc datastore; khi giao task cho subagent; trước khi tuyên bố task hoặc feature đã xong; và trước final review, handoff, PR hoặc merge. Quy định surface map và ownership, gate `make verify`, review bắt buộc bằng subagent nạp skill `reviewing-code` và `reviewing-service-boundaries` của repository, và evidence report.
---

# Fullstack Feature

Yêu cầu, spec, plan, TDD và cách thực thi từng task do workflow đang dùng quyết
định: chuỗi superpowers khi có, hoặc mục Development Process của `AGENTS.md` khi
làm trực tiếp. Skill này thêm các nghĩa vụ mà workflow chung không biết về
repository này. Bước tương ứng của workflow chung không thay thế chúng.

| Thời điểm | Nghĩa vụ |
|---|---|
| Viết spec hoặc plan | [1. Surface map](#1-surface-map) |
| Giao task cho subagent | [2. Task context](#2-task-context) |
| Trước khi tuyên bố task hoặc feature đã xong | [3. Gate theo lớp](#3-gate-theo-lớp) |
| Sau task cuối, trước handoff, PR hoặc merge | [4. Review của repository](#4-review-của-repository) |
| Kết thúc | [5. Evidence](#5-evidence) |

Khi các nguồn mâu thuẫn: quyết định của người dùng, rồi `AGENTS.md` và steering
docs, rồi skill này, rồi workflow chung.

## 1. Surface map

Spec hoặc plan của feature phải chứa:

- Từng service, frontend, contract, datastore và routing surface bị ảnh hưởng,
  phân loại theo [`docs/steering/change-impact.md`](../../../docs/steering/change-impact.md),
  mỗi surface một owner.
- Dependency order giữa các surface: contract, rồi provider, rồi consumer, rồi
  frontend. Ví dụ: `contracts -> order-service -> bff-service -> admin-fe`.
- Với mỗi task: surface sở hữu, path được sửa và gate
  `make verify SERVICE=<surface>` của nó.

Thay đổi protobuf, gRPC, event hoặc REST contract dùng chung là task riêng, làm
theo skill `contract-change`, và phải xong trước khi bắt đầu task provider hoặc
consumer phụ thuộc vào nó. Feature chạm nhiều service không phải boundary
violation mặc định; import chéo hoặc truy cập database của service khác thì có.

Thay đổi làm trực tiếp không qua spec hoặc plan vẫn cần acceptance criteria kiểm
chứng được ghi lại trước khi sửa code, vì reviewer ở mục 4 nhận AC làm đầu vào.

## 2. Task context

Prompt template của workflow chung không mang thông tin repository. Khi giao
task cho subagent, thêm khối này vào prompt:

```text
Đọc trước khi làm: root AGENTS.md; AGENTS.md gần nhất của <surface>;
steering: <chỉ các file docs/steering liên quan>; ADR: <nếu task chạm quyết định đã duyệt>.
Ownership: được sửa <paths>; chỉ đọc <paths, gồm contract và test của owner khác>.
Interfaces phải tuân thủ: <proto/REST/event + đường dẫn file>.
Gate: make verify SERVICE=<surface>

- Cần sửa ngoài ownership, đổi contract hoặc test của owner khác: dừng phần phụ
  thuộc và báo lại kèm lý do.
- Không sửa generated output bằng tay. Không xóa, skip hoặc hạ assertion để làm
  gate xanh.
- Báo cáo commands đã chạy kèm exit code; check nào không chạy được phải nêu lý do.
```

## 3. Gate theo lớp

Commands lấy từ [`docs/instructions/build-and-test.md`](../../../docs/instructions/build-and-test.md);
lựa chọn mức test theo [`docs/steering/testing.md`](../../../docs/steering/testing.md).

1. Focused checks cho phần code vừa đổi, trong từng task.
2. `make verify SERVICE=<surface>` cho mỗi surface đã chạm.
3. Contract checks khi boundary thay đổi, theo skill `contract-change`.
4. E2E hoặc repository-wide checks khi scope hoặc risk yêu cầu.

"Tests pass" ở cuối một task chỉ là lớp 1. Lớp 2 phải chạy trên trạng thái code
cuối cùng, sau commit cuối của feature và trước review ở mục 4. Kết quả gate là
exit code của command do chính agent điều phối chạy; số test trong báo cáo của
subagent không phải kết quả gate.

Trước khi sửa code, ghi lại failure đã tồn tại ở baseline để tách khỏi
regression. Khi gate fail, áp dụng
[`references/self-heal-protocol.md`](references/self-heal-protocol.md).

## 4. Review của repository

Final whole-branch review của workflow chung được thực hiện bằng các skill review
của repository, không phải reviewer generic hay review template của workflow đó:
reviewer generic không kiểm tra service boundary và không áp dụng Finding
Admission Gate. Review theo từng task của workflow chung giữ nguyên và không
thay cho bước này.

Mỗi review là một subagent mới chưa tham gia implementation, nhận prompt dựng từ
[`references/review-prompts.md`](references/review-prompts.md) và nạp đúng một
skill review:

- `reviewing-code`: mọi thay đổi.
- `reviewing-service-boundaries`: thêm khi thay đổi chạm contract, nhiều
  service, persistence, Kong/BFF routing hoặc communication path.

Hai broad review độc lập với nhau nên dispatch cùng lúc.

Xử lý kết quả:

1. Sửa mọi finding `blocking` và AC chưa đạt; mỗi bản sửa đi qua test fail trước.
   Finding `major` thì sửa hoặc ghi lý do kỹ thuật không sửa; finding `minor`
   và `non-blocking` không sửa thì đưa vào evidence report.
2. Chạy lại focused check và `make verify` của surface bị sửa.
3. Gửi scoped re-review bằng prompt mẫu tương ứng, nạp đúng skill đã tạo
   finding: danh sách finding chưa đóng, diff của bản sửa và kết quả chạy lại.
4. Tối đa 3 vòng cho mỗi skill review trên cùng phạm vi; sau đó dừng và báo người
   dùng. Giới hạn này thay cho giới hạn vòng final review của workflow chung.

Finding bị bác bỏ phải có lý do kỹ thuật trong evidence report. Visual review cho
thay đổi UI là advisory, trừ khi người dùng hoặc workspace quy định nó là gate.

## 5. Evidence

Khi workflow có ledger (`.superpowers/sdd/<plan>/progress.md`), ghi gate và
review vào đó ngay khi có kết quả:

```text
Gate: make verify SERVICE=order-service → exit 0
Final: reviewing-code round 1 — CHANGES REQUESTED (2 blocking)
Final: reviewing-code round 2 — APPROVE
Final: reviewing-service-boundaries round 1 — PASS
```

Ledger bị xóa khi plan kết thúc, nên báo cáo cuối và mô tả PR phải tự mang đủ:

- surfaces và behavior đã thay đổi;
- contract hoặc interface đã thêm hoặc cập nhật;
- commands đã chạy kèm exit code;
- checks bị skip cùng lý do;
- verdict của từng skill review, số vòng, findings còn mở và findings bị bác
  bỏ kèm lý do; ghi `reviewing-service-boundaries: không áp dụng — <lý do>`
  khi thay đổi không thuộc phạm vi của nó;
- assumptions và high-risk decisions đã được chốt;
- blocker hoặc residual risk còn lại.

Chỉ tuyên bố hoàn thành khi mọi AC trong scope có evidence, lớp gate 2 đã chạy
trên trạng thái cuối và verdict review đã được ghi lại.
