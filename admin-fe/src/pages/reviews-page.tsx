import {
  ChevronLeft,
  ChevronRight,
  Eye,
  EyeOff,
  MessageSquareReply,
  RefreshCw,
  Search,
  Star,
} from "lucide-react"
import { useCallback, useEffect, useState, type FormEvent } from "react"
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
import {
  listAdminReviews,
  replyAdminReview,
  setAdminReviewVisibility,
} from "@/lib/api"
import { resolveErrorMessage } from "@/lib/error-messages"
import { formatDateTime, formatNumber } from "@/lib/format"
import type { AdminReview, AdminReviewPage, ListAdminReviewsParams } from "@/lib/types"
import { cn } from "@/lib/utils"

const PAGE_SIZE = 20

function StarDisplay({ rating }: { rating: number }) {
  return (
    <div className="flex items-center gap-0.5" aria-label={`${rating} sao`}>
      {Array.from({ length: 5 }).map((_, i) => (
        <Star
          key={i}
          className={cn(
            "size-3.5",
            i < rating ? "fill-amber-400 text-amber-400" : "text-muted-foreground/30"
          )}
        />
      ))}
    </div>
  )
}

export function ReviewsPage() {
  const { can } = useAuth()
  const canModerate = can("review.admin.moderate")

  const [statusFilter, setStatusFilter] = useState<"all" | "visible" | "hidden">("all")
  const [replyFilter, setReplyFilter] = useState<"all" | "replied" | "unreplied">("all")
  const [ratingFilter, setRatingFilter] = useState<number>(0)
  const [searchDraft, setSearchDraft] = useState("")
  const [appliedQuery, setAppliedQuery] = useState("")
  const [page, setPage] = useState(1)

  const [data, setData] = useState<AdminReviewPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  const [replyingReview, setReplyingReview] = useState<AdminReview | null>(null)
  const [replyDraft, setReplyDraft] = useState("")
  const [replySaving, setReplySaving] = useState(false)

  const [hidingReview, setHidingReview] = useState<AdminReview | null>(null)
  const [hideReasonDraft, setHideReasonDraft] = useState("")
  const [visibilitySavingId, setVisibilitySavingId] = useState<string | null>(null)

  const loadReviews = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true)
      setErrorMessage(null)
      const params: ListAdminReviewsParams = {
        status: statusFilter,
        replyStatus: replyFilter,
        page,
        pageSize: PAGE_SIZE,
      }
      if (appliedQuery) params.q = appliedQuery
      if (ratingFilter > 0) params.rating = ratingFilter
      try {
        const res = await listAdminReviews(params, signal)
        if (signal?.aborted) return
        setData(res)
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
    [statusFilter, replyFilter, ratingFilter, appliedQuery, page]
  )

  useEffect(() => {
    const controller = new AbortController()
    void loadReviews(controller.signal)
    return () => controller.abort()
  }, [loadReviews])

  function handleSearchSubmit(e: FormEvent) {
    e.preventDefault()
    setPage(1)
    setAppliedQuery(searchDraft.trim())
  }

  async function handleSaveReply(e: FormEvent) {
    e.preventDefault()
    if (!replyingReview || replySaving) return
    setReplySaving(true)
    try {
      await replyAdminReview(replyingReview.id, replyDraft.trim())
      setReplyingReview(null)
      setReplyDraft("")
      await loadReviews()
    } catch {
      // Error toast handled by interceptor
    } finally {
      setReplySaving(false)
    }
  }

  async function handleConfirmHide(e: FormEvent) {
    e.preventDefault()
    if (!hidingReview || visibilitySavingId) return
    setVisibilitySavingId(hidingReview.id)
    try {
      await setAdminReviewVisibility(hidingReview.id, {
        hidden: true,
        reason: hideReasonDraft.trim(),
      })
      setHidingReview(null)
      setHideReasonDraft("")
      await loadReviews()
    } catch {
      // Error toast handled by interceptor
    } finally {
      setVisibilitySavingId(null)
    }
  }

  async function handleRestore(review: AdminReview) {
    if (visibilitySavingId) return
    setVisibilitySavingId(review.id)
    try {
      await setAdminReviewVisibility(review.id, { hidden: false })
      await loadReviews()
    } catch {
      // Error toast handled by interceptor
    } finally {
      setVisibilitySavingId(null)
    }
  }

  return (
    <div className="space-y-6">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardContent className="p-4">
            <p className="text-xs font-medium text-muted-foreground">Đánh giá hiển thị</p>
            <p className="mt-1 text-2xl font-bold tabular-nums" data-testid="kpi-visible-reviews">
              {data ? formatNumber(data.visibleCount) : "—"}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <p className="text-xs font-medium text-muted-foreground">Đánh giá đang ẩn</p>
            <p className="mt-1 text-2xl font-bold tabular-nums" data-testid="kpi-hidden-reviews">
              {data ? formatNumber(data.hiddenCount) : "—"}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <p className="text-xs font-medium text-muted-foreground">Chưa phản hồi</p>
            <p className="mt-1 text-2xl font-bold tabular-nums text-amber-600" data-testid="kpi-unreplied-reviews">
              {data ? formatNumber(data.unrepliedCount) : "—"}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <p className="text-xs font-medium text-muted-foreground">Điểm trung bình</p>
            <p className="mt-1 flex items-center gap-1.5 text-2xl font-bold tabular-nums">
              <span>{data ? data.averageRating.toFixed(1) : "—"}</span>
              <Star className="size-5 fill-amber-400 text-amber-400" />
            </p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardContent className="space-y-4 p-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex flex-wrap items-center gap-2">
              {(
                [
                  { key: "all", label: "Tất cả" },
                  { key: "visible", label: "Đang hiển thị" },
                  { key: "hidden", label: "Đã ẩn" },
                ] as const
              ).map((tab) => (
                <Button
                  key={tab.key}
                  type="button"
                  size="sm"
                  variant={statusFilter === tab.key ? "default" : "outline"}
                  onClick={() => {
                    setStatusFilter(tab.key)
                    setPage(1)
                  }}
                >
                  {tab.label}
                </Button>
              ))}
              <select
                aria-label="Lọc theo phản hồi"
                value={replyFilter}
                onChange={(e) => {
                  setReplyFilter(e.target.value as "all" | "replied" | "unreplied")
                  setPage(1)
                }}
                className="h-8 rounded-md border bg-background px-2.5 text-xs"
              >
                <option value="all">Mọi trạng thái phản hồi</option>
                <option value="unreplied">Chưa phản hồi</option>
                <option value="replied">Đã phản hồi</option>
              </select>
              <select
                aria-label="Lọc theo số sao"
                value={ratingFilter}
                onChange={(e) => {
                  setRatingFilter(Number(e.target.value))
                  setPage(1)
                }}
                className="h-8 rounded-md border bg-background px-2.5 text-xs"
              >
                <option value={0}>Mọi số sao</option>
                {[5, 4, 3, 2, 1].map((s) => (
                  <option key={s} value={s}>
                    {s} sao
                  </option>
                ))}
              </select>
            </div>

            <form onSubmit={handleSearchSubmit} className="flex items-center gap-2">
              <div className="relative">
                <Search className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
                <Input
                  value={searchDraft}
                  onChange={(e) => setSearchDraft(e.target.value)}
                  placeholder="Tìm sản phẩm, khách hàng, nội dung..."
                  className="h-9 w-64 pl-8 text-sm"
                />
              </div>
              <Button type="submit" size="sm" variant="secondary">
                Tìm
              </Button>
              <Button
                type="button"
                size="icon"
                variant="outline"
                aria-label="Tải lại danh sách đánh giá"
                onClick={() => void loadReviews()}
              >
                <RefreshCw className="size-4" />
              </Button>
            </form>
          </div>

          {errorMessage && (
            <div className="rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
              {errorMessage}
            </div>
          )}

          {loading ? (
            <div className="space-y-3">
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
            </div>
          ) : !data || data.items.length === 0 ? (
            <div className="py-12 text-center text-sm text-muted-foreground">
              Không có đánh giá nào khớp bộ lọc hiện tại.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Sản phẩm</TableHead>
                    <TableHead>Khách hàng & Sao</TableHead>
                    <TableHead className="min-w-[280px]">Nội dung & Phản hồi</TableHead>
                    <TableHead>Trạng thái</TableHead>
                    <TableHead className="text-right">Thao tác</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.items.map((review) => (
                    <TableRow key={review.id} data-testid={`admin-review-row-${review.id}`}>
                      <TableCell className="align-top">
                        <div className="font-medium text-foreground">
                          {review.productName || review.productId}
                        </div>
                        {review.variantLabel && (
                          <div className="mt-0.5 text-xs text-muted-foreground">
                            {review.variantLabel}
                          </div>
                        )}
                        <div className="mt-0.5 font-mono text-[11px] text-muted-foreground">
                          {review.productId}
                        </div>
                      </TableCell>
                      <TableCell className="align-top">
                        <div className="font-medium text-foreground">{review.authorName}</div>
                        <div className="mt-1">
                          <StarDisplay rating={review.rating} />
                        </div>
                        <div className="mt-1 text-[11px] text-muted-foreground">
                          {formatDateTime(review.createdAt)}
                        </div>
                      </TableCell>
                      <TableCell className="align-top">
                        <p className="whitespace-pre-line text-sm text-foreground">
                          {review.comment || "(Không có nội dung văn bản)"}
                        </p>
                        {review.images && review.images.length > 0 && (
                          <div className="mt-2 flex flex-wrap gap-1.5">
                            {review.images.map((url, idx) => (
                              <a
                                key={`${url}-${idx}`}
                                href={url}
                                target="_blank"
                                rel="noreferrer"
                                className="size-11 overflow-hidden rounded border bg-muted"
                              >
                                <img
                                  src={url}
                                  alt={`Ảnh đánh giá ${idx + 1}`}
                                  className="size-full object-cover"
                                />
                              </a>
                            ))}
                          </div>
                        )}
                        {review.adminReply && (
                          <div className="mt-2 rounded-md border bg-muted/40 p-2 text-xs">
                            <span className="font-semibold text-primary">Phản hồi shop: </span>
                            <span>{review.adminReply}</span>
                          </div>
                        )}
                      </TableCell>
                      <TableCell className="align-top">
                        {review.hidden ? (
                          <div className="space-y-1">
                            <Badge variant="secondary">Đã ẩn</Badge>
                            {review.hiddenReason && (
                              <p className="max-w-[160px] text-[11px] text-muted-foreground">
                                {review.hiddenReason}
                              </p>
                            )}
                          </div>
                        ) : (
                          <Badge variant="default">Hiển thị</Badge>
                        )}
                      </TableCell>
                      <TableCell className="align-top text-right">
                        {canModerate ? (
                          <div className="flex justify-end gap-1.5">
                            <Button
                              type="button"
                              size="sm"
                              variant="outline"
                              onClick={() => {
                                setReplyingReview(review)
                                setReplyDraft(review.adminReply ?? "")
                              }}
                            >
                              <MessageSquareReply className="size-3.5" />
                              {review.adminReply ? "Sửa phản hồi" : "Phản hồi"}
                            </Button>
                            {review.hidden ? (
                              <Button
                                type="button"
                                size="sm"
                                variant="secondary"
                                disabled={visibilitySavingId === review.id}
                                onClick={() => void handleRestore(review)}
                              >
                                <Eye className="size-3.5" />
                                Hiện lại
                              </Button>
                            ) : (
                              <Button
                                type="button"
                                size="sm"
                                variant="ghost"
                                className="text-destructive hover:text-destructive"
                                disabled={visibilitySavingId === review.id}
                                onClick={() => {
                                  setHidingReview(review)
                                  setHideReasonDraft(review.hiddenReason ?? "")
                                }}
                              >
                                <EyeOff className="size-3.5" />
                                Ẩn
                              </Button>
                            )}
                          </div>
                        ) : (
                          <span className="text-xs text-muted-foreground">Chỉ xem</span>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}

          {data && data.totalPages > 1 && (
            <div className="flex items-center justify-between pt-2 text-xs text-muted-foreground">
              <span>
                Trang {data.page} / {data.totalPages} ({formatNumber(data.totalItems)} đánh giá)
              </span>
              <div className="flex items-center gap-1.5">
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  <ChevronLeft className="size-4" />
                  Trước
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={page >= data.totalPages}
                  onClick={() => setPage((p) => Math.min(data.totalPages, p + 1))}
                >
                  Sau
                  <ChevronRight className="size-4" />
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {replyingReview && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Phản hồi đánh giá"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        >
          <form
            onSubmit={(e) => void handleSaveReply(e)}
            className="w-full max-w-lg rounded-xl border bg-card p-5 shadow-lg"
          >
            <h3 className="text-base font-semibold">Phản hồi đánh giá của {replyingReview.authorName}</h3>
            <p className="mt-1 text-xs text-muted-foreground">
              Sản phẩm: {replyingReview.productName || replyingReview.productId}
            </p>
            <label
              htmlFor="admin-review-reply-input"
              className="mt-4 block text-xs font-medium text-muted-foreground"
            >
              Nội dung phản hồi chính thức (để trống để xóa phản hồi)
            </label>
            <textarea
              id="admin-review-reply-input"
              rows={4}
              maxLength={1000}
              value={replyDraft}
              onChange={(e) => setReplyDraft(e.target.value)}
              placeholder="Nhập lời cảm ơn hoặc giải đáp cho khách hàng..."
              className="mt-1.5 w-full rounded-lg border bg-background p-2.5 text-sm"
            />
            <div className="mt-4 flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setReplyingReview(null)
                  setReplyDraft("")
                }}
              >
                Hủy
              </Button>
              <Button type="submit" disabled={replySaving}>
                {replySaving ? "Đang lưu..." : "Lưu phản hồi"}
              </Button>
            </div>
          </form>
        </div>
      )}

      {hidingReview && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Ẩn đánh giá"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        >
          <form
            onSubmit={(e) => void handleConfirmHide(e)}
            className="w-full max-w-md rounded-xl border bg-card p-5 shadow-lg"
          >
            <h3 className="text-base font-semibold">Ẩn đánh giá này?</h3>
            <p className="mt-1 text-xs text-muted-foreground">
              Đánh giá bị ẩn sẽ không hiển thị công khai và không còn tính vào điểm sao sản phẩm.
            </p>
            <label
              htmlFor="admin-review-hide-reason"
              className="mt-4 block text-xs font-medium text-muted-foreground"
            >
              Lý do ẩn (tùy chọn)
            </label>
            <Input
              id="admin-review-hide-reason"
              value={hideReasonDraft}
              onChange={(e) => setHideReasonDraft(e.target.value)}
              placeholder="Ví dụ: Nội dung quảng cáo / spam..."
              className="mt-1.5"
            />
            <div className="mt-4 flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setHidingReview(null)
                  setHideReasonDraft("")
                }}
              >
                Hủy
              </Button>
              <Button type="submit" variant="destructive" disabled={Boolean(visibilitySavingId)}>
                Xác nhận ẩn
              </Button>
            </div>
          </form>
        </div>
      )}
    </div>
  )
}
