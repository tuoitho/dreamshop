# Validation Error Field Details Design

## Summary

The Identity service currently returns the same generic `VALIDATION_FAILED` envelope for every invalid request, which hides which field failed and why. This slice adds an optional `errors` map to the validation error response so clients and developers can act on field-level validation messages, while keeping the existing envelope and codes stable for other error classes.

## Goals

- For bean-validation failures (`@Valid` DTOs), include `errors` as a map of `field -> validation message` in the `400 VALIDATION_FAILED` response.
- Return the field detail on every environment (no Spring-profile gating), exposing only field names and validation messages — never rejected values, SQL, or internal exception text.
- Keep the existing envelope stable for all other error classes (including malformed/unreadable JSON bodies).
- Update the frontend `ApiError` type so callers can use the new field.

## Non-goals

- No per-field error codes, ordering guarantees, or i18n of messages.
- No `errors` for `HttpMessageNotReadableException`; malformed bodies keep the current generic envelope.
- No change to `LoginRequest`, `RegisterRequest`, or `UpdateMeRequest` validation rules.
- No change to the response envelope for `EMAIL_ALREADY_REGISTERED`, `INVALID_CREDENTIALS`, `INVALID_REFRESH_TOKEN`, `AUTHENTICATION_UNAVAILABLE`, `USER_NOT_FOUND`, or `INTERNAL_ERROR`.

## Response shape

Bean-validation failure:

```json
{
  "code": "VALIDATION_FAILED",
  "message": "Request validation failed",
  "errors": {
    "email": "must be a well-formed email address",
    "password": "size must be between 12 and 128"
  },
  "trace_id": "bdb3a638ca664e7e8e2cff7715bc28fe"
}
```

Malformed body (unchanged, no `errors` key):

```json
{
  "code": "VALIDATION_FAILED",
  "message": "Request validation failed",
  "trace_id": "bdb3a638ca664e7e8e2cff7715bc28fe"
}
```

## Behavior

### Exception handling

Split the current combined handler in `GlobalExceptionHandler` into two:

- `MethodArgumentNotValidException` builds the `errors` map from `getBindingResult().getFieldErrors()`, mapping `field -> getDefaultMessage()`. When a field has multiple constraint violations, the map keeps one entry (last message wins). Rejected values are never included. Each field error is logged at `warn` with `field` and `message`.
- `HttpMessageNotReadableException` keeps the existing `error(HttpStatus.BAD_REQUEST, "VALIDATION_FAILED", "Request validation failed")` call with no `errors`.

### Envelope

`ApiError` gains a nullable `errors` field of type `Map<String, String>` serialized with `@JsonInclude(Include.NON_NULL)` so the key is absent (not `null`) when unset.

### Scope

Applies to every `@Valid` request DTO: `RegisterRequest`, `LoginRequest`, and `UpdateMeRequest`.

## Testing

- Update `RegisterApiTest.rejectsInvalidRegistrationWithSafeValidationError` to assert `$.errors` contains the `email` and `password` field messages and still returns `400 VALIDATION_FAILED` with a `trace_id`.
- Add an integration test asserting a malformed JSON body returns `400 VALIDATION_FAILED` without an `errors` key.
- Verify existing assertions on other error classes still hold (they only check `code`/`message`/`trace_id`, so the optional `errors` field does not break them).
- Frontend: extend `ApiError` in `ecommerce-fe/src/lib/types.ts` with `errors?: Record<string, string>`.

## Documentation

Update the error-contract section of `docs/superpowers/specs/2026-08-03-identity-register-login-jwks-design.md` to record that `VALIDATION_FAILED` may include an `errors` map of field-level validation messages, and that rejected values and internal details are still never exposed.
