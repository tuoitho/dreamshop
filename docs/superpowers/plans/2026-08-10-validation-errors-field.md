# Validation Error Field Details Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Include a field-level `errors` map in the `400 VALIDATION_FAILED` response from the Identity service's `@Valid` DTOs, while keeping malformed-body and other error classes unchanged.

**Architecture:** `ApiError` gains an optional `errors` map. `GlobalExceptionHandler` splits the current combined validation handler: `MethodArgumentNotValidException` builds the map from `BindingResult` field errors (Java property names, no rejected values) and logs each one at `warn`; `HttpMessageNotReadableException` keeps the generic envelope without `errors`. The frontend `ApiError` type gains an optional `errors` field.

**Tech Stack:** Java 21, Spring Boot 4.1, Jackson 3 (annotations via `com.fasterxml.jackson.annotation`, already used by `ApiError`), JUnit 5, MockMvc, Testcontainers, TypeScript (Vitest).

---

### Task 1: Extend ApiError and split the validation handler (TDD)

**Files:**
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/common/error/ApiError.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/security/SecurityConfiguration.java:50,57`
- Test: `services/identity-service/src/test/java/com/ecommerce/identity/auth/api/RegisterApiTest.java`

- [ ] **Step 1: Write the failing tests**

Update `rejectsInvalidRegistrationWithSafeValidationError` in `RegisterApiTest.java` to also assert the `errors` map, and add a new test for malformed JSON. The request `{"email":"not-an-email","password":"short","full_name":""}` produces Hibernate Validator defaults:
- `email` → `must be a well-formed email address`
- `password` → `size must be between 12 and 128`
- `fullName` → `must not be blank` (Java property name, per decision)

Replace the current `rejectsInvalidRegistrationWithSafeValidationError` body with:

```java
    @Test
    void rejectsInvalidRegistrationWithSafeValidationError() throws Exception {
        mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"email":"not-an-email","password":"short","full_name":""}
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.message").value("Request validation failed"))
                .andExpect(jsonPath("$.errors.email").value("must be a well-formed email address"))
                .andExpect(jsonPath("$.errors.password").value("size must be between 12 and 128"))
                .andExpect(jsonPath("$.errors.fullName").value("must not be blank"))
                .andExpect(jsonPath("$.trace_id", matchesPattern("[0-9a-f]{32}")));
    }

    @Test
    void rejectsMalformedBodyWithoutFieldErrors() throws Exception {
        mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\""))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors").doesNotExist());
    }
```

- [ ] **Step 2: Run the tests to verify they fail**

Run (requires Docker for Testcontainers):
`mvn -pl services/identity-service -am test -Dtest=RegisterApiTest -Dsurefire.failIfNoSpecifiedTests=false`

Expected: `rejectsInvalidRegistrationWithSafeValidationError` FAILS (`$.errors.email` cannot be resolved because `ApiError` has no `errors` field); `rejectsMalformedBodyWithoutFieldErrors` currently PASSES (no `errors` serialized yet).

- [ ] **Step 3: Add the `errors` field to ApiError**

Replace the contents of `ApiError.java` with:

```java
package com.ecommerce.identity.common.error;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.Map;

@JsonInclude(JsonInclude.Include.NON_NULL)
public record ApiError(
        String code,
        String message,
        Map<String, String> errors,
        @JsonProperty("trace_id") String traceId) {
}
```

- [ ] **Step 4: Split the validation handler and update callers**

In `GlobalExceptionHandler.java`, replace the current single `handleInvalidRequest` handler (lines 19-22) with two handlers:

```java
    @ExceptionHandler(MethodArgumentNotValidException.class)
    ResponseEntity<ApiError> handleValidationFailure(MethodArgumentNotValidException exception) {
        Map<String, String> errors = new LinkedHashMap<>();
        exception.getBindingResult().getFieldErrors().forEach(error -> {
            log.warn("validation_failed field={} message={}", error.getField(), error.getDefaultMessage());
            errors.put(error.getField(), error.getDefaultMessage());
        });
        return ResponseEntity.status(HttpStatus.BAD_REQUEST).body(new ApiError(
                "VALIDATION_FAILED", "Request validation failed", errors, TraceIdProvider.current()));
    }

    @ExceptionHandler(HttpMessageNotReadableException.class)
    ResponseEntity<ApiError> handleUnreadableRequest(HttpMessageNotReadableException exception) {
        return error(HttpStatus.BAD_REQUEST, "VALIDATION_FAILED", "Request validation failed");
    }
