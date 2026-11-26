import { AlertCircle, Check, Loader2, Lock, Shield, Unlock, UserCheck } from "lucide-react"
import { useEffect, useState } from "react"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { useAuth } from "@/hooks/use-auth"
import {
  assignAdminUserRole,
  getAdminUser,
  revokeAdminUserRole,
  updateAdminUserStatus,
} from "@/lib/api"
import { resolveErrorMessage } from "@/lib/error-messages"
import { formatDateTime } from "@/lib/format"
import type { AdminUser, RbacItem } from "@/lib/types"

interface UserDetailSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: string | null
  availableRoles: RbacItem[]
  canUpdateStatus: boolean
  canAssignRole: boolean
  onMutated: () => void
}

const FALLBACK_ROLES: RbacItem[] = [
  {
    name: "SUPER_ADMIN",
    type: "ROLE",
    description: "Quản trị tối cao hệ thống",
    systemItem: true,
    directChildren: [],
    createdAt: "",
    updatedAt: "",
  },
  {
    name: "ADMIN",
    type: "ROLE",
    description: "Quản trị viên vận hành",
    systemItem: true,
    directChildren: [],
    createdAt: "",
    updatedAt: "",
  },
  {
    name: "CUSTOMER",
    type: "ROLE",
    description: "Tài khoản mua sắm tiêu chuẩn",
    systemItem: true,
    directChildren: [],
    createdAt: "",
    updatedAt: "",
  },
]

