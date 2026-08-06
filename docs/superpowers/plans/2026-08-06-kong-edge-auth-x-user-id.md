# Kong Edge Authentication and x-user-id Propagation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Kong verifies RS256 JWTs at the edge using Identity's JWKS and forwards the verified caller as `x-user-id`/`x-user-roles`; Identity exposes `GET /users/me` that consumes those headers instead of self-verifying tokens.

**Architecture:** A custom Kong Lua plugin (`caller-context`) caches the JWKS, verifies signature/`iss`/`aud`/`exp`, strips client-supplied identity headers and injects verified ones. The Kong route for `/api/v1/users` is split from the public `/api/v1/auth` route. Identity reads `x-user-id`/`x-user-roles` in a Spring Security filter and serves the caller's profile from PostgreSQL.

**Tech Stack:** Kong 3.8 (Lua plugin + `lua-resty-openssl`), Docker Compose, Spring Boot 4.1 / Java 21 / Spring Security, PostgreSQL + Testcontainers, PowerShell smoke scripts, Make.

**Spec:** `docs/superpowers/specs/2026-08-06-kong-edge-auth-and-x-user-id-design.md`

---

## Repository-state warning

The git index currently contains 173 pre-existing staged files that do not belong to this work. **Never use a bare `git add .` or `git add -A`, and never run `git commit` without an explicit pathspec.** In every commit step below, commit with an explicit path list:

```bash
git add <the files from this task>
git commit -m "<message>" -- <the files from this task>
```

`git commit -- <paths>` commits only those paths and leaves the other staged files untouched.

---

## File structure

| File | Responsibility |
|---|---|
| `deployments/kong/plugins/caller-context/schema.lua` | Kong plugin schema/config fields |
| `deployments/kong/plugins/caller-context/handler.lua` | JWT verify + header inject/strip |
| `deployments/kong/Dockerfile` | Kong image with the custom plugin loaded |
| `deployments/kong/kong.yaml` | Route split + plugin configuration |
| `deployments/docker/compose.foundation.yml` | Build Kong from the Dockerfile |
| `services/identity-service/.../security/CallerContext.java` | Caller value object |
| `services/identity-service/.../security/CallerContextPrincipal.java` | Spring `Authentication` for the caller |
| `services/identity-service/.../security/CallerContextFilter.java` | Reads headers into the security context |
| `services/identity-service/.../security/SecurityConfiguration.java` | Policy + filter wiring (modified) |
| `services/identity-service/.../user/domain/UserNotFoundException.java` | 404 signal |
| `services/identity-service/.../user/application/CurrentUser.java` | Application result record |
| `services/identity-service/.../user/application/MeService.java` | Loads the caller profile |
| `services/identity-service/.../user/api/MeResponse.java` | Response DTO |
| `services/identity-service/.../user/api/MeController.java` | `GET /users/me` |
| `services/identity-service/.../user/persistence/IdentityUserRepository.java` | New `findWithProfileAndRolesById` (modified) |
| `services/identity-service/.../common/error/GlobalExceptionHandler.java` | `USER_NOT_FOUND` mapping (modified) |
| `services/identity-service/src/test/.../user/api/MeApiTest.java` | MockMvc tests for `/users/me` |
| `scripts/verify-edge-auth.ps1` | End-to-end smoke through Kong |
| `Makefile` | `verify-edge-auth` target (modified) |

Base package: `com.ecommerce.identity`.

---

## Task 1: Kong `caller-context` plugin + custom Kong image

**Files:**
- Create: `deployments/kong/plugins/caller-context/schema.lua`
- Create: `deployments/kong/plugins/caller-context/handler.lua`
- Create: `deployments/kong/Dockerfile`
- Modify: `deployments/docker/compose.foundation.yml:54-76`

- [ ] **Step 1: Create the plugin schema**

`deployments/kong/plugins/caller-context/schema.lua`:

```lua
return {
  name = "caller-context",
  fields = {
    { config = {
        type = "record",
        fields = {
          { jwks_uri = { type = "string", required = true } },
          { issuer = { type = "string", required = true } },
          { audience = { type = "string", required = true } },
          { user_id_header = { type = "string", default = "x-user-id" } },
          { roles_header = { type = "string", default = "x-user-roles" } },
          { cache_ttl_seconds = { type = "integer", default = 300 } },
          { clock_skew_seconds = { type = "integer", default = 0 } },
        },
      } },
    },
  }
```

