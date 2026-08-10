import { useState, type FormEvent } from "react"
import { useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAuth } from "@/hooks/use-auth"

export default function ProfilePage() {
  const { user, updateProfile, logout } = useAuth()
  const navigate = useNavigate()
  const [fullName, setFullName] = useState(user?.full_name ?? "")
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await updateProfile({ full_name: fullName })
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Không thể cập nhật hồ sơ")
    } finally {
      setSubmitting(false)
    }
  }

  async function signOut() {
    await logout()
    navigate("/login", { replace: true })
  }

  if (!user) return null
  return <div className="flex min-h-svh items-center justify-center p-4"><Card className="w-full max-w-md"><CardHeader><CardTitle>Hồ sơ</CardTitle><CardDescription>{user.email}</CardDescription></CardHeader><CardContent className="grid gap-4"><p className="text-sm">Vai trò: {user.roles.join(", ")}</p><form className="grid gap-3" onSubmit={submit}><Label htmlFor="full-name">Họ và tên</Label><Input id="full-name" value={fullName} onChange={(event) => setFullName(event.target.value)} /><Button type="submit" disabled={submitting}>{submitting ? "Đang lưu..." : "Lưu thay đổi"}</Button></form>{error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}<Button variant="outline" onClick={() => void signOut()}>Đăng xuất</Button></CardContent></Card></div>
}
