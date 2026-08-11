# Two-Level Profile Address Implementation Plan

**Goal:** Replace legacy district/ward profile addresses with validated province and ward codes from an Identity-hosted local administrative-data snapshot, and redesign the profile form around dependent selectors.

**Architecture:** Identity loads the 34-province JSON resource and exposes location endpoints through the existing `/api/v1` Kong route. V4 drops the legacy address columns, clears old province values, and introduces `ward_code`; frontend loads lists only from Identity and submits code pairs.

## Tasks

### 1. Identity address migration and catalog

**Files:** V4 migration; profile persistence, request/response/service mappings; `location` catalog/controller/error types; JSON resource; Identity migration, catalog, location API, and Me API tests.

1. Write failing focused tests for the V4 schema/data migration, catalog loading, location API, valid location update, invalid/mismatched pair, and atomic clearing.
2. Add the reviewed 34-province snapshot, immutable catalog, `GET /locations/provinces`, and `GET /locations/provinces/{code}/wards`; unknown province returns `404 NOT_FOUND`.
3. Add `ward_code`, drop `district`/`ward`, clear old `province_code`, and replace legacy profile mappings.
4. Validate nonempty province/ward pairs against the catalog after resolving PATCH values; retain existing null/empty semantics and required `full_name` behavior.
5. Run focused Identity tests and the Identity module suite.

### 2. Frontend dependent address and profile redesign

**Files:** `src/lib/types.ts`, `src/lib/api.ts`, location client/tests, profile page/tests, and removal of the legacy static province file.

1. Write failing tests for location client success/failure, disabled ward selector, province-change ward reset, code payload, retry feedback, and responsive profile form behavior.
2. Replace legacy address fields with `ward_code`; call Identity location endpoints through the existing API client.
3. Implement searchable dependent selectors with loading/error/retry states and redesign `/profile` as responsive personal-information and shipping-address sections.
4. Run focused frontend tests, all frontend tests, lint, and build.

### 3. Integration verification

1. Validate Docker compose configuration.
2. Run Identity focused/module tests and frontend gates.
3. Start foundation services, Identity on the Kong-configured upstream port, and Vite. Manually smoke the gateway location endpoints with an authenticated token.

**Known baseline:** `CallerContextFilterTest.logsAcceptedCallerUserId` is an unrelated existing failure. Do not modify it as part of this feature.