> **Schema gotchas (discovered during Task 7 smoke testing):**
> - The last line must be `}` (no trailing comma). `return { ... },` is a syntax error and Kong fails to load the schema.
> - Do not put `required = { "jwks_uri", ... }` inside the `config` record. Kong validates that key as the field-definition property `required` (a boolean), not as a record constraint. Required config fields are expressed with `required = true` on each field, as done above.

- [ ] **Step 2: Create the plugin handler**

`deployments/kong/plugins/caller-context/handler.lua`:

```lua
local cjson = require "cjson"
local pkey = require "resty.openssl.pkey"
local resty_http = require "resty.http"

local CallerContextHandler = {
  VERSION = "1.0.0",
  PRIORITY = 1001,
}

local CACHE_KEY_PREFIX = "caller-context:jwks:"

local function b64url_decode(value)
  local b64 = value:gsub("-", "+"):gsub("_", "/")
  local pad = (4 - #b64 % 4) % 4
  return ngx.decode_base64(b64 .. string.rep("=", pad))
end

local function decode_json_part(part)
  local raw = b64url_decode(part)
  if not raw then
    return nil
  end
  local ok, value = pcall(cjson.decode, raw)
  if not ok or type(value) ~= "table" then
    return nil
  end
  return value
end

local function aud_contains(aud, expected)
  if type(aud) == "string" then
    return aud == expected
  end
  if type(aud) == "table" then
    for _, item in ipairs(aud) do
      if item == expected then
        return true
      end
    end
  end
  return false
end

local function verify_claims(claims, conf)
  if claims.iss ~= conf.issuer then
    return false, "issuer mismatch"
  end
  if not aud_contains(claims.aud, conf.audience) then
    return false, "audience mismatch"
  end
  local now = ngx.time()
  if type(claims.exp) ~= "number" or now > claims.exp + conf.clock_skew_seconds then
    return false, "token expired"
  end
  return true
end

local function fetch_jwks(jwks_uri)
  local httpc = resty_http.new()
  httpc:set_timeout(2000)
  local res, err = httpc:request_uri(jwks_uri, { method = "GET" })
  if not res then
    return nil, err
  end
  if res.status ~= 200 then
    return nil, "JWKS endpoint returned " .. res.status
  end
  local ok, decoded = pcall(cjson.decode, res.body)
  if not ok or type(decoded) ~= "table" or type(decoded.keys) ~= "table" then
    return nil, "invalid JWKS body"
  end
  return decoded
end

local function load_jwks(jwks_uri)
  local jwks, err = fetch_jwks(jwks_uri)
  if not jwks then
    kong.log.warn("caller-context: JWKS fetch failed: ", err)
    return false
  end
  return jwks
end

local function find_key(jwks, kid)
  for _, key in ipairs(jwks.keys) do
    if key.kid == kid then
      return key
    end
  end
end

local function verify_signature(header, header_part, payload_part, signature_part, jwk)
  if header.alg ~= "RS256" or type(jwk.n) ~= "string" or type(jwk.e) ~= "string" then
    return false
  end
  local pk, err = pkey.new(cjson.encode({ kty = "RSA", n = jwk.n, e = jwk.e }), { format = "JWK" })
  if not pk then
    kong.log.warn("caller-context: failed to load JWK: ", err)
    return false
  end
  local signature = b64url_decode(signature_part)
  if not signature then
    return false
  end
  return pk:verify(signature, header_part .. "." .. payload_part, "sha256")
end

function CallerContextHandler:access(conf)
  kong.service.request.clear_header(conf.user_id_header)
  kong.service.request.clear_header(conf.roles_header)

  local auth_header = kong.request.get_header("Authorization")
  if type(auth_header) ~= "string" then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Missing or invalid access token" })
  end
  local token = auth_header:match("^[Bb]earer%s+(%S+)$")
  if not token then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Missing or invalid access token" })
  end

  local header_part, payload_part, signature_part = token:match("^([^.]+)%.([^.]+)%.([^.]+)$")
  if not header_part or not payload_part or not signature_part then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Missing or invalid access token" })
  end

  local header = decode_json_part(header_part)
  local claims = decode_json_part(payload_part)
  if not header or not claims or not header.kid then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Invalid or expired access token" })
  end

  local cache_key = CACHE_KEY_PREFIX .. conf.jwks_uri
  local jwks = kong.cache:get(cache_key, { ttl = conf.cache_ttl_seconds }, load_jwks, conf.jwks_uri)
  if not jwks then
    return kong.response.exit(503, { code = "AUTHENTICATION_UNAVAILABLE", message = "Authentication is temporarily unavailable" })
  end

  local jwk = find_key(jwks, header.kid)
  if not jwk then
    kong.cache:invalidate(cache_key)
    jwks = load_jwks(conf.jwks_uri)
    if not jwks then
      return kong.response.exit(503, { code = "AUTHENTICATION_UNAVAILABLE", message = "Authentication is temporarily unavailable" })
    end
    jwk = find_key(jwks, header.kid)
  end
  if not jwk then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Invalid or expired access token" })
  end

  local claims_ok, claims_err = verify_claims(claims, conf)
  if not claims_ok then
    kong.log.debug("caller-context: claims rejected: ", claims_err)
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Invalid or expired access token" })
  end
  if not verify_signature(header, header_part, payload_part, signature_part, jwk) then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Invalid or expired access token" })
  end
  if type(claims.sub) ~= "string" or claims.sub == "" then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Invalid or expired access token" })
  end

  kong.service.request.set_header(conf.user_id_header, claims.sub)
  if type(claims.roles) == "table" then
    kong.service.request.set_header(conf.roles_header, table.concat(claims.roles, ","))
  end
end

return CallerContextHandler
```

