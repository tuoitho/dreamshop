import {
  AlertCircle,
  GitBranch,
  KeyRound,
  Link2,
  Lock,
  Pencil,
  Plus,
  RefreshCw,
  ShieldCheck,
  Trash2,
  Unlink,
} from "lucide-react"
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react"
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
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { useAuth } from "@/hooks/use-auth"
import {
  addRbacChild,
  createRbacItem,
  deleteRbacItem,
  listRbacItems,
  removeRbacChild,
  toClientError,
  updateRbacItem,
} from "@/lib/api"
import { formatDateTime } from "@/lib/format"
import type { RbacItem, RbacItemType } from "@/lib/types"
import { cn } from "@/lib/utils"

interface InheritedEntry {
  name: string
  via: string
  item: RbacItem | undefined
}

function computeInheritedEntries(root: RbacItem, byName: ReadonlyMap<string, RbacItem>): InheritedEntry[] {
  const directSet = new Set(root.directChildren)
  const visited = new Set<string>([root.name, ...root.directChildren])
  const queue: Array<{ name: string; topAncestor: string }> = root.directChildren.map((child) => ({
    name: child,
    topAncestor: child,
  }))
  const results: InheritedEntry[] = []

  while (queue.length > 0) {
    const current = queue.shift()!
    const currentNode = byName.get(current.name)
    if (!currentNode) continue

    for (const nextChild of currentNode.directChildren) {
      if (visited.has(nextChild) || directSet.has(nextChild)) continue
      visited.add(nextChild)
      results.push({
        name: nextChild,
        via: current.topAncestor,
        item: byName.get(nextChild),
      })
      queue.push({ name: nextChild, topAncestor: current.topAncestor })
    }
  }

  return results
}

