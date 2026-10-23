# Delegation Prompts

Giao việc cho general-purpose subagent bằng prompt đầy đủ. Subagent không thấy hội
thoại, quyết định đã chốt hay file primary agent đã đọc; mọi thứ nó cần phải nằm
trong prompt. Ghép **khung chung** với đúng một **phần vai trò** bên dưới.

Chỉ dùng agent chuyên biệt khi repository cung cấp và việc cần quyền hạn riêng,
ví dụ reviewer chỉ đọc (`code-reviewer`, `boundary-reviewer`).

## Khung chung

```text
Role: <implementer | test owner | interface analyst | visual reviewer>

Repository: <đường dẫn root / worktree>
Đọc trước khi làm: root AGENTS.md; AGENTS.md gần nhất của <surface>;
steering: <chỉ các file docs/steering liên quan>; ADR: <nếu thay đổi chạm quyết định>.

Outcome: <kết quả cần đạt, một đến hai câu>
Acceptance criteria:
- AC-1: <điều kiện kiểm chứng được>
Out of scope: <những gì không làm>

Ownership:
- Được sửa: <paths>
- Chỉ đọc: <paths, gồm test/contract do owner khác giữ>
Interfaces phải tuân thủ: <proto/REST/event + đường dẫn file>
Quyết định đã chốt: <assumption, lựa chọn đã được người dùng duyệt>

Verification: <exact commands, ví dụ `make verify SERVICE=cart-service`>

Rules:
- Chỉ sửa trong ownership. Nếu cần sửa ngoài ownership, contract hoặc test của
  owner khác: dừng phần phụ thuộc, tiếp tục phần độc lập, báo conflict kèm lý do.
- Không xóa, skip hoặc hạ assertion để làm gate xanh. Không sửa generated output
  bằng tay.
- Không tự quyết breaking contract, auth/security boundary, destructive migration,
  dữ liệu nhạy cảm hoặc side effect khó hoàn tác; trả về lựa chọn và impact.
- Không báo hoàn thành khi chưa chạy verification. Check nào không chạy được phải
  nói rõ lý do.

Report:
- files đã sửa;
- AC đã đáp ứng và evidence tương ứng;
- commands đã chạy cùng kết quả (pass/fail/skip);
- conflicts, blockers, assumptions và residual risk.
```

## Phần vai trò

### Implementer

```text
- Inspect code lân cận, tests và build files của surface để theo pattern đang
  dùng; không mang convention từ surface khác sang.
- Chạy focused test trước khi sửa để ghi baseline, tách lỗi có sẵn khỏi regression.
- Behavior change: viết test fail trước, xác nhận fail đúng lý do, rồi mới
  implement (red → green → refactor).
- UI: xử lý loading, empty, success và error states; UI gating không thay thế
  authorization phía server.
- Thêm regression test cho mọi behavior path thay đổi, trừ test đã giao cho test
  owner.
```

### Test owner

```text
- Map mỗi test hoặc nhóm test tới một AC, interface behavior hoặc regression path.
- Chọn level thấp nhất đủ tin cậy theo docs/steering/testing.md: unit, focused
  framework, integration, contract, rồi mới E2E.
- Không sửa application code hoặc contract. Nếu spec/contract có vẻ sai, báo
  conflict kèm reproduction.
- Test viết trước implementation phải fail vì behavior chưa tồn tại, không vì
  syntax, fixture hay environment. Ghi rõ expected-red hoặc unexpected-pass.
- Dùng fixture deterministic và data cô lập; không dùng shared mutable data.
- Report thêm: mapping test → AC, coverage gaps.
```

### Interface analyst

```text
- Chuẩn hóa outcome, constraints, out-of-scope thành AC kiểm chứng được; tách
  requirement trực tiếp, assumption và derived requirement.
- Xác định affected surfaces theo docs/steering/change-impact.md
  (DIRECT/CONTRACT/DEPENDENT/INFRA/TEST) và owner của từng boundary.
- Chỉ thêm hoặc đổi durable interface khi có boundary lâu dài; không tạo contract
  trung gian chỉ để handoff.
- Với contract change, làm theo skill `contract-change`.
- Khi test và implementation mâu thuẫn: đối chiếu AC và interface, kết luận bên
  nào sai; không hạ AC để làm xanh gate.
- Report thêm: mapping AC → surfaces/interfaces, quyết định còn mở cần người dùng.
```

### Visual reviewer

```text
- Chỉ đọc: không sửa code, test, contract hoặc spec.
- Chỉ kết luận từ screenshot/recording được cung cấp; thiếu evidence thì báo
  evidence gap, không đoán giao diện từ code.
- Kiểm tra: thiếu state/thành phần, layout vỡ/overlap/overflow, text khó đọc hoặc
  placeholder, loading/empty/error không rõ, responsive sai, lệch design system.
- Tách lỗi quan sát chắc chắn khỏi góp ý thẩm mỹ. Mặc định advisory, chỉ blocking
  khi task nói rõ.
- Verdict: OK | OK with notes | Needs work | Insufficient evidence.
```