- [ ] **Step 3: Create the Kong Dockerfile**

`deployments/kong/Dockerfile`:

```dockerfile
FROM kong:3.8-ubuntu
COPY plugins/caller-context /usr/local/share/lua/5.1/kong/plugins/caller-context
ENV KONG_PLUGINS=bundled,caller-context
```

- [ ] **Step 4: Switch the Kong service to build from the Dockerfile**

In `deployments/docker/compose.foundation.yml`, replace the `kong` service block's image line with a build:

```yaml
  kong:
    build:
      context: ../kong
    container_name: ecommerce-kong
```

The rest of the `kong` block (environment, volumes, ports, networks, depends_on) stays unchanged. The relative path `../kong` is resolved against the compose file directory (`deployments/docker/`), i.e. `deployments/kong/`.

- [ ] **Step 5: Verify the Kong image builds**

Run (from repo root):

```bash
docker compose -f deployments/docker/compose.foundation.yml build kong
```

Expected: `docker compose build` completes; the log ends with `Successfully tagged ...kong...` or `nginx ...` image created.

- [ ] **Step 6: Commit**

```bash
git add deployments/kong/plugins/caller-context/schema.lua deployments/kong/plugins/caller-context/handler.lua deployments/kong/Dockerfile deployments/docker/compose.foundation.yml
git commit -m "feat(kong): add caller-context JWKS verification plugin and custom image" -- deployments/kong/plugins/caller-context/schema.lua deployments/kong/plugins/caller-context/handler.lua deployments/kong/Dockerfile deployments/docker/compose.foundation.yml
```

---

## Task 2: Kong route split and plugin configuration

**Files:**
- Modify: `deployments/kong/kong.yaml:3-15`

- [ ] **Step 1: Split the identity route and attach the plugin**

Replace the `services` block in `deployments/kong/kong.yaml` (lines 3-15) with:

```yaml
services:
  - name: identity-service
    url: http://identity-service:8080
    routes:
      - name: identity-auth-public
        paths: ["/api/v1/auth"]
        strip_path: true
      - name: identity-users-protected
        paths: ["/api/v1"]
        strip_path: true
        plugins:
          - name: caller-context
            config:
              jwks_uri: http://identity-service:8080/.well-known/jwks.json
              issuer: http://identity-service:8080
              audience: ecommerce-api
  - name: bff-service
    url: http://bff-service:8080
    routes:
      - name: storefront-api
        paths: ["/api/v1/storefront"]
        strip_path: true
```

> **Why `/api/v1` and not `/api/v1/users`:** Kong's `strip_path` removes the *entire* matched prefix, so a route at `/api/v1/users` would forward `GET /api/v1/users/me` upstream as `/me`. Identity serves `GET /users/me` (mirroring the `/register`, `/login` leaf-path convention), so the protected route must strip only `/api/v1`, forwarding `/users/me`. Kong resolves the more specific routes first (`/api/v1/auth` and `/api/v1/storefront` are longer prefixes and win for their requests), so auth and storefront routing is unaffected.

The global CORS `plugins:` block at the bottom of the file stays unchanged.

- [ ] **Step 2: Validate the compose/declarative YAML**

Run (from repo root):

```bash
docker compose -f deployments/docker/compose.foundation.yml config -q
```

Expected: exit code 0, no output on success. (This validates that the mounted `kong.yaml` path and compose file parse; Kong itself parses the declarative config at startup, exercised end-to-end in Task 8.)

