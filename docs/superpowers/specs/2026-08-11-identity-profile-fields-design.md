# Identity Profile and Administrative Address Design

## Summary

The authenticated profile page lets users manage contact information and a two-level Vietnamese shipping address: province/city, ward/commune, and address detail. The page is redesigned into responsive personal-information and shipping-address sections. Avatar remains out of scope.

Identity persists the profile fields, serves administrative divisions from a versioned local JSON file, and validates selected location codes. The React client loads location lists exclusively from Identity through Kong; it does not call an external administrative-divisions API at runtime.

## Goals

- Persist optional phone number, date of birth, gender, province code, ward code, and address detail.
- Extend `GET /users/me` and `PATCH /users/me` to read and update these fields.
- Provide a two-level administrative location API backed by local JSON for 34 provinces/cities and their wards/communes.
- Rework `/profile` into an accessible, responsive profile form with dependent province and ward selectors.
- Keep email readonly and leave the registration flow unchanged.
- Add regression coverage for profile updates, location validation, location APIs, migrations, and dependent selectors.

## Non-goals

- Avatar upload or URL.
- Editable email, password, roles, or account status.
- Changes to the registration form or register contract.
- District-level address selection.
- Runtime calls from the browser or backend to an external administrative-divisions API.
- Multiple shipping addresses or address-book management.

## Administrative Data

`identity-service` contains `src/main/resources/locations/vietnam-administrative-units.json`. The file is a versioned snapshot generated from the Vietnam Provinces Online API v2 and contains 34 province/city records. Each record includes its numeric `code`, display `name`, and ward/commune records with numeric `code` and display `name`.

The service loads this resource at startup into an immutable lookup. It exposes only this local snapshot at runtime:

- `GET /locations/provinces` returns the 34 province/city code-name pairs.
- `GET /locations/provinces/{provinceCode}/wards` returns ward/commune code-name pairs belonging to the requested province. An unknown province returns `404`.

Kong exposes these endpoints under `/api/v1/locations`. Updating administrative data is an explicit source-control update to the JSON snapshot and its tests.

## Data Model

The existing V3 profile fields remain for contact information. Migration `V4__migrate_profile_address_to_two_levels.sql` changes address persistence:

| Column | Type | Notes |
|---|---|---|
| `province_code` | `varchar(10)` | Nullable code from the 34-province JSON snapshot |
| `ward_code` | `varchar(10)` | Nullable ward/commune code from the JSON snapshot |
| `address_line` | `varchar(255)` | Optional detail line |

The migration adds `ward_code`, sets `province_code` to `NULL` for every existing profile, and drops the legacy `district` and `ward` columns. No location column is unique and no index is required.

`Gender` remains `MALE`, `FEMALE`, or `OTHER`. The profile aggregate stores the location codes and accepts a location only when the ward code belongs to the province code in the loaded administrative snapshot.

## HTTP Contract

`GET /users/me` and `PATCH /users/me` responses contain the new nullable profile fields alongside existing identity fields:

```json
{
  "user_id": "...",
  "email": "user@example.com",
  "full_name": "Nguyen Van A",
  "roles": ["CUSTOMER"],
  "phone": "0901234567",
  "date_of_birth": "1995-04-20",
  "gender": "MALE",
  "province_code": "79",
  "ward_code": "26560",
  "address_line": "12 Le Loi"
}
```

`PATCH /users/me` accepts any subset of mutable fields with these rules:

- `full_name` remains `@NotBlank` and `@Size(max = 100)`.
- `phone` is at most 20 characters and matches the Vietnamese mobile pattern.
- `date_of_birth` must be an ISO date in the past.
- `gender` is `MALE`, `FEMALE`, or `OTHER`.
- `province_code` and `ward_code` must be supplied together when either is changed. The selected ward must belong to the selected province. Invalid combinations return `400 VALIDATION_FAILED`.
- `null` preserves an existing optional value; an empty string clears it; a nonempty value replaces it. Clearing either location code clears both codes to keep the address consistent.

Caller identity comes only from `CallerContextPrincipal`; no request field selects another user. Missing or disabled callers continue to return `404 USER_NOT_FOUND`.

## Frontend

- `src/lib/types.ts` replaces `ward` and `district` with `ward_code` in profile types.
- `src/lib/locations.ts` calls the two location endpoints through the existing API client.
- `/profile` has personal-information and shipping-address sections. It uses a two-column layout on wide screens and a single column on narrow screens.
- Email is a readonly labeled field. Phone, date of birth, and gender retain their current validation and clear behavior.
- The province selector loads the 34 local-snapshot records from Identity. The ward selector is disabled until a province is selected, loads the selected province's wards, and resets whenever the province changes.
- Both selectors support searching, show loading and retry states, and submit codes rather than display names.
- The existing auth context keeps `updateProfile(payload)` and replaces its current user with the PATCH response.

## Testing and Acceptance Criteria

### Identity

- Migration tests verify `ward_code` is added, legacy `district` and `ward` columns are absent, and existing `province_code` values are cleared.
- Location-data tests verify the local JSON contains 34 unique provinces, has unique ward codes per province, and can be loaded at startup.
- Location API tests verify province and ward responses and `404` for an unknown province.
- `MeApiTest` verifies valid province-ward updates persist and return codes; mismatched or unknown location codes return `400 VALIDATION_FAILED`; omission preserves fields; empty location input clears both location codes.
- Existing tests continue to cover invalid phone, future date of birth, and unknown gender.

### Frontend

- Location client tests cover successful and failed province/ward loading.
- `profile-page.test.tsx` verifies readonly email, the dependent selector flow, ward reset when province changes, disabled ward selection before province selection, the expected code payload, and server-response state replacement.
- Tests cover location loading and retry feedback.

The slice is complete when Identity focused and module tests, frontend tests, lint, and build pass.
