// Reference implementation: Playwright Mode 2 shape-locked route helper cho microservices monorepo.
// Khi repo đã có e2e/fixtures/contract-routes.ts, import trực tiếp từ đó thay vì copy lại file này.
import * as fs from "node:fs"
import * as path from "node:path"
import { expect, type BrowserContext, type Page, type Request, type Route } from "@playwright/test"

export interface ContractSnapshotEntry {
  service: "bff-service" | "identity-service" | string
  endpoint: string
  status: number
  requestBody?: unknown
  responseBody: unknown
}

export interface ContractSnapshotDocument {
  domain?: string
  feature?: string
  snapshots: Record<string, ContractSnapshotEntry>
  nullable?: string[]
}

export interface ContractRouteRequest {
  request: Request
  url: URL
  body: unknown
  cookies: Record<string, string>
}

export interface ContractRouteReply {
  snapshot: string
  body?: unknown
  headers?: Record<string, string>
}

interface ContractRouteMatch {
  method: string
  path: string | RegExp
}

export interface ShapeLockedRoute extends ContractRouteMatch {
  /**
   * - 'bearer': yêu cầu header Authorization: Bearer <token>; nếu thiếu trả snapshot unauthenticated_401
   * - 'refreshCookie': yêu cầu cookie refresh_token; nếu thiếu trả snapshot auth_refresh_missing_401
   */
  auth?: "bearer" | "refreshCookie"
  /** Snapshot key chứa requestBody mẫu để đối chiếu payload FE gửi lên */
  requestSnapshot?: string
  /** Tên snapshot lỗi 401 tùy chỉnh nếu khác mặc định */
  unauthenticatedSnapshot?: string
  handle: (req: ContractRouteRequest) => ContractRouteReply | Promise<ContractRouteReply>
}

export interface StubReply {
  status: number
  body: unknown
  headers?: Record<string, string>
}

export interface StubRoute extends ContractRouteMatch {
  stub: StubReply | ((req: ContractRouteRequest) => StubReply | Promise<StubReply>)
}

export type ContractRoute = ShapeLockedRoute | StubRoute

const API_PATTERN = "**/api/v1/**"
const DEFAULT_UNAUTHENTICATED_SNAPSHOT = "unauthenticated_401"
const DEFAULT_MISSING_REFRESH_SNAPSHOT = "auth_refresh_missing_401"

/**
 * Nạp một hoặc nhiều file domain golden snapshot dưới contracts/http-snapshots/*.json
 * (ví dụ: ["../contracts/http-snapshots/identity-auth.json", "../contracts/http-snapshots/bff-orders.json"]).
 * Báo lỗi ngay nếu phát hiện trùng snapshot key giữa các file domain được ghép.
 */
export function loadContractSnapshots(
  snapshotFilePaths: string | string[],
  options: { nullable?: string[] } = {}
): ContractSnapshotDocument {
  const paths = Array.isArray(snapshotFilePaths) ? snapshotFilePaths : [snapshotFilePaths]
  if (paths.length === 0) {
    throw new Error("loadContractSnapshots requires at least one domain snapshot file path.")
  }

  const mergedSnapshots: Record<string, ContractSnapshotEntry> = {}
  const domains: string[] = []

  for (const filePath of paths) {
    const resolved = path.resolve(filePath)
    if (!fs.existsSync(resolved)) {
      throw new Error(
        `Missing golden contract snapshot file at ${resolved}. Generate it from BFF/Identity tests before running Mode 2 E2E.`
      )
    }
    const doc = JSON.parse(fs.readFileSync(resolved, "utf-8")) as ContractSnapshotDocument
    domains.push(doc.domain ?? doc.feature ?? path.basename(resolved, ".json"))

    for (const [key, entry] of Object.entries(doc.snapshots ?? {})) {
      if (key in mergedSnapshots) {
        throw new Error(
          `Duplicate snapshot key "${key}" detected while merging ${resolved}. Keep snapshot keys unique across domains.`
        )
      }
      mergedSnapshots[key] = entry
    }
  }

  return {
    domain: domains.join("+"),
    snapshots: mergedSnapshots,
    nullable: options.nullable ?? [],
  }
}