- [ ] **Step 3: Commit**

```bash
git add deployments/kong/kong.yaml
git commit -m "feat(kong): split identity routes and protect /api/v1/users with caller-context" -- deployments/kong/kong.yaml
```

---

## Task 3: Identity `MeApiTest` (failing)

This test targets `GET /users/me` end-to-end through the Spring Security chain. It compiles and runs even though the endpoint does not exist yet — today `/users/me` is `denyAll`, so the tests fail, which is the desired red state.

**Files:**
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/user/api/MeApiTest.java`

- [ ] **Step 1: Write the failing test**

```java
package com.ecommerce.identity.user.api;

import static org.hamcrest.Matchers.matchesPattern;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import com.jayway.jsonpath.JsonPath;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.http.MediaType;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;

@AutoConfigureMockMvc
class MeApiTest extends PostgresIntegrationTestSupport {
    @Autowired
    MockMvc mockMvc;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void resetDatabase() {
        jdbcTemplate.update("delete from user_roles");
        jdbcTemplate.update("delete from identity_profiles");
        jdbcTemplate.update("delete from identity_users");
        jdbcTemplate.update("delete from roles");
        jdbcTemplate.update("insert into roles (id, code) values (1, 'CUSTOMER')");
    }

    @Test
    void returnsCallerProfileForVerifiedUserIdHeader() throws Exception {
        String userId = registerUser("me@example.com");

        mockMvc.perform(get("/users/me").header("X-User-Id", userId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.user_id").value(userId))
                .andExpect(jsonPath("$.email").value("me@example.com"))
                .andExpect(jsonPath("$.full_name").value("Me User"))
                .andExpect(jsonPath("$.roles[0]").value("CUSTOMER"));
    }

    @Test
    void rejectsRequestWithoutCallerHeader() throws Exception {
        mockMvc.perform(get("/users/me"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("UNAUTHORIZED"))
                .andExpect(jsonPath("$.trace_id", matchesPattern("[0-9a-f]{32}")));
    }

    @Test
    void rejectsMalformedCallerHeader() throws Exception {
        mockMvc.perform(get("/users/me").header("X-User-Id", "not-a-uuid"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("UNAUTHORIZED"));
    }

    @Test
    void returnsNotFoundForUnknownUser() throws Exception {
        mockMvc.perform(get("/users/me").header("X-User-Id", UUID.randomUUID().toString()))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value("USER_NOT_FOUND"));
    }

    @Test
    void returnsNotFoundForDisabledAccount() throws Exception {
        String userId = registerUser("disabled@example.com");
        jdbcTemplate.update("update identity_users set status = 'DISABLED' where normalized_email = ?",
                "disabled@example.com");

        mockMvc.perform(get("/users/me").header("X-User-Id", userId))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value("USER_NOT_FOUND"));
    }

    private String registerUser(String email) throws Exception {
        MvcResult result = mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\"%s\",\"password\":\"correct horse battery staple\",\"full_name\":\"Me User\"}"
                                .formatted(email)))
                .andExpect(status().isCreated())
                .andReturn();
        return JsonPath.read(result.getResponse().getContentAsString(), "$.user_id");
    }
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run (from repo root; Docker must be running for Testcontainers):

```bash
mvn -q -pl services/identity-service -am test -Dtest=MeApiTest -Dsurefire.failIfNoSpecifiedTests=false
```

Expected: FAIL. `GET /users/me` currently returns `403` (`denyAll`), so `status().isOk()` / `isUnauthorized()` / `isNotFound()` assertions fail. (Compilation succeeds because the test only uses MockMvc and existing classes.) Note the `-Dsurefire.failIfNoSpecifiedTests=false` flag: without it, Maven's upstream `contracts` module (which has no tests) aborts the run with "No tests were executed".

---

## Task 4: Identity caller-context plumbing

**Files:**
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/security/CallerContext.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/security/CallerContextPrincipal.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/security/CallerContextFilter.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/security/SecurityConfiguration.java`

- [ ] **Step 1: Create the caller value object**

`CallerContext.java`:

```java
package com.ecommerce.identity.security;

import java.util.Set;
import java.util.UUID;

public record CallerContext(UUID userId, Set<String> roles) {
}
```

- [ ] **Step 2: Create the Authentication implementation**

`CallerContextPrincipal.java`:

```java
package com.ecommerce.identity.security;

import java.util.Collection;
import java.util.Set;
import java.util.UUID;
import org.springframework.security.core.Authentication;
import org.springframework.security.core.GrantedAuthority;

public record CallerContextPrincipal(UUID userId, Set<String> roles) implements Authentication {

