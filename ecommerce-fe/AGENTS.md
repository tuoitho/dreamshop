# Frontend Instructions

These instructions apply to `ecommerce-fe/` in addition to the root
[`AGENTS.md`](../AGENTS.md).

## Ownership

The frontend owns:

- Presentation, responsive layout, and accessible interaction.
- Client routing and protected-route user experience.
- View-local and client session state.
- API adaptation into UI-friendly types and stable error handling.
- Loading, empty, validation, success, and failure feedback.

The frontend does not own Identity, Catalog, Inventory, Cart, or Order business
truth. Client validation improves user experience but never replaces server
validation or authorization. Do not reproduce server state machines or security
decisions in components.

## API Boundaries

- Send public API requests through Kong using the configured API base URL.
- Keep API calls in adapters under `src/lib/` rather than scattering Axios calls
  through pages and components.
- Align TypeScript request/response fields with the REST contract using
  `camelCase`.
- Parse the shared success/pagination/error envelopes consistently.
- Branch client behavior on stable error `code`, not human-readable `message`.
- Use mutation success `message` for notifications when the contract provides it.
- Do not expose internal gRPC/protobuf transport concepts directly to UI
  components.
- Inspect backend consumers/contracts before changing frontend assumptions about
  optional fields, enums, status codes, or error details.

## Session and Security

- Keep access tokens in memory; never store them in `localStorage`,
  `sessionStorage`, URLs, or logs.
- Send refresh tokens only through the server-managed HttpOnly cookie.
- On application reload, restore the session through the refresh endpoint.
- Retry a protected request after refresh at most once.
- Coalesce concurrent refresh attempts through one shared promise.
- If refresh fails, clear in-memory auth state and transition to the unauthenticated
  flow without retry loops.
- Do not render raw backend exceptions, stack traces, tokens, or sensitive
  response details.

Changes to token storage, refresh concurrency, retry count, cookie assumptions,
or protected routing require focused session regression tests and security impact
review.

## Source Layout

- `src/components/ui/`: reusable UI primitives; extend/reuse them before adding
  one-off controls.
- `src/components/layout/`: shared storefront layout.
- `src/context/`: provider state and context definitions.
- `src/hooks/`: typed hooks that consume context or reusable client behavior.
- `src/lib/`: API client, auth session, types, validation, and utilities.
- `src/pages/`: route-level composition and user workflows.
- `src/routes/`: route guards and routing-specific behavior.
- `src/data/`: local/demo data only; it is not backend business truth.

Keep pages focused on composition. Put reusable interactions in components,
shared state access in hooks/context, and transport/session behavior in `lib`.
Avoid broad global context for state that belongs to one page or component tree.

## React and TypeScript

- Use TypeScript and preserve strict type checking.
- Prefer explicit domain/UI types over untyped objects or unchecked casts.
- Keep React components focused and keep side effects in well-bounded hooks or
  providers.
- Preserve semantic HTML, keyboard access, visible focus, labels, and usable
  validation feedback.
- Handle loading, empty, failure, and retry states for asynchronous screens.
- Reuse existing Tailwind and UI primitive conventions rather than introducing a
  parallel design system.
- Do not add a dependency when the existing stack can express the behavior
  clearly.

## Testing

Use Vitest and Testing Library. Test behavior visible to the user and avoid
asserting implementation details. Place tests under `src/` as `*.test.ts` or
`*.test.tsx`.

Cover affected paths such as:

- API envelope/error-code mapping.
- Login, refresh, concurrent refresh, logout, and session restoration.
- Protected-route transitions.
- Client validation plus server validation/error feedback.
- Mutation success notification.
- Keyboard and accessible form interaction.

From repository root, run:

```powershell
npm --prefix ecommerce-fe run test
npm --prefix ecommerce-fe run lint
npm --prefix ecommerce-fe run build
```

During iteration, a focused Vitest file is acceptable; run the full affected
frontend checks before handoff.

## Related Steering

- [API contracts](../docs/steering/api-contracts.md)
- [Change impact](../docs/steering/change-impact.md)
- [Testing](../docs/steering/testing.md)
- [Delivery](../docs/steering/delivery.md)
