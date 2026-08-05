import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useAuth } from "@/hooks/use-auth"
import { fetchHealth } from "@/lib/api"

export default function DashboardPage() {
  const { token, logout } = useAuth()
  const navigate = useNavigate()
  const [health, setHealth] = useState<{ status: string } | null>(null)
  const [healthError, setHealthError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    fetchHealth()
      .then((data) => {
        if (!cancelled) setHealth(data)
      })
      .catch((err: Error) => {
        if (!cancelled) setHealthError(err.message)
      })
    return () => {
      cancelled = true
    }
  }, [token])

  function handleLogout() {
    logout()
    navigate("/login", { replace: true })
  }

  return (
    <div className="flex min-h-svh flex-col items-center justify-center gap-6 p-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle className="text-xl">Dashboard</CardTitle>
          <CardDescription>Bạn đã đăng nhập thành công qua Kong &amp; Identity Service</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4">
          <div className="grid gap-1 rounded-lg border p-3">
            <p className="text-sm text-muted-foreground">Access token (JWT)</p>
            <p className="break-all font-mono text-xs" aria-label="access-token">
              {token}
            </p>
          </div>
          <div className="grid gap-1 rounded-lg border p-3">
            <p className="text-sm text-muted-foreground">GET /auth/actuator/health (Bearer token)</p>
            {health ? (
              <p className="text-sm font-medium" aria-label="health-status">
                Status: {health.status}
              </p>
            ) : healthError ? (
              <p className="text-sm text-destructive" role="alert">
                {healthError}
              </p>
            ) : (
              <p className="text-sm text-muted-foreground">Đang kiểm tra...</p>
            )}
          </div>
          <Button variant="outline" onClick={handleLogout}>
            Đăng xuất
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