    @Override
    public Collection<? extends GrantedAuthority> getAuthorities() {
        return Set.of();
    }

    @Override
    public Object getCredentials() {
        return null;
    }

    @Override
    public Object getDetails() {
        return null;
    }

    @Override
    public Object getPrincipal() {
        return this;
    }

    @Override
    public boolean isAuthenticated() {
        return true;
    }

    @Override
    public void setAuthenticated(boolean isAuthenticated) throws IllegalArgumentException {
        throw new IllegalArgumentException("Caller context is always authenticated");
    }

    @Override
    public String getName() {
        return userId.toString();
    }
}
```

- [ ] **Step 3: Create the header-reading filter**

`CallerContextFilter.java`:

```java
package com.ecommerce.identity.security;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.util.Arrays;
import java.util.Set;
import java.util.UUID;
import java.util.stream.Collectors;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.filter.OncePerRequestFilter;

public class CallerContextFilter extends OncePerRequestFilter {
    static final String USER_ID_HEADER = "X-User-Id";
    static final String ROLES_HEADER = "X-User-Roles";

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain filterChain)
            throws ServletException, IOException {
        String userId = request.getHeader(USER_ID_HEADER);
        if (userId != null && !userId.isBlank()) {
            try {
                SecurityContextHolder.getContext().setAuthentication(
                        new CallerContextPrincipal(UUID.fromString(userId), parseRoles(request.getHeader(ROLES_HEADER))));
            } catch (IllegalArgumentException ignored) {
                // malformed header: stay unauthenticated, authorization decides
            }
        }
        try {
            filterChain.doFilter(request, response);
        } finally {
            SecurityContextHolder.clearContext();
        }
    }

    private static Set<String> parseRoles(String header) {
        if (header == null || header.isBlank()) {
            return Set.of();
        }
        return Arrays.stream(header.split(","))
                .map(String::trim)
                .filter(value -> !value.isEmpty())
                .collect(Collectors.toUnmodifiableSet());
    }
}
```

- [ ] **Step 4: Wire the filter and 401 entry point into the security chain**

Replace `SecurityConfiguration.java` with:

```java
package com.ecommerce.identity.security;

import com.ecommerce.identity.common.error.ApiError;
import com.ecommerce.identity.common.error.TraceIdProvider;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import tools.jackson.databind.ObjectMapper;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.http.HttpMethod;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.annotation.web.configurers.AbstractHttpConfigurer;
import org.springframework.security.config.http.SessionCreationPolicy;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.access.intercept.AuthorizationFilter;

@Configuration
public class SecurityConfiguration {
    @Bean
    SecurityFilterChain securityFilterChain(HttpSecurity http, ObjectMapper objectMapper) throws Exception {
        return http
                .csrf(AbstractHttpConfigurer::disable)
                .sessionManagement(session -> session.sessionCreationPolicy(SessionCreationPolicy.STATELESS))
                .formLogin(AbstractHttpConfigurer::disable)
                .httpBasic(AbstractHttpConfigurer::disable)
                .logout(AbstractHttpConfigurer::disable)
                .addFilterBefore(new CallerContextFilter(), AuthorizationFilter.class)
                .exceptionHandling(exceptionHandling -> exceptionHandling
                        .authenticationEntryPoint((request, response, exception) -> writeUnauthorized(objectMapper, response))
                        .accessDeniedHandler((request, response, exception) -> writeDenied(objectMapper, response)))
                .authorizeHttpRequests(authorize -> authorize
                        .requestMatchers(HttpMethod.POST, "/register").permitAll()
                        .requestMatchers(HttpMethod.POST, "/login").permitAll()
                        .requestMatchers(HttpMethod.GET, "/.well-known/jwks.json").permitAll()
                        .requestMatchers(HttpMethod.GET, "/actuator/health", "/actuator/health/**").permitAll()
                        .requestMatchers(HttpMethod.GET, "/users/me").authenticated()
                        .anyRequest().denyAll())
                .build();
    }

    private static void writeUnauthorized(ObjectMapper objectMapper, HttpServletResponse response) throws IOException {
        response.setStatus(HttpStatus.UNAUTHORIZED.value());
        response.setContentType(MediaType.APPLICATION_JSON_VALUE);
        objectMapper.writeValue(response.getOutputStream(),
                new ApiError("UNAUTHORIZED", "Missing or invalid access token", TraceIdProvider.current()));
    }

