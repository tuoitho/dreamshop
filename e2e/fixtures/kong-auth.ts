import { expect, test, type Page } from "@playwright/test"

export const DEFAULT_KONG_API_BASE = process.env.KONG_API_BASE_URL || "http://localhost:8000/api/v1"

export const DEFAULT_LOCAL_TEST_ACCOUNTS = {
  customer: {
    email: process.env.E2E_CUSTOMER_EMAIL || "customer1@gmail.com",
    password: process.env.E2E_CUSTOMER_PASSWORD || "123456123456",
  },
  admin: {
    email: process.env.E2E_ADMIN_EMAIL || "admin@example.com",
    password: process.env.E2E_ADMIN_PASSWORD || "AdminPassword123!",
  },
} as const

export interface KongHealthResult {
  reachable: boolean
  reason?: string
}

/**
 * Kiểm tra nhanh Kong gateway & 2 upstream công khai (Identity JWKS + BFF storefront products)
 * với timeout ngắn trước khi chạy Mode 1 live E2E.
 */
export async function ensureKongStackReachable(
  page: Page,
  apiBase = DEFAULT_KONG_API_BASE
): Promise<KongHealthResult> {
  const base = apiBase.replace(/\/+$/, "")
  try {
    const jwks = await page.request.get(`${base}/auth/.well-known/jwks.json`, { timeout: 3_000 })
    if (!jwks.ok()) {
      return { reachable: false, reason: `Identity JWKS via Kong returned HTTP ${jwks.status()}` }
    }
    const products = await page.request.get(`${base}/storefront/products?page=1&pageSize=1`, {
      timeout: 3_000,
    })
    if (!products.ok()) {
      return { reachable: false, reason: `BFF products via Kong returned HTTP ${products.status()}` }
    }
    return { reachable: true }
  } catch (error) {
    return {
      reachable: false,
      reason: error instanceof Error ? error.message : String(error),
    }
  }
}

/**
 * Fail immediately when running in CI or E2E_STRICT_LIVE=1; otherwise skip cleanly with diagnostic reason.
 */
export async function assertOrSkipKongStackReachable(
  page: Page,
  apiBase = DEFAULT_KONG_API_BASE
): Promise<void> {
  const health = await ensureKongStackReachable(page, apiBase)
  if (health.reachable) {
    return
  }
  const message = `Live Kong stack unavailable (${health.reason}). Start make infra-up and local services before running Mode 1.`
  if (process.env.E2E_STRICT_LIVE === "1" || Boolean(process.env.CI)) {
    throw new Error(message)
  }
  test.skip(true, message)
}

export interface KongLoginOptions {
  email?: string
  password?: string
  role?: "customer" | "admin"
  fullName?: string
  /**
   * Mặc định false theo root AGENTS.md (tái sử dụng tài khoản seeded, không tạo tài khoản tạm).
   * Chỉ bật true khi test nhắm đích danh registration hoặc cần cô lập đa người dùng.
   */
  autoRegisterCustomer?: boolean
}

/**
 * Bootstrap session cho Mode 1 qua Kong mà KHÔNG lưu accessToken vào localStorage.
 *
 * page.request dùng chung cookie jar với BrowserContext nên Set-Cookie: refresh_token (HttpOnly)
 * được trình duyệt tự động giữ lại. Khi mở ecommerce-fe hoặc admin-fe, AuthProvider tự gọi
 * POST /api/v1/auth/refresh để nạp accessToken vào memory đúng như người dùng thật.
 */
export async function loginViaKongCookieJar(
  page: Page,
  options: KongLoginOptions = {},
  apiBase = DEFAULT_KONG_API_BASE
): Promise<{ accessToken: string }> {
  const base = apiBase.replace(/\/+$/, "")
  const role = options.role ?? "customer"
  const defaults = DEFAULT_LOCAL_TEST_ACCOUNTS[role]
  const email = options.email ?? defaults.email
  const password = options.password ?? defaults.password

  let resp = await page.request.post(`${base}/auth/login`, {
    data: { email, password },
  })

  if (resp.status() === 401 && options.autoRegisterCustomer === true && role === "customer") {
    const reg = await page.request.post(`${base}/auth/register`, {
      data: {
        email,
        password,
        fullName: options.fullName ?? "E2E Customer",
      },
    })
    expect([201, 409]).toContain(reg.status())
    resp = await page.request.post(`${base}/auth/login`, {
      data: { email, password },
    })
  }

  expect(resp.ok(), `POST /auth/login via Kong failed with HTTP ${resp.status()}`).toBeTruthy()
  const body = (await resp.json()) as { data?: { accessToken?: string }; message?: unknown }
  expect(typeof body.message, "Success envelope must contain string message").toBe("string")
  expect(body.data?.accessToken, "Login response must contain data.accessToken").toBeTruthy()

  const cookies = await page.context().cookies(base)
  const refreshCookie = cookies.find((c) => c.name === "refresh_token")
  expect(refreshCookie, "Expected HttpOnly refresh_token cookie in BrowserContext").toBeDefined()
  expect(refreshCookie?.httpOnly).toBe(true)

  return { accessToken: body.data!.accessToken! }
}

/**
 * Xác nhận ứng dụng không ghi bất kỳ token/JWT nào vào localStorage hoặc sessionStorage.
 */
export async function assertNoTokenInWebStorage(page: Page): Promise<void> {
  const suspiciousKeys = await page.evaluate(() => {
    const hits: string[] = []
    for (const store of [window.localStorage, window.sessionStorage]) {
      for (let i = 0; i < store.length; i++) {
        const key = store.key(i)
        if (key && /token|jwt|auth|refresh/i.test(key)) {
          hits.push(key)
        }
      }
    }
    return hits
  })
  expect(suspiciousKeys, "No auth/JWT/refresh token may be stored in Web Storage").toEqual([])
}
