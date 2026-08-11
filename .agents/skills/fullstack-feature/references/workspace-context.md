# Workspace Context

Đọc reference này khi bắt đầu một feature trong repository chưa quen thuộc hoặc khi
feature có khả năng chạm nhiều application surface.

## Discovery tối thiểu

1. Đọc root `AGENTS.md` và steering docs được root instructions trỏ tới.
2. Inspect cấu trúc đủ để xác định các surface có khả năng liên quan.
3. Với mỗi surface dự kiến chạm, đọc instructions gần nhất áp dụng cho thư mục đó.
4. Kiểm tra code, tests, contracts và build files hiện hữu để xác nhận pattern đang
   được dùng.
5. Ghi lại affected surfaces, dependency, ownership, interface và verification
   commands đã được repository định nghĩa.

Chỉ đọc sâu vào surface có liên quan. Không load toàn bộ repository nếu scope đã rõ.

## Source of truth cho verification

Ưu tiên theo thứ tự:

1. Command được workspace/service-local instructions chỉ định.
2. Canonical CI, Makefile, task runner hoặc script verify của repository.
3. Package/build tool configuration hiện hữu.
4. Command focused tương đương đã được codebase sử dụng.

Nếu các nguồn mâu thuẫn, dùng instruction có scope gần nhất hoặc dừng để làm rõ khi
sự khác biệt có thể thay đổi kết quả gate.

## Không tự scaffold hoặc detect quá mức

- Không tạo cấu trúc `backend/`, `frontend/`, `contracts/` hay `tests/` mặc định.
- Không suy luận architecture chỉ từ `pom.xml`, `go.mod` hoặc `package.json`.
- Không tạo `project.json` để lặp lại thông tin đã có trong workspace instructions.
- Không viết repo-discovery script để phân loại framework/role cho một feature đơn
  lẻ.
- Không thêm harness chỉ để chuẩn hóa output nếu commands hiện hữu đã đủ làm gate.

Chỉ thêm harness-specific configuration khi repository thực sự thiếu thông tin cần
cho một cơ chế deterministic được yêu cầu. Giữ nó nhỏ và không biến nó thành nguồn
sự thật thứ hai cho architecture hoặc technology.

## Readiness

Trước implementation, xác nhận những dependency thực sự cần cho feature có thể chạy:

- toolchain/commands cần thiết tồn tại;
- test environment không trỏ tới production data;
- required service hoặc infrastructure có cách khởi động được repository mô tả;
- durable contracts đang ở trạng thái hợp lệ nếu feature phụ thuộc vào chúng.

Không bắt toàn bộ monorepo phải xanh trước khi bắt đầu nếu failure nằm ngoài scope và
đã tồn tại sẵn. Ghi baseline failure để phân biệt regression do feature tạo ra.
