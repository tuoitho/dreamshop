# Identity Profile Fields Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let authenticated users read and update optional phone, birth date, gender, and shipping-address fields through `/users/me` and the `/me` page.

**Architecture:** Keep profile information in the existing `identity_profiles` one-to-one entity and extend its existing `GET/PATCH /users/me` mapping chain. The frontend retains the current auth context and PATCH endpoint; it adds controlled fields that submit the complete mutable profile, using empty strings to clear optional strings.

**Tech Stack:** Java 21, Spring Boot, JPA/Hibernate, Flyway, JUnit 5/MockMvc/Testcontainers, React, TypeScript, Vite, Vitest, Testing Library, Radix Select.

---

### Task 1: Profile Schema and Persistence

**Files:**
- Create: `services/identity-service/src/main/resources/db/migration/V3__add_profile_contact_fields.sql`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/domain/Gender.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityProfileEntity.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityUserEntity.java`
- Test: `services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java`

- [ ] **Step 1: Add failing schema assertions for the seven nullable profile columns**

Assert `identity_profiles` has `phone varchar(20)`, `date_of_birth date`, `gender varchar(16)`, `province_code varchar(10)`, `district varchar(100)`, `ward varchar(100)`, and `address_line varchar(255)` using the existing JDBC metadata test style.

- [ ] **Step 2: Run the migration test to verify it fails**

Run: `mvn -pl services/identity-service -am -Dtest=IdentityDatabaseMigrationTest -Dsurefire.failIfNoSpecifiedTests=false test`

Expected: FAIL because the V3 migration and columns do not exist.

- [ ] **Step 3: Add the migration, enum, and nullable JPA mappings**

Create the migration with these statements:

```sql
alter table identity_profiles add column phone varchar(20);
alter table identity_profiles add column date_of_birth date;
alter table identity_profiles add column gender varchar(16);
alter table identity_profiles add column province_code varchar(10);
alter table identity_profiles add column district varchar(100);
alter table identity_profiles add column ward varchar(100);
alter table identity_profiles add column address_line varchar(255);
```

Create `Gender` with `MALE`, `FEMALE`, and `OTHER`. Map it with `@Enumerated(EnumType.STRING)`. Add a profile mutation method that retains an optional field when its request value is `null`, turns a provided blank string into `null`, and replaces a provided nonblank value. Have `IdentityUserEntity` delegate its aggregate profile update and set `updatedAt`.

- [ ] **Step 4: Run the migration test to verify it passes**

Run: `mvn -pl services/identity-service -am -Dtest=IdentityDatabaseMigrationTest -Dsurefire.failIfNoSpecifiedTests=false test`

Expected: PASS.

### Task 2: Identity Me API Contract

**Files:**
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/api/UpdateMeRequest.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/api/MeResponse.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/api/MeController.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/application/CurrentUser.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/application/MeService.java`
- Test: `services/identity-service/src/test/java/com/ecommerce/identity/user/api/MeApiTest.java`

- [ ] **Step 1: Add failing MockMvc tests for profile fields and validation**

Add cases that PATCH all fields and verify the response and a subsequent GET, omit one optional field while retaining its stored value, clear an optional string with `""`, and return `400 VALIDATION_FAILED` for an invalid Vietnamese mobile number, future `date_of_birth`, and unknown `gender`. Continue sending valid `full_name` in PATCH tests because it remains required.

- [ ] **Step 2: Run the focused API test to verify it fails**

Run: `mvn -pl services/identity-service -am -Dtest=MeApiTest -Dsurefire.failIfNoSpecifiedTests=false test`

Expected: FAIL because the current request, response, and service expose only `full_name`.

- [ ] **Step 3: Extend request, application, and response mappings**

Use snake-case `@JsonProperty` fields in `UpdateMeRequest`: `phone`, `date_of_birth`, `gender`, `province_code`, `district`, `ward`, and `address_line`. Validate maximum lengths from the migration, `@Past LocalDate dateOfBirth`, and phone with `@Pattern(regexp = "^(?:\\+84|0)(?:3|5|7|8|9)\\d{8}$")`. Keep `fullName` as `@NotBlank @Size(max = 100)`.

Thread these values through `MeController`, `MeService`, `CurrentUser`, and `MeResponse`, preserving caller identity exclusively from `CallerContextPrincipal`. Do not change the existing exception handling: invalid enum JSON must continue through the invalid-request handler as `VALIDATION_FAILED`.

- [ ] **Step 4: Run the focused API test to verify it passes**

Run: `mvn -pl services/identity-service -am -Dtest=MeApiTest -Dsurefire.failIfNoSpecifiedTests=false test`

Expected: PASS.

### Task 3: Frontend Types and Province Data

**Files:**
- Modify: `ecommerce-fe/src/lib/types.ts`
- Create: `ecommerce-fe/src/data/provinces.ts`

- [ ] **Step 1: Extend profile client types**

Add `phone`, `date_of_birth`, `gender`, `province_code`, `district`, `ward`, and `address_line` to `CurrentUser` as nullable values. Add the same fields to `UpdateProfileRequest` as strings, so empty form controls can intentionally clear values.

- [ ] **Step 2: Add the fixed province source**

Export the complete fixed 63-item GSO province list as `{ code: string; name: string }` entries. Store each GSO code as the select value and show its Vietnamese name.

### Task 4: Profile Form

**Files:**
- Modify: `ecommerce-fe/src/pages/profile-page.tsx`
- Test: `ecommerce-fe/src/pages/profile-page.test.tsx`

- [ ] **Step 1: Write a failing profile form test**

Create a full `CurrentUser` fixture and test that email is a readonly input. Fill every editable field, choose gender and province through their Radix controls, submit, and expect:

```ts
{
  full_name: "Updated Name",
  phone: "0901234567",
  date_of_birth: "1995-04-20",
  gender: "MALE",
  province_code: "79",
  district: "Quan 1",
  ward: "Phuong Ben Nghe",
  address_line: "12 Le Loi",
}
```

- [ ] **Step 2: Run the focused frontend test to verify it fails**

Run: `npm --prefix ecommerce-fe test -- profile-page.test.tsx`

Expected: FAIL because the form renders only `full_name`.

- [ ] **Step 3: Implement the controlled profile form**

Initialize every nullable current-user field as `""`; render labels and controls for Họ và tên, Email, Số điện thoại, Ngày sinh, Giới tính, Tỉnh/thành phố, Quận/huyện, Phường/xã, and Địa chỉ chi tiết. Render email with `readOnly`. Use `type="date"` for birth date and existing Radix `Select` components for gender and province. Submit all mutable fields to the unchanged `updateProfile(payload)` context method.

- [ ] **Step 4: Run the focused frontend test to verify it passes**

Run: `npm --prefix ecommerce-fe test -- profile-page.test.tsx`

Expected: PASS.

### Task 5: Surface Verification

**Files:**
- Verify only; no source changes expected.

- [ ] **Step 1: Run the complete Identity test suite**

Run: `mvn -pl services/identity-service -am test`

Expected: PASS.

- [ ] **Step 2: Run frontend test, lint, and production build**

Run: `npm --prefix ecommerce-fe run test`

Expected: PASS.

Run: `npm --prefix ecommerce-fe run lint`

Expected: PASS.

Run: `npm --prefix ecommerce-fe run build`

Expected: PASS.
