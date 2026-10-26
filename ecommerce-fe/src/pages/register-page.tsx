import { useState, type FormEvent } from "react"
import { Link, Navigate, useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAuth } from "@/hooks/use-auth"

export default function RegisterPage() {
  const { register, ready, isAuthenticated } = useAuth()
  const navigate = useNavigate()
  const [fullName, setFullName] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  if (ready && isAuthenticated && !submitting) {
    return <Navigate to="/" replace />
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError(null)

    if (password.length < 12) {
      setError("Mật khẩu phải có ít nhất 12 ký tự")
      return
    }
    if (password !== confirmPassword) {
      setError("Mật khẩu xác nhận không khớp")
      return
    }

    setSubmitting(true)
    try {
      await register({ email, password, fullName })
      navigate("/login", { replace: true, state: { registered: true } })
    } catch (err) {
      setError(err instanceof Error ? err.message : "Đăng ký thất bại")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-[calc(100svh-12rem)] items-center justify-center px-4 py-12">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-xl">Đăng ký</CardTitle>
          <CardDescription>Tạo tài khoản mới</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="grid gap-4" onSubmit={handleSubmit}>
            <div className="grid gap-2">
              <Label htmlFor="fullName">Họ và tên</Label>
              <Input
                id="fullName"
                type="text"
                required
                autoComplete="name"
                placeholder="Nguyen Van A"
                value={fullName}
                onChange={(e) => setFullName(e.target.value)}
              />
            </div>
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
                autoComplete="new-password"
                placeholder="Ít nhất 12 ký tự"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="confirmPassword">Xác nhận mật khẩu</Label>
              <Input
                id="confirmPassword"
                type="password"
                required
                autoComplete="new-password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
              />
            </div>
            {error ? (
              <p className="text-sm text-destructive" role="alert">
                {error}
              </p>
            ) : null}
            <Button type="submit" disabled={submitting}>
              {submitting ? "Đang đăng ký..." : "Đăng ký"}
            </Button>
          </form>
          <p className="mt-4 text-center text-sm text-muted-foreground">
            Đã có tài khoản?{" "}
            <Link to="/login" className="font-medium text-foreground underline underline-offset-4">
              Đăng nhập
            </Link>
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