    private static void writeForbidden(ObjectMapper objectMapper, HttpServletResponse response) throws IOException {
        response.setStatus(HttpStatus.FORBIDDEN.value());
        response.setContentType(MediaType.APPLICATION_JSON_VALUE);
        objectMapper.writeValue(response.getOutputStream(),
                new ApiError("FORBIDDEN", "Access is denied", TraceIdProvider.current()));
    }

    private static void writeDenied(ObjectMapper objectMapper, HttpServletResponse response) throws IOException {
        if (SecurityContextHolder.getContext().getAuthentication() instanceof CallerContextPrincipal) {
            writeForbidden(objectMapper, response);
        } else {
            writeUnauthorized(objectMapper, response);
        }
    }
}
```

Note: `CallerContextFilter` is created **inline** inside `securityFilterChain(...)`, NOT declared as a bean. A `Filter` bean would be auto-registered by Spring Boot as a standalone servlet filter (running a second time after the security chain) *and* added to the chain via `addFilterBefore` — double execution. Inline instantiation registers it exactly once, inside the security chain.

Note on 401-vs-403: with Spring Security 7.1 the `ExceptionTranslationFilter` sends denied requests to the `authenticationEntryPoint` (401) only when the context holds an *anonymous* or *remember-me* authentication; a null context falls through to the `accessDeniedHandler` (403). This app does not install an anonymous token, so a missing/malformed `X-User-Id` produced 403 instead of the contract's 401. The `accessDeniedHandler` therefore decides on the actual authentication type: only a real `CallerContextPrincipal` that still fails authorization is 403; everything else is 401.

- [ ] **Step 5: Run the failing test to confirm progress**

```bash
mvn -q -pl services/identity-service -am test -Dtest=MeApiTest -Dsurefire.failIfNoSpecifiedTests=false
```

Expected: `rejectsRequestWithoutCallerHeader`, `rejectsMalformedCallerHeader` now PASS (401). `returnsCallerProfileForVerifiedUserIdHeader` still FAILS (404: no controller). `returnsNotFoundForUnknownUser` / `returnsNotFoundForDisabledAccount` still FAIL (404/500: no controller or exception mapping).

---

## Task 5: Identity `/users/me` feature

**Files:**
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityUserRepository.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/domain/UserNotFoundException.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/application/CurrentUser.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/application/MeService.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/api/MeResponse.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/api/MeController.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java`

- [ ] **Step 1: Add the profile+roles repository query**

Add to `IdentityUserRepository.java` (after `findWithRolesByNormalizedEmail`):

```java
@EntityGraph(attributePaths = {"profile", "roles"})
Optional<IdentityUserEntity> findWithProfileAndRolesById(UUID id);
```

- [ ] **Step 2: Create the not-found exception**

`UserNotFoundException.java`:

```java
package com.ecommerce.identity.user.domain;

public class UserNotFoundException extends RuntimeException {
}
```

- [ ] **Step 3: Create the application result record**

`CurrentUser.java`:

```java
package com.ecommerce.identity.user.application;

import java.util.Set;
import java.util.UUID;

public record CurrentUser(UUID userId, String email, String fullName, Set<String> roles) {
}
```

- [ ] **Step 4: Create the application service**

`MeService.java`:

```java
package com.ecommerce.identity.user.application;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.user.domain.UserNotFoundException;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.util.UUID;
import org.springframework.stereotype.Service;

@Service
public class MeService {
    private final IdentityUserRepository repository;

    public MeService(IdentityUserRepository repository) {
        this.repository = repository;
    }

    public CurrentUser current(UUID userId) {
        IdentityUserEntity user = repository.findWithProfileAndRolesById(userId)
                .filter(candidate -> candidate.status() == AccountStatus.ACTIVE)
                .orElseThrow(UserNotFoundException::new);
        return new CurrentUser(user.id(), user.email(), user.fullName(), user.roleCodes());
    }
}
```

- [ ] **Step 5: Create the response DTO**

`MeResponse.java`:

```java
package com.ecommerce.identity.user.api;

import com.ecommerce.identity.user.application.CurrentUser;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.Set;
import java.util.UUID;

public record MeResponse(
        @JsonProperty("user_id") UUID userId,
        String email,
        @JsonProperty("full_name") String fullName,
        Set<String> roles) {
    static MeResponse from(CurrentUser user) {
        return new MeResponse(user.userId(), user.email(), user.fullName(), user.roles());
    }
}
```

- [ ] **Step 6: Create the controller**

`MeController.java`:

