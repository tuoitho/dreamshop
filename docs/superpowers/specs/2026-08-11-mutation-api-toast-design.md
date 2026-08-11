# Mutation API Toast Design

## Goal

Show a consistent success or failure toast for every user-initiated frontend API mutation. Toast text comes from the API response `message` field.

## Scope

- Add the `sonner` toast library to the React application and mount its `Toaster` once at the application root.
- Add response handling to the shared Axios `http` client for methods other than `GET`.
- Do not show a toast for token refresh because it uses the separate `refreshHttp` client and runs automatically.
- Ensure Identity service REST request and response fields use `camelCase`, with mutation responses exposing a `message` field for both successful and failed requests.

## Frontend Behavior

The shared `http` Axios instance will inspect request method and response data:

- A successful non-GET request calls `toast.success(data.message)`.
- A failed non-GET request calls `toast.error(response.data.message)`.
- For failures without a JSON response message, the client uses the existing transport error message.
- GET requests never cause a toast.
- Retried authenticated mutations are toasted only when their final request succeeds or fails.

Mutation success responses use the envelope `{ data, message }`. The API client unwraps `data` before returning to callers, so existing UI consumers continue to receive endpoint data directly. TypeScript types and payloads use `camelCase`.

## Identity API Contract

All existing error responses use the `ApiError` envelope and expose `message`. The entire Identity REST contract uses `camelCase`, including request fields, response fields, validation error keys, and `traceId`. Successful mutation endpoints use `{ data, message }`:

- `POST /register`
- `POST /login`
- `POST /logout`
- `PATCH /users/me`

`POST /logout` changes from `204 No Content` to `200 OK` so its body can provide the required `message`.

Read endpoints retain their current payload shapes because they do not participate in mutation toast behavior.

## Verification

- Frontend tests prove mutation success and error messages invoke the appropriate Sonner toast, while GET and token refresh do not.
- Identity MockMvc tests prove each mutation success response contains `message`; existing tests retain error-message assertions.
- Run focused frontend and Identity tests, then frontend build and relevant service test command.