export function assertMatchesContractShape(
  actual: unknown,
  expected: unknown,
  currentPath = "$",
  nullableKeys: readonly string[] = []
): void {
  if (expected === null || actual === null) {
    expect(actual === null, `Null mismatch at ${currentPath}`).toBe(expected === null)
    return
  }

  if (Array.isArray(expected)) {
    expect(Array.isArray(actual), `${currentPath} must be an array`).toBe(true)
    const actualArr = actual as unknown[]
    if (expected.length > 0 && actualArr.length > 0) {
      for (let i = 0; i < actualArr.length; i++) {
        let lastError: unknown = null
        const candidates =
          i < expected.length
            ? [expected[i], ...expected.filter((_, idx) => idx !== i)]
            : expected
        let matched = false
        for (const candidate of candidates) {
          try {
            assertMatchesContractShape(actualArr[i], candidate, `${currentPath}[${i}]`, nullableKeys)
            matched = true
            break
          } catch (err) {
            lastError = err
          }
        }
        if (!matched && lastError) {
          throw lastError
        }
      }
    }
    return
  }

  if (expected && typeof expected === "object") {
    expect(
      actual !== null && typeof actual === "object" && !Array.isArray(actual),
      `${currentPath} must be an object`
    ).toBe(true)

    const expectedObj = expected as Record<string, unknown>
    const actualObj = actual as Record<string, unknown>
    const expectedKeys = Object.keys(expectedObj).sort()
    const actualKeys = Object.keys(actualObj).sort()

    expect(
      actualKeys,
      `Contract drift at ${currentPath}: key set mismatch between mock payload and golden snapshot`
    ).toEqual(expectedKeys)

    for (const key of expectedKeys) {
      const nullDiffers = (actualObj[key] === null) !== (expectedObj[key] === null)
      if (nullDiffers && nullableKeys.includes(key)) {
        continue
      }
      assertMatchesContractShape(actualObj[key], expectedObj[key], `${currentPath}.${key}`, nullableKeys)
    }
    return
  }

  expect(typeof actual, `Primitive type mismatch at ${currentPath}`).toBe(typeof expected)
}

function assertRequestMatchesRecorded(
  sent: unknown,
  recorded: unknown,
  currentPath: string,
  nullableKeys: readonly string[] = []
): void {
  if (recorded === null) {
    expect(sent === null, `Request null mismatch at ${currentPath}`).toBe(true)
    return
  }
  expect(sent, `Request payload at ${currentPath} must not be null or empty`).not.toBeNull()
  if (Array.isArray(recorded)) {
    expect(Array.isArray(sent), `${currentPath} must be an array`).toBe(true)
    if (recorded.length > 0) {
      for (const [index, item] of (sent as unknown[]).entries()) {
        assertRequestMatchesRecorded(item, recorded[0], `${currentPath}[${index}]`, nullableKeys)
      }
    }
    return
  }
  if (typeof recorded === "object") {
    expect(
      typeof sent === "object" && !Array.isArray(sent),
      `${currentPath} must be an object`
    ).toBe(true)
    const recordedObj = recorded as Record<string, unknown>
    for (const [key, value] of Object.entries(sent as Record<string, unknown>)) {
      expect(
        key in recordedObj,
        `Request key "${key}" at ${currentPath} is not in the recorded backend request`
      ).toBe(true)
      if (value === null && nullableKeys.includes(key)) {
        continue
      }
      assertRequestMatchesRecorded(value, recordedObj[key], `${currentPath}.${key}`, nullableKeys)
    }
    return
  }
  expect(typeof sent, `Request type mismatch at ${currentPath}`).toBe(typeof recorded)
}