```

Add imports to `GlobalExceptionHandler.java`:

```java
import java.util.LinkedHashMap;
import java.util.Map;
```

Update the private `error(...)` helper (line 56-58) to pass `null` for errors:

```java
    private ResponseEntity<ApiError> error(HttpStatus status, String code, String message) {
        return ResponseEntity.status(status).body(new ApiError(code, message, null, TraceIdProvider.current()));
    }
```

Update both `ApiError` constructor calls in `SecurityConfiguration.java` to add `null` for the new `errors` argument:

```java
                new ApiError("UNAUTHORIZED", "Missing or invalid access token", null, TraceIdProvider.current()));
```
```java
                new ApiError("FORBIDDEN", "Access is denied", null, TraceIdProvider.current()));
```

- [ ] **Step 5: Run the tests to verify they pass**

Run:
`mvn -pl services/identity-service -am test -Dtest=RegisterApiTest -Dsurefire.failIfNoSpecifiedTests=false`

Expected: both updated/new tests PASS. Then run the full identity suite to catch regressions in `MeApiTest` (only asserts `code`, so must stay green) and `JwksApiTest`/`RefreshApiTest`/`LoginApiTest`:

`mvn -pl services/identity-service -am test`

- [ ] **Step 6: Commit**

```bash
git add services/identity-service/src/main/java/com/ecommerce/identity/common/error/ApiError.java services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/main/java/com/ecommerce/identity/security/SecurityConfiguration.java services/identity-service/src/test/java/com/ecommerce/identity/auth/api/RegisterApiTest.java
git commit -m "feat(identity): return field-level validation errors"
```

---

### Task 2: Update frontend ApiError type

**Files:**
- Modify: `ecommerce-fe/src/lib/types.ts`

- [ ] **Step 1: Add the optional `errors` field**

In `ecommerce-fe/src/lib/types.ts`, change the `ApiError` interface to:

```ts
export interface ApiError {
  code: string
  message: string
  errors?: Record<string, string>
  trace_id: string
}
```

- [ ] **Step 2: Run frontend checks**

Run:
`npm --prefix ecommerce-fe run test`

Expected: existing tests (including `api.test.ts` envelope translation) PASS. Then run:

`npm --prefix ecommerce-fe run lint`

Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
git add ecommerce-fe/src/lib/types.ts
git commit -m "feat(frontend): type field-level validation errors"
```

---

### Task 3: Update the error-contract documentation

**Files:**
- Modify: `docs/superpowers/specs/2026-08-03-identity-register-login-jwks-design.md`

- [ ] **Step 1: Document the `errors` map**

In the "Error and logging contract" section, after the line `Internal exception messages, SQL, hostnames and key paths never appear in client responses.` (line 311), add:

```text
The `VALIDATION_FAILED` response may additionally include an `errors` map whose keys are Java bean property names and whose values are the corresponding validation messages, e.g. `{"code":"VALIDATION_FAILED","message":"Request validation failed","errors":{"password":"size must be between 12 and 128"},"trace_id":"<trace identifier>"}`. Field names and validation messages are safe to expose; rejected values, SQL and internal exception text still never appear. Malformed request bodies return the envelope without the `errors` key.
```

- [ ] **Step 2: Commit**

```bash
git add docs/superpowers/specs/2026-08-03-identity-register-login-jwks-design.md
git commit -m "docs: describe validation error details in error contract"
```

---

### Task 4: End-to-end verification

- [ ] **Step 1: Full Maven build**

Run (from repo root, Docker running):
`mvn test`

Expected: all Maven modules (`contracts/java`, `services/identity-service`, `services/order-service`) build and test green.

- [ ] **Step 2: Frontend build**

Run:
`npm --prefix ecommerce-fe run build`

Expected: TypeScript compiles with the updated `ApiError` type.
