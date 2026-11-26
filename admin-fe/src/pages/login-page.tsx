import { zodResolver } from "@hookform/resolvers/zod"
import {
  AlertCircle,
  ArrowRight,
  Boxes,
  Eye,
  EyeOff,
  KeyRound,
  Loader2,
  LockKeyhole,
  Package,
  ShieldCheck,
  Users,
} from "lucide-react"
import { useState } from "react"
import { useForm } from "react-hook-form"
import { Navigate, useLocation, useNavigate } from "react-router-dom"
import { z } from "zod"
import { BrandLogo } from "@/components/layout/brand-logo"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAuth } from "@/hooks/use-auth"
import { resolveErrorMessage } from "@/lib/error-messages"
import { cn } from "@/lib/utils"

const loginSchema = z.object({
  email: z.string().trim().min(1, "Vui lòng nhập email vận hành.").email("Email không hợp lệ."),
  password: z.string().min(1, "Vui lòng nhập mật khẩu."),
})

type LoginFormValues = z.infer<typeof loginSchema>

const DEV_ACCOUNTS = [
  { label: "Admin", email: "admin@example.com", password: "AdminPassword123!" },
  { label: "Customer", email: "customer1@gmail.com", password: "123456123456" },
] as const

const CONSOLE_MODULES = [
  {
    code: "CAT",
    title: "Danh mục & Media",
    description: "Quản lý sản phẩm, giá bán và thư viện ảnh lưu trữ S3/MinIO.",
    icon: Package,
  },
  {
    code: "INV",
    title: "Kho vận & Tồn kho",
    description: "Theo dõi tồn khả dụng, giữ chỗ đơn hàng và điều chỉnh nhập xuất.",
    icon: Boxes,
  },
  {
    code: "IAM",
    title: "Tài khoản & RBAC",
    description: "Phân cấp vai trò quản trị và kiểm soát phạm vi quyền theo nghiệp vụ.",
    icon: Users,
  },
  {
    code: "SEC",
    title: "Phiên xác thực",
    description: "Kiểm soát truy cập tập trung qua cổng API và chính sách quyền hiệu lực.",
    icon: KeyRound,
  },
] as const