function parseJsonBody(request: Request): unknown {
  try {
    return request.postDataJSON()
  } catch {
    return null
  }
}

function parseCookies(cookieHeader: string | undefined): Record<string, string> {
  if (!cookieHeader) return {}
  const result: Record<string, string> = {}
  for (const part of cookieHeader.split(";")) {
    const idx = part.indexOf("=")
    if (idx === -1) continue
    const key = part.slice(0, idx).trim()
    const val = part.slice(idx + 1).trim()
    if (key) result[key] = val
  }
  return result
}

function corsHeaders(request: Request): Record<string, string> {
  const origin = request.headers()["origin"]
  if (!origin) return {}
  return {
    "access-control-allow-origin": origin,
    "access-control-allow-credentials": "true",
    "access-control-allow-methods": "GET,POST,PUT,PATCH,DELETE,OPTIONS",
    "access-control-allow-headers":
      request.headers()["access-control-request-headers"] ||
      "Authorization,Content-Type,Idempotency-Key,X-Trace-Id",
    vary: "Origin",
  }
}

function failRunningTest(violation: string): void {
  expect.soft(violation, "Shape-locked mock violation").toBeUndefined()
}

async function fulfillSnapshot(
  route: Route,
  request: Request,
  snapshots: ContractSnapshotDocument,
  snapshotKey: string,
  overrideBody?: unknown,
  extraHeaders?: Record<string, string>
): Promise<void> {
  const entry = snapshots.snapshots[snapshotKey]
  expect(entry, `Unknown contract snapshot key "${snapshotKey}"`).toBeDefined()

  const body = overrideBody !== undefined ? overrideBody : structuredClone(entry.responseBody)
  assertMatchesContractShape(
    body,
    entry.responseBody,
    `snapshot(${snapshotKey}).responseBody`,
    snapshots.nullable
  )

  await route.fulfill({
    status: entry.status,
    contentType: "application/json",
    headers: {
      ...corsHeaders(request),
      ...extraHeaders,
    },
    body: body === null ? "" : JSON.stringify(body),
  })
}

/**
 * Điểm đăng ký duy nhất cho mọi mocked API route (/api/v1/**) trong Mode 2.
 * Hỗ trợ cross-origin CORS + credentials cho :5173/:5174 -> http://localhost:8000/api/v1.
 */
export async function mountContractRoutes(
  target: Page | BrowserContext,
  snapshots: ContractSnapshotDocument,
  routes: ContractRoute[],
  report: (violation: string) => void = failRunningTest
): Promise<void> {
  await target.route(API_PATTERN, async (route) => {
    const request = route.request()

    if (request.method() === "OPTIONS") {
      await route.fulfill({
        status: 204,
        headers: corsHeaders(request),
      })
      return
    }

    const url = new URL(request.url())
    const label = `${request.method()} ${url.pathname}`
    const declared = routes.find(
      (candidate) =>
        candidate.method === request.method() &&
        (typeof candidate.path === "string"
          ? candidate.path === url.pathname
          : candidate.path.test(url.pathname))
    )

    if (!declared) {
      report(`Unmocked API request: ${label}`)
      await route.abort()
      return
    }

    try {
      const body = parseJsonBody(request)
      const allHeaders = await request.allHeaders()
      const cookies = parseCookies(allHeaders["cookie"] ?? request.headers()["cookie"])

      if ("stub" in declared) {
        const reply =
          typeof declared.stub === "function"
            ? await declared.stub({ request, url, body, cookies })
            : declared.stub
        await route.fulfill({
          status: reply.status,
          contentType: "application/json",
          headers: {
            ...corsHeaders(request),
            ...reply.headers,
          },
          body: JSON.stringify(reply.body),
        })
        return
      }

      if (declared.auth === "bearer") {
        const authorization = allHeaders["authorization"] ?? request.headers()["authorization"] ?? ""
        if (!/^Bearer .+/.test(authorization)) {
          await fulfillSnapshot(
            route,
            request,
            snapshots,
            declared.unauthenticatedSnapshot ?? DEFAULT_UNAUTHENTICATED_SNAPSHOT
          )
          return
        }
      }

      if (declared.auth === "refreshCookie") {
        if (!cookies["refresh_token"]) {
          await fulfillSnapshot(
            route,
            request,
            snapshots,
            declared.unauthenticatedSnapshot ?? DEFAULT_MISSING_REFRESH_SNAPSHOT
          )
          return
        }
      }

      if (declared.requestSnapshot) {
        const recorded = snapshots.snapshots[declared.requestSnapshot]
        expect(
          recorded?.requestBody,
          `Snapshot "${declared.requestSnapshot}" has no recorded requestBody`
        ).toBeDefined()
        assertRequestMatchesRecorded(body, recorded.requestBody, "requestBody", snapshots.nullable)
      }

      const reply = await declared.handle({ request, url, body, cookies })
      await fulfillSnapshot(route, request, snapshots, reply.snapshot, reply.body, reply.headers)
    } catch (error) {
      report(`${label}: ${error instanceof Error ? error.message : String(error)}`)
      await route.abort()
    }
  })
}

