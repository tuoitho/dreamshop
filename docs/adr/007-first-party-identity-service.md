# ADR-007: First-party Identity Service và token/session security

## Status

Accepted

## Context

E-commerce cần đăng ký, đăng nhập, profile và quyền cơ bản. Dùng Keycloak giảm rủi ro nhưng không đáp ứng mục tiêu học tự xây identity/auth. Ngược lại, tự xây một OAuth/OIDC provider tổng quát, password reset mail, MFA, social login hoặc third-party client vượt xa phạm vi 8–12 tuần và mang rủi ro bảo mật cao.

## Decision

Tạo một Identity Service bằng Java + Spring Boot/Spring Security, chỉ phục vụ first-party clients của dự án. Service sở hữu user/profile, role, password credential, session và refresh token; public qua Kong tại `/auth/*` và `/users/me`.

Password được hash bằng Argon2id (bcrypt là fallback có ghi rõ lý do). Access JWT ký bất đối xứng, có `sub`, role, issuer, audience, expiry, token ID và `kid`; JWKS public phục vụ verify/key rotation. Refresh token lưu hash, rotate mỗi lần refresh, có token family/revocation; reuse token revoked revoke toàn family. BFF/Identity verify issuer, audience và signature trước khi tạo caller context; Order vẫn làm authorization ownership.

MVP không implement generic OIDC/OAuth authorization server, MFA, email verification/reset, social login hay third-party client integration.

## Consequences

- Được: học trực tiếp security boundary quan trọng — password hashing, key management, JWT validation, refresh rotation, revocation và auth tests.
- Được: data/contract của user profile nằm trong service do dự án sở hữu; không phụ thuộc IdP ngoài.
- Mất: tăng một service Java, database migration, key/session lifecycle và security test burden.
- Trade-off: phạm vi auth bị cố ý giới hạn cho first-party e-commerce. Khi cần OIDC/MFA/social login thật, xem xét thay thế hoặc federation với IdP chuyên dụng thay vì mở rộng tùy tiện service này.