```java
package com.ecommerce.identity.user.api;

import com.ecommerce.identity.security.CallerContextPrincipal;
import com.ecommerce.identity.user.application.MeService;
import lombok.RequiredArgsConstructor;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class MeController {
    private final MeService meService;

    @GetMapping("/users/me")
    MeResponse me(Authentication authentication) {
        CallerContextPrincipal caller = (CallerContextPrincipal) authentication.getPrincipal();
        return MeResponse.from(meService.current(caller.userId()));
    }
}
```

- [ ] **Step 7: Map the not-found exception**

Add to `GlobalExceptionHandler.java` (next to the other handlers):

```java
@ExceptionHandler(UserNotFoundException.class)
ResponseEntity<ApiError> handleUserNotFound(UserNotFoundException exception) {
    return error(HttpStatus.NOT_FOUND, "USER_NOT_FOUND", "User not found");
}
```

and add the import:

```java
import com.ecommerce.identity.user.domain.UserNotFoundException;
```

- [ ] **Step 8: Run the test to verify it passes**

```bash
mvn -q -pl services/identity-service -am test -Dtest=MeApiTest -Dsurefire.failIfNoSpecifiedTests=false
```

Expected: all five `MeApiTest` cases PASS.

- [ ] **Step 9: Run the full identity suite to check for regressions**

```bash
mvn -q -pl services/identity-service -am test
```

Expected: `BUILD SUCCESS`; existing register/login/JWKS/persistence tests still pass. (Docker must be running for Testcontainers.)

- [ ] **Step 10: Commit**

```bash
git add services/identity-service/src/main/java/com/ecommerce/identity/user services/identity-service/src/main/java/com/ecommerce/identity/security services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/test/java/com/ecommerce/identity/user
git commit -m "feat(identity): consume caller context headers and serve /users/me" -- services/identity-service/src/main/java/com/ecommerce/identity/user services/identity-service/src/main/java/com/ecommerce/identity/security services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/test/java/com/ecommerce/identity/user
```

---

## Task 6: End-to-end smoke script

**Files:**
- Create: `scripts/verify-edge-auth.ps1`
- Modify: `Makefile`

- [ ] **Step 1: Write the smoke script**

`scripts/verify-edge-auth.ps1`:

```powershell
[CmdletBinding()]
param(
    [string] $AuthBaseUrl = 'http://localhost:8000/api/v1/auth',
    [string] $UsersBaseUrl = 'http://localhost:8000/api/v1/users'
)

$ErrorActionPreference = 'Stop'

function ConvertFrom-Base64Url([string] $Value) {
    $padded = $Value.Replace('-', '+').Replace('_', '/')
    switch ($padded.Length % 4) {
        2 { $padded += '==' }
        3 { $padded += '=' }
        1 { throw 'Invalid Base64URL value.' }
    }
    return [Convert]::FromBase64String($padded)
}

function Assert-StatusCode([object] $Response, [int] $Expected, [string] $Operation) {
    if ($Response.StatusCode -ne $Expected) {
        throw "$Operation returned unexpected HTTP status $($Response.StatusCode)."
    }
}

$authBase = $AuthBaseUrl.TrimEnd('/')
$usersBase = $UsersBaseUrl.TrimEnd('/')
$email = "edge-auth-$([Guid]::NewGuid().ToString('N'))@example.test"
$password = "local-only-$([Guid]::NewGuid().ToString('N'))"
$requestBody = @{ email = $email; password = $password; full_name = 'Edge Auth Smoke Test' } | ConvertTo-Json -Compress

$register = Invoke-WebRequest -Uri "$authBase/register" -Method Post -ContentType 'application/json' -Body $requestBody -SkipHttpErrorCheck
Assert-StatusCode $register 201 'Registration'

$login = Invoke-WebRequest -Uri "$authBase/login" -Method Post -ContentType 'application/json' -Body (@{ email = $email; password = $password } | ConvertTo-Json -Compress) -SkipHttpErrorCheck
Assert-StatusCode $login 200 'Login'

$accessToken = ($login.Content | ConvertFrom-Json).access_token
if ([string]::IsNullOrWhiteSpace($accessToken)) {
    throw 'Login response did not contain an access token.'
}

$claims = [Text.Encoding]::UTF8.GetString((ConvertFrom-Base64Url $accessToken.Split('.')[1])) | ConvertFrom-Json

$me = Invoke-WebRequest -Uri "$usersBase/me" -Headers @{ Authorization = "Bearer $accessToken" } -SkipHttpErrorCheck
Assert-StatusCode $me 200 'Authenticated /users/me'
$meBody = $me.Content | ConvertFrom-Json
if ($meBody.user_id -ne $claims.sub) {
    throw '/users/me did not return the caller matching the token subject.'
}
if (@($meBody.roles) -notcontains 'CUSTOMER') {
    throw '/users/me did not return the CUSTOMER role.'
}
if ($meBody.email -ne $email) {
    throw '/users/me did not return the registered email.'
}

$anonymous = Invoke-WebRequest -Uri "$usersBase/me" -SkipHttpErrorCheck
Assert-StatusCode $anonymous 401 'Unauthenticated /users/me'
if (($anonymous.Content | ConvertFrom-Json).code -ne 'UNAUTHORIZED') {
    throw 'Unauthenticated /users/me did not return UNAUTHORIZED.'
}

$tokenParts = $accessToken.Split('.')
if ($tokenParts.Count -ne 3) {
    throw 'Access token is not a three-part JWT.'
}
$tamperedSignature = $tokenParts[2].Substring(0, $tokenParts[2].Length - 1) + $(if ($tokenParts[2].EndsWith('x')) { 'y' } else { 'x' })
$tamperedToken = "$($tokenParts[0]).$($tokenParts[1]).$tamperedSignature"
$tampered = Invoke-WebRequest -Uri "$usersBase/me" -Headers @{ Authorization = "Bearer $tamperedToken" } -SkipHttpErrorCheck
Assert-StatusCode $tampered 401 'Tampered-token /users/me'
if (($tampered.Content | ConvertFrom-Json).code -ne 'UNAUTHORIZED') {
    throw 'Kong accepted a tampered token signature.'
}

$forgedHeaders = @{ Authorization = "Bearer $accessToken"; 'X-User-Id' = [Guid]::NewGuid().ToString() }
$forged = Invoke-WebRequest -Uri "$usersBase/me" -Headers $forgedHeaders -SkipHttpErrorCheck
Assert-StatusCode $forged 200 'Forged-header /users/me'
if (($forged.Content | ConvertFrom-Json).user_id -ne $claims.sub) {
    throw 'Kong forwarded a client-supplied X-User-Id instead of the verified subject.'
}

Write-Host 'Kong edge auth /users/me smoke checks passed.'
```

