# ADR-007: First-party Identity Service và token/session security

## Status

Accepted

## Context

E-commerce cần đăng ký, đăng nhập, profile và quyền cơ bản. Dùng Keycloak giảm rủi ro nhưng không đáp ứng mục tiêu học tự xây identity/auth. Ngược lại, tự xây một OAuth/OIDC provider tổng quát, password reset mail, MFA, social login hoặc third-party client vượt xa phạm vi 8–12 tuần và mang rủi ro bảo mật cao.

## Decision

Tạo một Identity Service bằng Java + Spring Boot/Spring Security, chỉ phục vụ first-party clients của dự án. Service sở hữu user/profile, sổ địa chỉ giao hàng, địa giới hành chính, phân quyền RBAC, password credential, session và refresh token; public qua Kong tại `/api/v1/auth`, `/api/v1/users`, `/api/v1/locations`, `/api/v1/admin/users` và `/api/v1/admin/rbac`, đồng thời phục vụ đánh giá phân quyền nội bộ qua private gRPC (`AuthorizationService` trên cổng `50054`).

Password được hash bằng Argon2id. Access JWT ký bất đối xứng (RS256), có `sub`, `roles`, `iss`, `aud`, `exp`, `jti` và `kid`; JWKS công khai phục vụ verify/key rotation. Refresh token lưu hash, rotate mỗi lần refresh, có token family/revocation; reuse token đã rotate sẽ revoke toàn family. Plugin `caller-context` trên Kong xác thực chữ ký RS256, `iss`, `aud` và `exp` rồi gắn `X-User-Id` / `X-User-Roles` cho các route được bảo vệ (còn các route public strip sẵn hai header này); các thao tác nhạy cảm về quyền thời gian thực (như `cart.item.add`, `product.delete`) được Catalog/Cart kiểm tra trực tiếp qua private gRPC tới Identity.

MVP không implement generic OIDC/OAuth authorization server, MFA, email verification/reset, social login hay third-party client integration.

## Consequences

- Được: học trực tiếp security boundary quan trọng — password hashing, key management, JWT validation, refresh rotation, revocation và auth tests.
- Được: data/contract của user profile nằm trong service do dự án sở hữu; không phụ thuộc IdP ngoài.
- Mất: tăng một service Java, database migration, key/session lifecycle và security test burden.
- Trade-off: phạm vi auth bị cố ý giới hạn cho first-party e-commerce. Khi cần OIDC/MFA/social login thật, xem xét thay thế hoặc federation với IdP chuyên dụng thay vì mở rộng tùy tiện service này.