export function RbacPage() {
  const { can, refreshPermissions } = useAuth()
  const canManage = can("rbac.manage")

  const [items, setItems] = useState<RbacItem[]>([])
  const [loading, setLoading] = useState(true)
  const [typeFilter, setTypeFilter] = useState<"ALL" | RbacItemType>("ALL")
  const [selectedName, setSelectedName] = useState<string>("")
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [attachChildName, setAttachChildName] = useState<string>("")
  const [busyMutation, setBusyMutation] = useState(false)

  const [editorOpen, setEditorOpen] = useState(false)
  const [editorMode, setEditorMode] = useState<"create" | "edit">("create")
  const [formName, setFormName] = useState("")
  const [formDesc, setFormDesc] = useState("")
  const [formError, setFormError] = useState<string | null>(null)

  const [confirmDeleteOpen, setConfirmDeleteOpen] = useState(false)

  const loadRbac = useCallback(async (signal?: AbortSignal) => {
    setLoading(true)
    setErrorMessage(null)
    try {
      const res = await listRbacItems(undefined, signal)
      if (signal?.aborted) return
      setItems(res)
      setSelectedName((prev) => {
        if (prev && res.some((item) => item.name === prev)) return prev
        const defaultRole =
          res.find((item) => item.name === "ADMIN") ?? res.find((item) => item.type === "ROLE")
        return defaultRole?.name ?? res[0]?.name ?? ""
      })
    } catch (error) {
      if (!signal?.aborted) {
        setErrorMessage(toClientError(error).message)
      }
    } finally {
      if (!signal?.aborted) {
        setLoading(false)
      }
    }
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    void loadRbac(controller.signal)
    return () => controller.abort()
  }, [loadRbac])

  const byName = useMemo(() => new Map(items.map((item) => [item.name, item])), [items])

  const filteredList = useMemo(
    () => (typeFilter === "ALL" ? items : items.filter((item) => item.type === typeFilter)),
    [items, typeFilter]
  )

  const selectedItem = byName.get(selectedName) ?? filteredList[0] ?? null
  const isSelectedRole = selectedItem?.type === "ROLE"
  const isSelectedSuperAdmin = selectedItem?.name === "SUPER_ADMIN"

  const attachableCandidates = useMemo(() => {
    if (!selectedItem || selectedItem.type !== "ROLE" || selectedItem.name === "SUPER_ADMIN") {
      return []
    }
    const existingDirect = new Set(selectedItem.directChildren)
    return items.filter(
      (candidate) =>
        candidate.name !== selectedItem.name &&
        candidate.name !== "SUPER_ADMIN" &&
        !existingDirect.has(candidate.name)
    )
  }, [items, selectedItem])

  useEffect(() => {
    if (attachableCandidates.length === 0) {
      setAttachChildName("")
      return
    }
    if (!attachableCandidates.some((c) => c.name === attachChildName)) {
      setAttachChildName(attachableCandidates[0]!.name)
    }
  }, [attachableCandidates, attachChildName])

  const inheritedEntries = useMemo(
    () => (selectedItem ? computeInheritedEntries(selectedItem, byName) : []),
    [selectedItem, byName]
  )

  const directParentNames = useMemo(() => {
    if (!selectedItem) return []
    return items
      .filter((candidate) => candidate.directChildren.includes(selectedItem.name))
      .map((candidate) => candidate.name)
  }, [items, selectedItem])

  const openCreateModal = () => {
    setEditorMode("create")
    setFormName("")
    setFormDesc("")
    setFormError(null)
    setEditorOpen(true)
  }

  const openEditModal = () => {
    if (!selectedItem || selectedItem.type !== "ROLE") return
    setEditorMode("edit")
    setFormName(selectedItem.name)
    setFormDesc(selectedItem.description ?? "")
    setFormError(null)
    setEditorOpen(true)
  }

  const handleSaveEditor = async (event: FormEvent) => {
    event.preventDefault()
    const trimmedName = formName.trim()
    if (!trimmedName) {
      setFormError("Vui lòng nhập mã vai trò.")
      return
    }
    setBusyMutation(true)
    setFormError(null)
    try {
      if (editorMode === "create") {
        const created = await createRbacItem({
          name: trimmedName,
          type: "ROLE",
          description: formDesc.trim() || undefined,
        })
        await loadRbac()
        setSelectedName(created.name)
      } else if (selectedItem) {
        const updated = await updateRbacItem(selectedItem.name, {
          name: trimmedName,
          description: formDesc.trim() || undefined,
        })
        await loadRbac()
        setSelectedName(updated.name)
      }
      void refreshPermissions()
      setEditorOpen(false)
    } catch (error) {
      setFormError(toClientError(error).message)
    } finally {
      setBusyMutation(false)
    }
  }

  const handleConfirmDelete = async () => {
    if (!selectedItem) return
    const deletingName = selectedItem.name
    setConfirmDeleteOpen(false)
    setBusyMutation(true)
    setErrorMessage(null)
    try {
      await deleteRbacItem(deletingName)
      setSelectedName("")
      await loadRbac()
      void refreshPermissions()
    } catch (error) {
      setErrorMessage(toClientError(error).message)
    } finally {
      setBusyMutation(false)
    }
  }

  const handleAttachChild = async (event: FormEvent) => {
    event.preventDefault()
    if (!selectedItem || !attachChildName) return
    setBusyMutation(true)
    setErrorMessage(null)
    try {
      await addRbacChild(selectedItem.name, attachChildName)
      await loadRbac()
      void refreshPermissions()
    } catch (error) {
      setErrorMessage(toClientError(error).message)
    } finally {
      setBusyMutation(false)
    }
  }

  const handleDetachChild = async (childName: string) => {
    if (!selectedItem) return
    setBusyMutation(true)
    setErrorMessage(null)
    try {
      await removeRbacChild(selectedItem.name, childName)
      await loadRbac()
      void refreshPermissions()
    } catch (error) {
      setErrorMessage(toClientError(error).message)
    } finally {
      setBusyMutation(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            Cây phân quyền RBAC
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Quản lý từ điển vai trò, quyền thực thi và đồ thị kế thừa phân cấp Parent → Child.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void loadRbac()}
            disabled={loading}
          >
            <RefreshCw className={loading ? "size-3.5 animate-spin" : "size-3.5"} />
            Làm mới
          </Button>
          {canManage && (
            <Button type="button" size="sm" onClick={openCreateModal}>
              <Plus className="size-4" />
              Tạo vai trò
            </Button>
          )}
        </div>
      </div>

      {errorMessage && (
        <div
          role="alert"
          className="flex items-start gap-2.5 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm font-medium text-red-900"
        >
          <AlertCircle className="mt-0.5 size-4 shrink-0 text-red-600" />
          <span>{errorMessage}</span>
        </div>
      )}

      {loading && items.length === 0 ? (
        <div className="grid gap-6 lg:grid-cols-[360px_1fr]">
          <Skeleton className="h-96 w-full rounded-xl" />
          <Skeleton className="h-96 w-full rounded-xl" />
        </div>
      ) : (
        <div className="grid items-start gap-6 lg:grid-cols-[360px_1fr]">
          <Card className="overflow-hidden">
            <div className="border-b border-border p-3">
              <div className="grid grid-cols-3 gap-1 rounded-lg border border-border bg-muted p-1">
                {(
                  [
                    { key: "ALL", label: "Tất cả" },
                    { key: "ROLE", label: "Vai trò" },
                    { key: "PERMISSION", label: "Quyền" },
                  ] as const
                ).map((tab) => (
                  <button
                    key={tab.key}
                    type="button"
                    onClick={() => setTypeFilter(tab.key)}
                    className={cn(
                      "cursor-pointer rounded-md py-1.5 text-xs font-medium transition-colors",
                      typeFilter === tab.key
                        ? "border border-border bg-card font-semibold text-foreground shadow-2xs"
                        : "text-muted-foreground hover:text-foreground"
                    )}
                  >
                    {tab.label}
                  </button>
                ))}
              </div>
            </div>

            <div
              role="listbox"
              aria-label="Danh sách mục RBAC"
              className="max-h-[600px] space-y-2 overflow-y-auto p-3"
            >
              {filteredList.map((item) => {
                const isSelected = selectedItem?.name === item.name
                return (
                  <button
                    key={item.name}
                    type="button"
                    role="option"
                    aria-selected={isSelected}
                    onClick={() => {
                      setSelectedName(item.name)
                      setErrorMessage(null)
                    }}
                    className={cn(
                      "flex w-full cursor-pointer items-start justify-between gap-2 rounded-lg border p-3 text-left transition-colors",
                      isSelected
                        ? "border-primary bg-accent"
                        : "border-border bg-card hover:bg-muted/60"
                    )}
                  >
                    <div className="min-w-0">
                      <div className="flex items-center gap-1.5">
                        {item.type === "ROLE" ? (
                          <ShieldCheck className="size-3.5 shrink-0 text-primary" />
                        ) : (
                          <KeyRound className="size-3.5 shrink-0 text-muted-foreground" />
                        )}
                        <span className="truncate font-mono text-xs font-semibold text-foreground">
                          {item.name}
                        </span>
                      </div>
                      {item.description && (
                        <p className="mt-1 line-clamp-1 text-xs text-muted-foreground">
                          {item.description}
                        </p>
                      )}
                    </div>

                    <div className="flex shrink-0 flex-col items-end gap-1">
                      {item.systemItem || item.type === "PERMISSION" ? (
                        <Badge variant="secondary" className="gap-1 px-1.5 py-0 text-[10px]">
                          <Lock className="size-2.5" />
                          Hệ thống
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
                          Tùy chỉnh
                        </Badge>
                      )}
                    </div>
                  </button>
                )
              })}
            </div>
          </Card>

          {selectedItem ? (
            <div className="space-y-5">
              <Card>
                <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-4">
                  <div className="space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <CardTitle className="font-mono text-lg">{selectedItem.name}</CardTitle>
                      <Badge variant={selectedItem.type === "ROLE" ? "default" : "outline"}>
                        {selectedItem.type === "ROLE" ? "Vai trò" : "Quyền"}
                      </Badge>
                      {(selectedItem.systemItem || selectedItem.type === "PERMISSION") && (
                        <Badge variant="secondary" className="gap-1">
                          <Lock className="size-3" />
                          Khóa hệ thống
                        </Badge>
                      )}
                    </div>
                    <CardDescription>
                      {selectedItem.description || "Chưa có mô tả chi tiết."}
                    </CardDescription>
                  </div>

                  {canManage && isSelectedRole && !selectedItem.systemItem && (
                    <div className="flex items-center gap-2">
                      <Button type="button" variant="outline" size="sm" onClick={openEditModal}>
                        <Pencil className="size-3.5" />
                        Sửa thông tin
                      </Button>
                      <Button
                        type="button"
                        variant="destructive"
                        size="sm"
                        disabled={busyMutation}
                        onClick={() => setConfirmDeleteOpen(true)}
                      >
                        <Trash2 className="size-3.5" />
                        Xóa vai trò tùy chỉnh
                      </Button>
                    </div>
                  )}
                </CardHeader>

                <CardContent className="space-y-4">
                  <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
                    <span>Cập nhật: {formatDateTime(selectedItem.updatedAt)}</span>
                    {directParentNames.length > 0 && (
                      <span>
                        Được gắn vào vai trò:{" "}
                        <span className="font-mono font-medium text-foreground">
                          {directParentNames.join(", ")}
                        </span>
                      </span>
                    )}
                  </div>

                  {!isSelectedRole && (
                    <div className="rounded-xl border border-border bg-muted/40 px-4 py-3 text-xs text-muted-foreground">
                      Quyền thực thi (<span className="font-mono font-medium">PERMISSION</span>) được cố
                      định từ mã nguồn và migration của từng microservice; chỉ có thể gắn hoặc gỡ
                      khỏi các vai trò (<span className="font-mono font-medium">ROLE</span>).
                    </div>
                  )}

                  {isSelectedSuperAdmin && (
                    <div className="rounded-xl border border-border bg-muted/40 px-4 py-3 text-xs text-muted-foreground">
                      Vai trò <span className="font-mono font-semibold text-foreground">SUPER_ADMIN</span>{" "}
                      mặc định sở hữu toàn quyền hệ thống mà không cần gắn từng quyền con trực tiếp.
                    </div>
                  )}

                  {canManage && isSelectedRole && !isSelectedSuperAdmin && attachableCandidates.length > 0 && (
                    <form
                      onSubmit={(e) => void handleAttachChild(e)}
                      className="flex flex-wrap items-end gap-3 rounded-xl border border-border bg-background p-4"
                    >
                      <div className="min-w-[220px] flex-1 space-y-1.5">
                        <Label htmlFor="rbac-attach-child">Chọn mục con để gắn</Label>
                        <select
                          id="rbac-attach-child"
                          value={attachChildName}
                          onChange={(e) => setAttachChildName(e.target.value)}
                          className="flex h-9 w-full rounded-lg border border-input bg-card px-3 text-xs font-mono text-foreground outline-none focus-visible:border-ring"
                        >
                          {attachableCandidates.map((candidate) => (
                            <option key={candidate.name} value={candidate.name}>
                              {candidate.name} ({candidate.type})
                            </option>
                          ))}
                        </select>
                      </div>
                      <Button type="submit" size="sm" disabled={busyMutation || !attachChildName}>
                        <Link2 className="size-3.5" />
                        Gắn mục con
                      </Button>
                    </form>
                  )}
                </CardContent>
              </Card>

              {isSelectedRole && (
                <Card>
                  <CardHeader>
                    <CardTitle className="flex items-center gap-2 text-base">
                      <GitBranch className="size-4 text-primary" />
                      Quan hệ phân cấp trực tiếp & kế thừa
                    </CardTitle>
                    <CardDescription>
                      Phân biệt rõ các mục được gắn trực tiếp với các quyền nhận gián tiếp qua cây kế thừa.
                    </CardDescription>
                  </CardHeader>

                  <CardContent className="space-y-5">
                    <div className="space-y-2.5">
                      <div className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                        Mục con trực tiếp ({selectedItem.directChildren.length})
                      </div>
                      {selectedItem.directChildren.length > 0 ? (
                        <div className="space-y-2">
                          {selectedItem.directChildren.map((childName) => {
                            const childMeta = byName.get(childName)
                            return (
                              <div
                                key={childName}
                                className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-card px-4 py-3"
                              >
                                <div>
                                  <div className="flex items-center gap-2">
                                    <span className="font-mono text-xs font-semibold text-foreground">
                                      {childName}
                                    </span>
                                    <Badge variant="success">Trực tiếp</Badge>
                                    {childMeta && (
                                      <Badge variant="outline" className="text-[10px]">
                                        {childMeta.type}
                                      </Badge>
                                    )}
                                  </div>
                                  {childMeta?.description && (
                                    <div className="mt-0.5 text-xs text-muted-foreground">
                                      {childMeta.description}
                                    </div>
                                  )}
                                </div>

                                {canManage && !isSelectedSuperAdmin && (
                                  <Button
                                    type="button"
                                    variant="outline"
                                    size="sm"
                                    disabled={busyMutation}
                                    aria-label={`Gỡ liên kết ${childName}`}
                                    onClick={() => void handleDetachChild(childName)}
                                  >
                                    <Unlink className="size-3.5" />
                                    Gỡ liên kết
                                  </Button>
                                )}
                              </div>
                            )
                          })}
                        </div>
                      ) : (
                        <p className="rounded-lg border border-dashed border-border px-4 py-4 text-xs text-muted-foreground">
                          {isSelectedSuperAdmin
                            ? "SUPER_ADMIN tự động nhận toàn bộ quyền thực thi trong hệ thống."
                            : "Vai trò này hiện chưa gắn mục con trực tiếp nào."}
                        </p>
                      )}
                    </div>

                    <div className="space-y-2.5">
                      <div className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                        Mục nhận qua kế thừa ({inheritedEntries.length})
                      </div>
                      {inheritedEntries.length > 0 ? (
                        <div className="space-y-2">
                          {inheritedEntries.map((entry) => (
                            <div
                              key={entry.name}
                              className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-background px-4 py-2.5"
                            >
                              <div>
                                <span className="font-mono text-xs font-medium text-foreground">
                                  {entry.name}
                                </span>
                                {entry.item?.description && (
                                  <div className="mt-0.5 text-xs text-muted-foreground">
                                    {entry.item.description}
                                  </div>
                                )}
                              </div>
                              <Badge variant="outline" className="font-mono text-[11px]">
                                Kế thừa qua {entry.via}
                              </Badge>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <p className="text-xs text-muted-foreground">
                          Không phát sinh quyền kế thừa gián tiếp ở cấp sâu hơn.
                        </p>
                      )}
                    </div>
                  </CardContent>
                </Card>
              )}
            </div>
          ) : (
            <Card>
              <CardContent className="p-8 text-center text-sm text-muted-foreground">
                Chọn một vai trò hoặc quyền ở cột bên trái để xem chi tiết.
              </CardContent>
            </Card>
          )}
        </div>
      )}

      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editorMode === "create"
                ? "Tạo vai trò tùy chỉnh"
                : "Cập nhật vai trò tùy chỉnh"}
            </DialogTitle>
            <DialogDescription>
              Định danh vai trò viết IN_HOA_GACH_DUOI (VD: WAREHOUSE_LEAD). Các quyền thực thi
              (PERMISSION) được cố định từ mã nguồn hệ thống.
            </DialogDescription>
          </DialogHeader>

          {formError && (
            <div
              role="alert"
              className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs font-medium text-red-900"
            >
              {formError}
            </div>
          )}

          <form onSubmit={(e) => void handleSaveEditor(e)} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="rbac-item-name">Mã vai trò *</Label>
              <Input
                id="rbac-item-name"
                value={formName}
                onChange={(e) => setFormName(e.target.value)}
                placeholder="VD: WAREHOUSE_LEAD"
                className="font-mono"
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="rbac-item-desc">Mô tả chức năng</Label>
              <Textarea
                id="rbac-item-desc"
                rows={3}
                value={formDesc}
                onChange={(e) => setFormDesc(e.target.value)}
                placeholder="Ghi chú phạm vi quyền hạn cho vận hành viên…"
              />
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setEditorOpen(false)}>
                Hủy
              </Button>
              <Button type="submit" disabled={busyMutation}>
                {busyMutation ? "Đang lưu…" : "Lưu vai trò"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={confirmDeleteOpen} onOpenChange={setConfirmDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Xóa vai trò tùy chỉnh?</AlertDialogTitle>
            <AlertDialogDescription>
              Vai trò <span className="font-mono font-semibold">{selectedItem?.name}</span> và toàn
              bộ liên kết phân cấp trực tiếp của nó sẽ bị gỡ vĩnh viễn.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Hủy</AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleConfirmDelete()}>
              Xác nhận xóa
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