- [ ] **Step 2: Add the Makefile target**

Add to `Makefile` (next to `verify-identity`):

```make
verify-edge-auth:
	pwsh -NoProfile -File scripts/verify-edge-auth.ps1
```

- [ ] **Step 3: Commit**

```bash
git add scripts/verify-edge-auth.ps1 Makefile
git commit -m "test(edge-auth): smoke script for Kong /users/me enforcement" -- scripts/verify-edge-auth.ps1 Makefile
```

---

## Task 7: End-to-end verification through Kong

**Files:** none (verification only)

- [ ] **Step 1: Build and start the stack**

Run (from repo root):

```bash
make up
```

Expected: all containers start; Kong restarts with the custom plugin loaded; Identity is healthy. Check `docker compose -f deployments/docker/compose.foundation.yml ps` shows `ecommerce-kong` running.

- [ ] **Step 2: Run the edge-auth smoke checks**

```bash
make verify-edge-auth
```

Expected: prints `Kong edge auth /users/me smoke checks passed.` (This exercises the JWKS fetch, RS256 verification, header strip/inject, and `/users/me`.)

- [ ] **Step 3: Run the existing identity smoke checks for regressions**

```bash
make verify-identity
```

Expected: prints `Identity register/login/JWKS smoke checks passed.`

- [ ] **Step 4: Manual confirmation of public routes staying open**

Run:

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/api/v1/auth/.well-known/jwks.json
```

Expected: `200` (JWKS still public; no token required).

---

## Self-review notes

- Spec coverage: plugin behavior (§6.3/§6.4) → Task 1; route split (§6.5) → Task 2; Identity caller context (§8.1/§8.2) → Task 4; `/users/me` (§7/§8.3) → Task 5; test strategy (§11) → Tasks 3/5/6; operational JWKS cache (§12) is inherent to the plugin code.
- No placeholders: every code file is written in full above.
- Type consistency: `CallerContextPrincipal(userId, roles)` is used in `CallerContextFilter`, `MeController`, and matches `CallerContext`; `CurrentUser` fields flow into `MeResponse`; repository method `findWithProfileAndRolesById(UUID)` matches `MeService` usage.
- Known env caveats: Maven tests need Docker (Testcontainers PostgreSQL); `make up` needs local Docker. If `pkey:verify` behaves differently in the installed Kong OpenSSL build, run Task 7 first before debugging plugin crypto.
