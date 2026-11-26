import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table"
import { ChevronLeft, ChevronRight, RefreshCw, Search, Settings2 } from "lucide-react"
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useAuth } from "@/hooks/use-auth"
import { listAdminUsers, listRbacItems } from "@/lib/api"
import { resolveErrorMessage } from "@/lib/error-messages"
import { formatDateTime, formatNumber } from "@/lib/format"
import type { AccountStatus, AdminUser, AdminUserPage, RbacItem } from "@/lib/types"
import { cn } from "@/lib/utils"
import { UserDetailSheet } from "./user-detail-sheet"

type UserStatusTab = "ALL" | AccountStatus

export function UsersPage() {
  const { can } = useAuth()
  const canUpdateStatus = can("user.status.update")
  const canAssignRole = can("user.role.assign")
  const canViewRbac = can("rbac.view")

  const [statusTab, setStatusTab] = useState<UserStatusTab>("ALL")
  const [searchDraft, setSearchDraft] = useState("")
  const [appliedQuery, setAppliedQuery] = useState("")
  const [page, setPage] = useState(1)
  const [data, setData] = useState<AdminUserPage | null>(null)
  const [availableRoles, setAvailableRoles] = useState<RbacItem[]>([])
  const [loading, setLoading] = useState(true)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null)

  const loadUsers = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true)
      setErrorMessage(null)
      try {
        const res = await listAdminUsers(
          {
            status: statusTab === "ALL" ? undefined : statusTab,
            q: appliedQuery || undefined,
            page,
            pageSize: 15,
          },
          signal
        )
        if (!signal?.aborted) {
          setData(res)
        }
      } catch (error) {
        if (!signal?.aborted) {
          setErrorMessage(resolveErrorMessage(error))
        }
      } finally {
        if (!signal?.aborted) {
          setLoading(false)
        }
      }
    },
    [statusTab, appliedQuery, page]
  )

  useEffect(() => {
    const controller = new AbortController()
    void loadUsers(controller.signal)
    return () => controller.abort()
  }, [loadUsers])

  useEffect(() => {
    if (!canViewRbac) return
    const controller = new AbortController()
    void (async () => {
      try {
        const roles = await listRbacItems("ROLE", controller.signal)
        if (!controller.signal.aborted) {
          setAvailableRoles(roles)
        }
      } catch {
        // Fallback role list in UserDetailSheet handles missing RBAC read access.
      }
    })()
    return () => controller.abort()
  }, [canViewRbac])

  const handleSearchSubmit = (event: FormEvent) => {
    event.preventDefault()
    setPage(1)
    setAppliedQuery(searchDraft.trim())
  }

  const columns = useMemo<ColumnDef<AdminUser>[]>(
    () => [
      {
        accessorKey: "fullName",
        header: "Người dùng",
        cell: ({ row }) => {
          const item = row.original
          const initials = (item.fullName || item.email || "U")
            .trim()
            .slice(0, 2)
            .toUpperCase()
          return (
            <div className="flex min-w-[220px] items-center gap-3">
              <div className="flex size-9 shrink-0 items-center justify-center rounded-full border border-emerald-200 bg-emerald-50 font-mono text-xs font-semibold text-emerald-800">
                {initials}
              </div>
              <div className="min-w-0">
                <div className="truncate font-medium text-foreground">
                  {item.fullName || "Chưa đặt tên"}
                </div>
                <div className="truncate font-mono text-xs text-muted-foreground">{item.email}</div>
              </div>
            </div>
          )
        },
      },
      {
        accessorKey: "roles",
        header: "Vai trò RBAC",
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-1">
            {row.original.roles.length > 0 ? (
              row.original.roles.map((role) => (
                <Badge key={role} variant="outline" className="font-mono text-[11px]">
                  {role}
                </Badge>
              ))
            ) : (
              <span className="text-xs text-muted-foreground">—</span>
            )}
          </div>
        ),
      },
      {
        accessorKey: "status",
        header: "Trạng thái",
        cell: ({ row }) =>
          row.original.status === "ACTIVE" ? (
            <Badge variant="success">Hoạt động</Badge>
          ) : (
            <Badge variant="destructive">Đã khóa</Badge>
          ),
      },
      {
        accessorKey: "createdAt",
        header: "Ngày tham gia",
        cell: ({ row }) => (
          <span className="font-mono text-xs text-muted-foreground">
            {formatDateTime(row.original.createdAt)}
          </span>
        ),
      },
      {
        id: "actions",
        header: () => <div className="text-right">Thao tác</div>,
        cell: ({ row }) => {
          const item = row.original
          return (
            <div className="text-right">
              <Button
                type="button"
                variant="outline"
                size="sm"
                aria-label={`Chi tiết ${item.fullName || item.email}`}
                onClick={() => setSelectedUserId(item.userId)}
              >
                <Settings2 className="size-3.5" />
                Chi tiết
              </Button>
            </div>
          )
        },
      },
    ],
    []
  )

  const table = useReactTable({
    data: data?.items ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  const totalGlobal = (data?.activeCount ?? 0) + (data?.disabledCount ?? 0)

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            Quản trị tài khoản người dùng
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Tra cứu hồ sơ người dùng, kiểm soát trạng thái truy cập và phân bổ vai trò vận hành.
          </p>
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => void loadUsers()}
          disabled={loading}
        >
          <RefreshCw className={loading ? "size-3.5 animate-spin" : "size-3.5"} />
          Làm mới
        </Button>
      </div>

      {errorMessage && (
        <div
          role="alert"
          className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900"
        >
          {errorMessage}
        </div>
      )}

      <Card className="overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4">
          <div className="inline-flex flex-wrap items-center gap-1 rounded-lg border border-border bg-muted p-1">
            {(
              [
                { key: "ALL", label: "Tất cả", count: totalGlobal },
                { key: "ACTIVE", label: "Hoạt động", count: data?.activeCount ?? 0 },
                { key: "DISABLED", label: "Đã khóa", count: data?.disabledCount ?? 0 },
              ] as const
            ).map((tab) => (
              <button
                key={tab.key}
                type="button"
                onClick={() => {
                  setStatusTab(tab.key)
                  setPage(1)
                }}
                className={cn(
                  "inline-flex cursor-pointer items-center gap-2 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                  statusTab === tab.key
                    ? "border border-border bg-card font-semibold text-foreground shadow-2xs"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                <span>{tab.label}</span>
                <span className="rounded-full bg-background px-1.5 py-0.5 font-mono text-[11px] tabular-nums">
                  {formatNumber(tab.count)}
                </span>
              </button>
            ))}
          </div>

          <form onSubmit={handleSearchSubmit} className="flex flex-wrap items-center gap-2">
            <div className="relative">
              <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={searchDraft}
                onChange={(e) => setSearchDraft(e.target.value)}
                placeholder="Tìm theo email hoặc họ tên…"
                className="w-60 pl-8 text-xs sm:w-72"
              />
            </div>
            <Button type="submit" variant="outline" size="sm">
              Lọc
            </Button>
          </form>
        </div>

        <CardContent className="p-0">
          {loading && !data ? (
            <div className="space-y-3 p-6">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : table.getRowModel().rows.length > 0 ? (
            <Table>
              <TableHeader>
                {table.getHeaderGroups().map((headerGroup) => (
                  <TableRow key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
                      <TableHead key={header.id}>
                        {header.isPlaceholder
                          ? null
                          : flexRender(header.column.columnDef.header, header.getContext())}
                      </TableHead>
                    ))}
                  </TableRow>
                ))}
              </TableHeader>
              <TableBody>
                {table.getRowModel().rows.map((row) => (
                  <TableRow key={row.id}>
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id}>
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </TableCell>
                    ))}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <div className="px-6 py-12 text-center text-sm text-muted-foreground">
              Không tìm thấy tài khoản phù hợp với điều kiện lọc.
            </div>
          )}

          {data && data.totalPages > 1 && (
            <div className="flex items-center justify-between border-t border-border px-5 py-3 text-xs text-muted-foreground">
              <span>
                Trang <span className="font-mono font-semibold text-foreground">{data.page}</span> /{" "}
                <span className="font-mono">{data.totalPages}</span> ({formatNumber(data.totalItems)}{" "}
                tài khoản)
              </span>
              <div className="flex items-center gap-1.5">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  <ChevronLeft className="size-3.5" />
                  Trước
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={page >= data.totalPages}
                  onClick={() => setPage((p) => p + 1)}
                >
                  Sau
                  <ChevronRight className="size-3.5" />
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <UserDetailSheet
        open={Boolean(selectedUserId)}
        onOpenChange={(next) => {
          if (!next) setSelectedUserId(null)
        }}
        userId={selectedUserId}
        availableRoles={availableRoles}
        canUpdateStatus={canUpdateStatus}
        canAssignRole={canAssignRole}
        onMutated={() => void loadUsers()}
      />
    </div>
  )
}