export function UserDetailSheet({
  open,
  onOpenChange,
  userId,
  availableRoles,
  canUpdateStatus,
  canAssignRole,
  onMutated,
}: UserDetailSheetProps) {
  const { user: currentUser, refreshPermissions } = useAuth()
  const [detail, setDetail] = useState<AdminUser | null>(null)
  const [loading, setLoading] = useState(false)
  const [busyAction, setBusyAction] = useState<string | null>(null)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [confirmStatusOpen, setConfirmStatusOpen] = useState(false)

  useEffect(() => {
    if (!open || !userId) {
      setDetail(null)
      setErrorMessage(null)
      return
    }

    const controller = new AbortController()
    setLoading(true)
    setErrorMessage(null)
    void (async () => {
      try {
        const res = await getAdminUser(userId, controller.signal)
        if (!controller.signal.aborted) {
          setDetail(res)
        }
      } catch (error) {
        if (!controller.signal.aborted) {
          setErrorMessage(resolveErrorMessage(error))
        }
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false)
        }
      }
    })()

    return () => controller.abort()
  }, [open, userId])

  const roleCatalog = (() => {
    const base = availableRoles.length > 0 ? availableRoles : FALLBACK_ROLES
    const known = new Set(base.map((r) => r.name))
    const extras: RbacItem[] = (detail?.roles ?? [])
      .filter((r) => !known.has(r))
      .map((name) => ({
        name,
        type: "ROLE",
        description: null,
        systemItem: false,
        directChildren: [],
        createdAt: "",
        updatedAt: "",
      }))
    return [...base, ...extras]
  })()

  const handleConfirmStatusToggle = async () => {
    if (!detail) return
    const nextStatus = detail.status === "ACTIVE" ? "DISABLED" : "ACTIVE"
    setConfirmStatusOpen(false)
    setBusyAction("status")
    setErrorMessage(null)
    try {
      const updated = await updateAdminUserStatus(detail.userId, nextStatus)
      setDetail(updated)
      onMutated()
    } catch (error) {
      setErrorMessage(resolveErrorMessage(error))
    } finally {
      setBusyAction(null)
    }
  }

  const handleToggleRole = async (roleName: string, assigned: boolean) => {
    if (!detail) return
    setBusyAction(`role:${roleName}`)
    setErrorMessage(null)
    try {
      const updated = assigned
        ? await revokeAdminUserRole(detail.userId, roleName)
        : await assignAdminUserRole(detail.userId, roleName)
      setDetail(updated)
      onMutated()
      if (currentUser?.userId === detail.userId) {
        void refreshPermissions()
      }
    } catch (error) {
      setErrorMessage(resolveErrorMessage(error))
    } finally {
      setBusyAction(null)
    }
  }

  const isActive = detail?.status === "ACTIVE"
  const isSelf = Boolean(detail && currentUser?.userId === detail.userId)

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent className="sm:max-w-[560px]">
          <SheetHeader>
            <SheetTitle>Hồ sơ & Quyền hạn tài khoản</SheetTitle>
            <SheetDescription>
              Kiểm tra trạng thái hoạt động và quản lý các vai trò RBAC được gán cho người dùng.
            </SheetDescription>
          </SheetHeader>

          {loading ? (
            <div className="flex flex-1 items-center justify-center p-10 text-sm text-muted-foreground">
              <Loader2 className="mr-2 size-4 animate-spin" />
              Đang tải thông tin tài khoản…
            </div>
          ) : detail ? (
            <div className="flex-1 space-y-6 overflow-y-auto px-6 py-5">
              {errorMessage && (
                <div
                  role="alert"
                  className="flex items-start gap-2.5 rounded-lg border border-red-200 bg-red-50 px-3.5 py-2.5 text-xs font-medium text-red-900"
                >
                  <AlertCircle className="mt-0.5 size-4 shrink-0 text-red-600" />
                  <span>{errorMessage}</span>
                </div>
              )}

              <div className="rounded-xl border border-border bg-background p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <div className="text-base font-semibold text-foreground">
                      {detail.fullName || "Chưa cập nhật họ tên"}
                    </div>
                    <div className="font-mono text-xs text-muted-foreground">{detail.email}</div>
                  </div>
                  {isActive ? (
                    <Badge variant="success">Hoạt động</Badge>
                  ) : (
                    <Badge variant="destructive">Đã khóa</Badge>
                  )}
                </div>

                <div className="mt-4 grid grid-cols-2 gap-3 border-t border-border pt-3 text-xs">
                  <div>
                    <span className="text-muted-foreground">Mã định danh:</span>
                    <div className="mt-0.5 font-mono text-foreground">{detail.userId}</div>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Ngày tạo:</span>
                    <div className="mt-0.5 font-mono text-foreground">
                      {formatDateTime(detail.createdAt)}
                    </div>
                  </div>
                </div>
              </div>

              <div className="space-y-2.5">
                <div className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                  Trạng thái truy cập
                </div>
                <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border bg-card p-4">
                  <div className="space-y-0.5">
                    <div className="text-sm font-medium text-foreground">
                      {isActive ? "Tài khoản đang được phép đăng nhập" : "Tài khoản đang bị vô hiệu hóa"}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      Khóa tài khoản sẽ thu hồi phiên làm việc và chặn cấp mới JWT.
                    </p>
                  </div>

                  {canUpdateStatus && (
                    <Button
                      type="button"
                      variant={isActive ? "destructive" : "default"}
                      size="sm"
                      disabled={busyAction === "status" || (isActive && isSelf)}
                      title={
                        isActive && isSelf
                          ? "Không thể tự khóa tài khoản đang đăng nhập"
                          : undefined
                      }
                      onClick={() => setConfirmStatusOpen(true)}
                    >
                      {isActive ? (
                        <>
                          <Lock className="size-3.5" />
                          Khóa tài khoản
                        </>
                      ) : (
                        <>
                          <Unlock className="size-3.5" />
                          Mở khóa tài khoản
                        </>
                      )}
                    </Button>
                  )}
                </div>
              </div>

              <div className="space-y-2.5">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                    Vai trò RBAC ({detail.roles.length})
                  </span>
                  <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                    <Shield className="size-3.5 text-primary" />
                    Áp dụng tức thì
                  </span>
                </div>

                <div className="space-y-2">
                  {roleCatalog.map((role) => {
                    const assigned = detail.roles.includes(role.name)
                    const isBusy = busyAction === `role:${role.name}`
                    return (
                      <div
                        key={role.name}
                        className="flex items-center justify-between gap-3 rounded-xl border border-border bg-card px-4 py-3"
                      >
                        <div className="min-w-0">
                          <div className="flex items-center gap-2">
                            <span className="font-mono text-xs font-semibold text-foreground">
                              {role.name}
                            </span>
                            {assigned && (
                              <Badge variant="success" className="gap-1 px-1.5 py-0 text-[10px]">
                                <Check className="size-2.5" />
                                Đang gán
                              </Badge>
                            )}
                          </div>
                          {role.description && (
                            <div className="mt-0.5 text-xs text-muted-foreground">
                              {role.description}
                            </div>
                          )}
                        </div>

                        {canAssignRole && (
                          <Button
                            type="button"
                            variant={assigned ? "outline" : "secondary"}
                            size="sm"
                            disabled={isBusy}
                            aria-label={
                              assigned
                                ? `Thu hồi vai trò ${role.name}`
                                : `Gán vai trò ${role.name}`
                            }
                            onClick={() => void handleToggleRole(role.name, assigned)}
                          >
                            <UserCheck className="size-3.5" />
                            {assigned ? "Thu hồi" : "Gán vai trò"}
                          </Button>
                        )}
                      </div>
                    )
                  })}
                </div>
              </div>
            </div>
          ) : (
            <div className="p-6 text-sm text-muted-foreground">
              {errorMessage ?? "Không tìm thấy thông tin người dùng."}
            </div>
          )}
        </SheetContent>
      </Sheet>

      <AlertDialog open={confirmStatusOpen} onOpenChange={setConfirmStatusOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {isActive ? "Xác nhận khóa tài khoản?" : "Xác nhận mở khóa tài khoản?"}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {isActive
                ? `Tài khoản ${detail?.email ?? ""} sẽ bị thu hồi phiên đăng nhập hiện tại và không thể truy cập hệ thống cho tới khi được mở khóa lại.`
                : `Tài khoản ${detail?.email ?? ""} sẽ được khôi phục quyền đăng nhập vào hệ thống.`}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Hủy</AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleConfirmStatusToggle()}>
              {isActive ? "Xác nhận khóa" : "Xác nhận mở khóa"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