export function LoginPage() {
  const { status, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [serverError, setServerError] = useState<string | null>(null)
  const [showPassword, setShowPassword] = useState(false)
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
      setServerError(resolveErrorMessage(error))
    }
  }

  return (
    <div className="grid min-h-screen w-full bg-background lg:grid-cols-[1.08fr_1fr]">
      <aside className="relative hidden flex-col justify-between overflow-hidden border-r border-zinc-800/80 bg-[#0d1117] p-9 text-zinc-100 xl:px-12 xl:py-10 lg:flex">
        {/* Subtle architectural grid and ambient teal glow */}
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 opacity-[0.07]"
          style={{
            backgroundImage:
              "linear-gradient(to right, #94a3b8 1px, transparent 1px), linear-gradient(to bottom, #94a3b8 1px, transparent 1px)",
            backgroundSize: "40px 40px",
          }}
        />
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -top-28 -left-24 size-[420px] rounded-full bg-teal-500/15 blur-[110px]"
        />
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -right-24 bottom-0 size-[340px] rounded-full bg-emerald-500/10 blur-[100px]"
        />

        {/* Brand header */}
        <div className="relative z-10 flex items-center justify-between gap-4">
          <div className="flex items-center gap-3.5">
            <div className="rounded-xl ring-1 ring-white/15">
              <BrandLogo className="size-10" />
            </div>
            <div>
              <div className="font-display text-base font-bold tracking-tight text-white">
                DreamShop Admin
              </div>
              <div className="font-mono text-[11px] font-medium tracking-[0.16em] text-teal-400 uppercase">
                Operator Control Center
              </div>
            </div>
          </div>

          <div className="inline-flex items-center gap-2 rounded-full border border-teal-400/25 bg-teal-400/10 px-3 py-1 font-mono text-[11px] font-medium text-teal-300">
            <span className="size-1.5 rounded-full bg-teal-400" />
            <span>RBAC Enforced</span>
          </div>
        </div>

        {/* Hero thesis & module matrix */}
        <div className="relative z-10 my-auto max-w-xl space-y-6 py-4">
          <div className="space-y-3.5">
            <div className="inline-flex items-center gap-2 font-mono text-xs font-medium tracking-wider text-zinc-400 uppercase">
              <ShieldCheck className="size-4 text-teal-400" />
              <span>Không gian vận hành thương mại</span>
            </div>
            <h1 className="font-display text-[1.95rem] leading-[1.2] font-bold tracking-tight text-white xl:text-[2.1rem]">
              Điều hành sản phẩm, tồn kho và phân quyền tập trung.
            </h1>
            <p className="max-w-lg text-sm leading-relaxed text-zinc-400">
              Cổng quản trị dành riêng cho đội ngũ vận hành DreamShop. Kiểm soát danh mục, tồn kho
              và phân quyền người dùng với độ chính xác cao.
            </p>
          </div>

          <div className="grid grid-cols-2 gap-3">
            {CONSOLE_MODULES.map((item) => {
              const Icon = item.icon
              return (
                <div
                  key={item.code}
                  className="rounded-xl border border-white/[0.08] bg-white/[0.03] p-3.5 backdrop-blur-xs transition-colors hover:border-teal-400/25 hover:bg-white/[0.05]"
                >
                  <div className="mb-2 flex items-center justify-between">
                    <div className="flex size-8 items-center justify-center rounded-lg border border-white/10 bg-zinc-900/90 text-teal-400">
                      <Icon className="size-4" />
                    </div>
                    <span className="font-mono text-[10px] font-semibold tracking-widest text-zinc-500">
                      {item.code}
                    </span>
                  </div>
                  <div className="text-sm font-semibold text-zinc-100">{item.title}</div>
                  <p className="mt-1 text-xs leading-relaxed text-zinc-400">{item.description}</p>
                </div>
              )
            })}
          </div>
        </div>

        {/* Footer metadata */}
        <div className="relative z-10 flex items-center justify-between border-t border-white/10 pt-4 font-mono text-xs text-zinc-500">
          <span>DreamShop Commerce Platform</span>
          <span>API v1 · Kong Ingress</span>
        </div>
      </aside>

      {/* Right authentication workspace */}
      <main className="relative flex flex-col items-center justify-center p-6 sm:p-10">
        <div className="w-full max-w-[420px]">
          {/* Mobile / Tablet brand header */}
          <div className="mb-8 flex items-center justify-center gap-3 lg:hidden">
            <BrandLogo className="size-10" />
            <div>
              <div className="font-display text-base font-bold tracking-tight text-foreground">
                DreamShop Admin
              </div>
              <div className="font-mono text-[11px] font-medium tracking-[0.14em] text-teal-700 uppercase">
                Operator Control Center
              </div>
            </div>
          </div>

          <div className="rounded-2xl border border-border bg-card p-7 shadow-[0_1px_2px_rgba(16,24,40,0.04),0_12px_32px_-8px_rgba(16,24,40,0.08)] sm:p-9">
            <div className="mb-7 flex items-start justify-between gap-4">
              <div>
                <h2 className="font-display text-[1.35rem] font-bold tracking-tight text-foreground">
                  Đăng nhập điều hành
                </h2>
                <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
                  Sử dụng tài khoản quản trị hoặc vận hành viên được cấp quyền.
                </p>
              </div>
              <div className="flex size-10 shrink-0 items-center justify-center rounded-xl border border-border bg-muted/70 text-foreground">
                <LockKeyhole className="size-4.5" />
              </div>
            </div>

            {serverError && (
              <div
                role="alert"
                className="mb-5 flex items-start gap-2.5 rounded-xl border border-red-200 bg-red-50 px-3.5 py-3 text-xs font-medium text-red-900"
              >
                <AlertCircle className="mt-0.5 size-4 shrink-0 text-red-600" />
                <span>{serverError}</span>
              </div>
            )}

            {isDevMode && (
              <div className="mb-5 flex items-center justify-between gap-3 rounded-xl border border-border bg-muted/50 px-3 py-2">
                <span className="font-mono text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
                  DEV Preset
                </span>
                <div className="flex items-center gap-1.5">
                  {DEV_ACCOUNTS.map((account) => {
                    const selected =
                      currentEmail === account.email && currentPassword === account.password
                    return (
                      <Button
                        key={account.email}
                        type="button"
                        size="sm"
                        variant={selected ? "default" : "outline"}
                        aria-pressed={selected}
                        disabled={isSubmitting}
                        className={cn(
                          "h-7 rounded-lg px-2.5 text-xs",
                          !selected && "bg-card text-muted-foreground hover:text-foreground"
                        )}
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

            <form onSubmit={handleSubmit(onSubmit)} className="space-y-4.5" noValidate>
              <div className="space-y-1.5">
                <Label htmlFor="login-email" className="text-xs font-semibold text-foreground">
                  Email vận hành
                </Label>
                <Input
                  id="login-email"
                  type="email"
                  autoComplete="email"
                  placeholder="admin@dreamshop.vn"
                  className="h-10.5 rounded-xl px-3.5"
                  aria-invalid={Boolean(errors.email)}
                  {...register("email")}
                />
                {errors.email && (
                  <p className="text-xs font-medium text-red-700">{errors.email.message}</p>
                )}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="login-password" className="text-xs font-semibold text-foreground">
                  Mật khẩu
                </Label>
                <div className="relative">
                  <Input
                    id="login-password"
                    type={showPassword ? "text" : "password"}
                    autoComplete="current-password"
                    placeholder="••••••••••••"
                    className="h-10.5 rounded-xl pr-10 pl-3.5"
                    aria-invalid={Boolean(errors.password)}
                    {...register("password")}
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword((prev) => !prev)}
                    aria-label={showPassword ? "Ẩn ký tự" : "Hiển thị ký tự"}
                    className="absolute inset-y-0 right-1 flex items-center justify-center rounded-lg px-2.5 text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none"
                  >
                    {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                  </button>
                </div>
                {errors.password && (
                  <p className="text-xs font-medium text-red-700">{errors.password.message}</p>
                )}
              </div>

              <Button
                type="submit"
                className="mt-2 h-11 w-full rounded-xl font-semibold"
                size="lg"
                disabled={isSubmitting}
              >
                {isSubmitting ? (
                  <>
                    <Loader2 className="size-4 animate-spin" />
                    <span>Đang xác thực…</span>
                  </>
                ) : (
                  <>
                    <span>Đăng nhập vào trung tâm</span>
                    <ArrowRight className="size-4" />
                  </>
                )}
              </Button>
            </form>
          </div>

          <p className="mt-5 text-center text-xs text-muted-foreground">
            Chỉ dành cho nhân sự vận hành được cấp quyền truy cập.
          </p>
        </div>
      </main>
    </div>
  )
}
