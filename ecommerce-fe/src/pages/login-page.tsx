import { useState, type FormEvent } from "react"
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAuth } from "@/hooks/use-auth"
import { isCustomerOnlyRoleSet } from "@/lib/auth-session"
import { resolveErrorMessage } from "@/lib/error-messages"

const DEV_ACCOUNTS = [
  { label: "Customer", email: "customer1@gmail.com", password: "123456123456" },
] as const

interface LoginLocationState {
  from?: string | { pathname: string; search?: string; hash?: string }
  registered?: boolean
}

function resolveDestination(state: unknown): string {
  const fromState = (state as LoginLocationState | null)?.from
  const candidate =
    typeof fromState === "string"
      ? fromState
      : fromState
        ? `${fromState.pathname}${fromState.search ?? ""}${fromState.hash ?? ""}`
        : "/"
  return candidate.startsWith("/") && !candidate.startsWith("//") ? candidate : "/"
}

export default function LoginPage() {
  const { login, user, ready, isAuthenticated } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const isDevMode = import.meta.env.MODE === "development"
  const [email, setEmail] = useState<string>(isDevMode ? DEV_ACCOUNTS[0].email : "")
  const [password, setPassword] = useState<string>(isDevMode ? DEV_ACCOUNTS[0].password : "")
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const destination = resolveDestination(location.state)
  const justRegistered = Boolean((location.state as LoginLocationState | null)?.registered)
  const hasCustomerAccess = isAuthenticated && (user === null || isCustomerOnlyRoleSet(user.roles))

  if (ready && hasCustomerAccess && !submitting) {
    return <Navigate to={destination} replace />
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await login({ email, password })
      navigate(destination, { replace: true })
    } catch (err) {
      setError(resolveErrorMessage(err, { fallback: "Đăng nhập thất bại. Vui lòng thử lại." }))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-[calc(100svh-12rem)] items-center justify-center px-4 py-12">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-xl">Đăng nhập</CardTitle>
          <CardDescription>Đăng nhập vào tài khoản của bạn</CardDescription>
        </CardHeader>
        <CardContent>
          {justRegistered ? (
            <div
              role="status"
              className="mb-4 rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700 dark:text-emerald-300"
            >
              Đăng ký thành công! Vui lòng đăng nhập để tiếp tục.
            </div>
          ) : null}
          {isDevMode ? (
            <div className="mb-4 flex items-center gap-2 rounded-lg border border-dashed border-border bg-muted/40 px-3 py-2">
              <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">DEV</span>
              <div className="flex flex-wrap gap-1.5">
                {DEV_ACCOUNTS.map((account) => {
                  const selected = email === account.email && password === account.password
                  return (
                    <Button
                      key={account.email}
                      type="button"
                      size="sm"
                      variant={selected ? "secondary" : "outline"}
                      aria-pressed={selected}
                      disabled={submitting}
                      onClick={() => {
                        setEmail(account.email)
                        setPassword(account.password)
                        setError(null)
                      }}
                    >
                      {account.label}
                    </Button>
                  )
                })}
              </div>
            </div>
          ) : null}
          <form className="grid gap-4" onSubmit={handleSubmit}>
            <div className="grid gap-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                required
                autoComplete="email"
                placeholder="you@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="password">Mật khẩu</Label>
              <Input
                id="password"
                type="password"
                required
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            {error ? (
              <p className="text-sm text-destructive" role="alert">
                {error}
              </p>
            ) : null}
            <Button type="submit" disabled={submitting}>
              {submitting ? "Đang đăng nhập..." : "Đăng nhập"}
            </Button>
          </form>
          <p className="mt-4 text-center text-sm text-muted-foreground">
            Chưa có tài khoản?{" "}
            <Link to="/register" className="font-medium text-foreground underline underline-offset-4">
              Đăng ký
            </Link>
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