export interface RecordedPresignedUpload {
  url: string
  contentType: string
  size: number
}

/**
 * Helper riêng cho admin-fe direct MinIO presigned PUT upload (ADR-012):
 * - Bắt buộc có Content-Type hợp lệ và KHÔNG gắn Authorization header trên PUT.
 * - Lưu binary đã PUT trong bộ nhớ và phục vụ lại khi trình duyệt GET/HEAD URL công khai
 *   để <img> decode thành công (naturalWidth > 0) trong Mode 2 không cần Docker/MinIO.
 */
export async function mountPresignedUploadTrap(
  target: Page | BrowserContext,
  options: { urlPattern?: string; allowedContentTypes?: string[] } = {},
  report: (violation: string) => void = failRunningTest
): Promise<RecordedPresignedUpload[]> {
  const uploads: RecordedPresignedUpload[] = []
  const objectStore = new Map<string, { body: Buffer; contentType: string }>()
  const pattern = options.urlPattern ?? "**/product-media/**"

  await target.route(pattern, async (route) => {
    const request = route.request()
    const key = new URL(request.url()).pathname

    if (request.method() === "OPTIONS") {
      await route.fulfill({ status: 204, headers: corsHeaders(request) })
      return
    }

    if (request.method() === "GET" || request.method() === "HEAD") {
      const stored = objectStore.get(key)
      if (!stored) {
        await route.fulfill({ status: 404, headers: corsHeaders(request), body: "" })
        return
      }
      await route.fulfill({
        status: 200,
        contentType: stored.contentType,
        headers: {
          ...corsHeaders(request),
          "cache-control": "no-store",
        },
        body: request.method() === "HEAD" ? Buffer.alloc(0) : stored.body,
      })
      return
    }

    if (request.method() !== "PUT") {
      await route.continue()
      return
    }

    if (request.headers()["authorization"]) {
      report(`ADR-012 violation: presigned PUT to ${request.url()} must not include Authorization header`)
      await route.abort()
      return
    }

    const contentType = request.headers()["content-type"] ?? ""
    if (!contentType) {
      report(`ADR-012 violation: presigned PUT to ${request.url()} is missing Content-Type header`)
      await route.abort()
      return
    }
    if (options.allowedContentTypes && !options.allowedContentTypes.includes(contentType)) {
      report(`Unexpected Content-Type "${contentType}" on presigned PUT to ${request.url()}`)
      await route.abort()
      return
    }

    const buf = request.postDataBuffer() ?? Buffer.alloc(0)
    objectStore.set(key, { body: buf, contentType })
    uploads.push({ url: request.url(), contentType, size: buf.byteLength })
    await route.fulfill({
      status: 200,
      headers: {
        ...corsHeaders(request),
        etag: '"e2e-mock-etag"',
      },
    })
  })

  return uploads
}
