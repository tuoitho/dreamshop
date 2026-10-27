import { zodResolver } from "@hookform/resolvers/zod"
import { AlertCircle, LockKeyhole, ShieldCheck } from "lucide-react"
import { useState } from "react"
import { useForm } from "react-hook-form"
import { Navigate, useLocation, useNavigate } from "react-router-dom"
import { z } from "zod"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAuth } from "@/hooks/use-auth"
import { toClientError } from "@/lib/api"

const loginSchema = z.object({
  email: z.string().trim().min(1, "Vui lòng nhập email vận hành.").email("Email không hợp lệ."),
  password: z.string().min(1, "Vui lòng nhập mật khẩu."),
})

type LoginFormValues = z.infer<typeof loginSchema>

const DEV_ACCOUNTS = [
  { label: "Admin", email: "admin@example.com", password: "AdminPassword123!" },
  { label: "Customer", email: "customer1@gmail.com", password: "123456123456" },
] as const

export function LoginPage() {
  const { status, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [serverError, setServerError] = useState<string | null>(null)
  const isDevMode = import.meta.env.MODE === "development"
  const defaultAccount = isDevMode ? DEV_ACCOUNTS[0] : null

  const redirectTo =
    (location.state as { from?: string } | null)?.from &&
    (location.state as { from?: string }).from !== "/login"
      ? (location.state as { from: string }).from
      : "/"

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      email: defaultAccount ? defaultAccount.email : "",
      password: defaultAccount ? defaultAccount.password : "",
    },
  })

  const currentEmail = watch("email")
  const currentPassword = watch("password")

  if (status === "authenticated") {
    return <Navigate to={redirectTo} replace />
  }

  const onSubmit = async (values: LoginFormValues) => {
    setServerError(null)
    try {
      await login(values)
      navigate(redirectTo, { replace: true })
    } catch (error) {
      setServerError(toClientError(error).message)
    }
  }

  return (
    <div className="grid min-h-screen w-full bg-background lg:grid-cols-[1.05fr_1fr]">
      <div className="hidden flex-col justify-between border-r border-sidebar-border bg-sidebar p-12 text-sidebar-foreground lg:flex">
        <div className="flex items-center gap-3">
          <div className="flex size-10 items-center justify-center rounded-lg border border-emerald-500/30 bg-emerald-500/15 font-mono text-base font-semibold text-emerald-300">
            S
          </div>
          <div>
            <div className="text-base font-semibold text-white">S-Shop Admin</div>
            <div className="font-mono text-[11px] tracking-widest text-emerald-400 uppercase">
              Operator Control Center
            </div>
          </div>
        </div>

        <div className="max-w-md space-y-5">
          <div className="inline-flex items-center gap-2 rounded-full border border-emerald-500/30 bg-emerald-500/10 px-3 py-1 font-mono text-xs text-emerald-300">
            <ShieldCheck className="size-3.5" />
            <span>RBAC Enforced Console</span>
          </div>
          <h1 className="text-3xl font-semibold tracking-tight text-white">
            Điều hành tập trung sản phẩm, tồn kho, tài khoản và phân quyền.
          </h1>
          <p className="text-sm leading-relaxed text-sidebar-muted">
            Mọi thao tác quản trị được xác thực qua phiên JWT ngắn hạn trong bộ nhớ, đồng bộ trực
            tiếp với hệ thống phân quyền phân cấp RBAC và kho lưu trữ đối tượng S3/MinIO.
          </p>
        </div>

        <div className="flex items-center justify-between border-t border-sidebar-border pt-6 font-mono text-xs text-sidebar-muted">
          <span>S-Shop Commerce Platform</span>
          <span>API v1 · Kong Ingress</span>
        </div>
      </div>

      <div className="flex items-center justify-center p-6 sm:p-10">
        <div className="w-full max-w-md rounded-xl border border-border bg-card p-7 shadow-xs sm:p-8">
          <div className="mb-6 flex items-start justify-between gap-4">
            <div>
              <h2 className="text-xl font-semibold tracking-tight text-foreground">
                Đăng nhập điều hành
              </h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Sử dụng tài khoản quản trị hoặc vận hành viên được cấp quyền.
              </p>
            </div>
            <div className="flex size-10 shrink-0 items-center justify-center rounded-lg border border-border bg-muted text-primary">
              <LockKeyhole className="size-5" />
            </div>
          </div>

          {serverError && (
            <div
              role="alert"
              className="mb-5 flex items-start gap-2.5 rounded-lg border border-red-200 bg-red-50 px-3.5 py-3 text-xs font-medium text-red-900"
            >
              <AlertCircle className="mt-0.5 size-4 shrink-0 text-red-600" />
              <span>{serverError}</span>
            </div>
          )}

          {isDevMode && (
            <div className="mb-4 flex items-center gap-2 rounded-lg border border-dashed border-border bg-muted/40 px-3 py-2">
              <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                DEV
              </span>
              <div className="flex flex-wrap gap-1.5">
                {DEV_ACCOUNTS.map((account) => {
                  const selected =
                    currentEmail === account.email && currentPassword === account.password
                  return (
                    <Button
                      key={account.email}
                      type="button"
                      size="sm"
                      variant={selected ? "secondary" : "outline"}
                      aria-pressed={selected}
                      disabled={isSubmitting}
                      onClick={() => {
                        setValue("email", account.email, { shouldValidate: true })
                        setValue("password", account.password, { shouldValidate: true })
                        setServerError(null)
                      }}
                    >
                      {account.label}
                    </Button>
                  )
                })}
              </div>
            </div>
          )}

          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
            <div className="space-y-1.5">
              <Label htmlFor="login-email">Email vận hành</Label>
              <Input
                id="login-email"
                type="email"
                autoComplete="email"
                placeholder="admin@sshop.vn"
                aria-invalid={Boolean(errors.email)}
                {...register("email")}
              />
              {errors.email && (
                <p className="text-xs font-medium text-red-700">{errors.email.message}</p>
              )}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="login-password">Mật khẩu</Label>
              <Input
                id="login-password"
                type="password"
                autoComplete="current-password"
                placeholder="••••••••••••"
                aria-invalid={Boolean(errors.password)}
                {...register("password")}
              />
              {errors.password && (
                <p className="text-xs font-medium text-red-700">{errors.password.message}</p>
              )}
            </div>

            <Button type="submit" className="mt-2 w-full" size="lg" disabled={isSubmitting}>
              {isSubmitting ? "Đang xác thực…" : "Đăng nhập vào trung tâm"}
            </Button>
          </form>
        </div>
      </div>
    </div>
  )
}
