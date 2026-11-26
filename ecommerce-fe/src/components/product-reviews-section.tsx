import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useSearchParams } from "react-router-dom"
import {
  Camera,
  CheckCircle2,
  Edit3,
  Loader2,
  MessageSquareReply,
  Star,
  Trash2,
  X,
} from "lucide-react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { useAuth } from "@/hooks/use-auth"
import {
  createProductReview,
  deleteProductReview,
  getReviewEligibility,
  listProductReviews,
  updateProductReview,
  uploadReviewMediaFile,
} from "@/lib/api"
import { resolveErrorMessage } from "@/lib/error-messages"
import { formatRating } from "@/lib/product-availability"
import type {
  ProductReview,
  ReviewEligibility,
  ReviewRatingSummary,
} from "@/lib/types"
import { cn } from "@/lib/utils"

const MAX_REVIEW_IMAGES = 5
const ALLOWED_IMAGE_TYPES = new Set([
  "image/jpeg",
  "image/png",
  "image/webp",
  "image/avif",
])
const MAX_IMAGE_BYTES = 5 * 1024 * 1024

interface ProductReviewsSectionProps {
  productId: string
  initialRating: number
  initialReviewCount: number
  onSummaryChange?: (summary: ReviewRatingSummary) => void
}

function formatDate(raw: string): string {
  if (!raw) return ""
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleDateString("vi-VN", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  })
}

function StarRow({
  rating,
  size = "sm",
}: {
  rating: number
  size?: "sm" | "md"
}) {
  const cls = size === "md" ? "size-5" : "size-3.5"
  return (
    <div className="flex items-center gap-0.5" aria-label={`${rating} trên 5 sao`}>
      {Array.from({ length: 5 }).map((_, i) => (
        <Star
          key={i}
          className={cn(
            cls,
            i < Math.round(rating)
              ? "fill-amber-400 text-amber-400"
              : "text-muted-foreground/30"
          )}
        />
      ))}
    </div>
  )
}

export function ProductReviewsSection({
  productId,
  initialRating,
  initialReviewCount,
  onSummaryChange,
}: ProductReviewsSectionProps) {
  const { user, isAuthenticated } = useAuth()
  const [searchParams] = useSearchParams()
  const preferredOrderId = searchParams.get("reviewOrder") ?? ""

  const [reviews, setReviews] = useState<ProductReview[]>([])
  const [summary, setSummary] = useState<ReviewRatingSummary>({
    averageRating: initialRating,
    totalReviews: initialReviewCount,
    breakdown: [5, 4, 3, 2, 1].map((rating) => ({ rating, count: 0 })),
  })
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(0)
  const [ratingFilter, setRatingFilter] = useState<number>(0)
  const [hasImagesOnly, setHasImagesOnly] = useState(false)
  const [sortBy, setSortBy] = useState<"newest" | "highest" | "lowest">("newest")
  const [loading, setLoading] = useState(true)

  const [eligibility, setEligibility] = useState<ReviewEligibility | null>(null)
  const [composerOpen, setComposerOpen] = useState(false)
  const [editingReview, setEditingReview] = useState<ProductReview | null>(null)
  const [selectedOrderId, setSelectedOrderId] = useState("")
  const [formRating, setFormRating] = useState(5)
  const [formComment, setFormComment] = useState("")
  const [formImages, setFormImages] = useState<string[]>([])
  const [uploading, setUploading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [previewImage, setPreviewImage] = useState<string | null>(null)

  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const canLoadEligibility = isAuthenticated && Boolean(user)

  const loadReviews = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true)
      try {
        const res = await listProductReviews(
          productId,
          {
            rating: ratingFilter || undefined,
            hasImages: hasImagesOnly || undefined,
            sort: sortBy,
            page,
            pageSize: 10,
          },
          signal
        )
        if (!res) return
        setReviews(res.items ?? [])
        setTotalPages(res.totalPages ?? 0)
        if (res.summary) {
          setSummary(res.summary)
        }
      } catch {
        // Keep previous state on abort/network glitch
      } finally {
        if (!signal?.aborted) {
          setLoading(false)
        }
      }
    },
    [productId, ratingFilter, hasImagesOnly, sortBy, page]
  )

  const loadEligibility = useCallback(
    async (signal?: AbortSignal) => {
      if (!canLoadEligibility) {
        setEligibility(null)
        return
      }
      try {
        const res = await getReviewEligibility(productId, signal)
        if (!res) return
        setEligibility(res)
        const unreviewed = (res.eligibleOrders ?? []).filter((o) => !o.reviewed)
        if (unreviewed.length > 0) {
          const matched = unreviewed.find((o) => o.orderId === preferredOrderId)
          setSelectedOrderId(matched ? matched.orderId : unreviewed[0].orderId)
          if (matched) {
            setComposerOpen(true)
          }
        }
      } catch {
        setEligibility(null)
      }
    },
    [canLoadEligibility, productId, preferredOrderId]
  )

  useEffect(() => {
    const controller = new AbortController()
    void loadReviews(controller.signal)
    return () => controller.abort()
  }, [loadReviews])

  useEffect(() => {
    const controller = new AbortController()
    void loadEligibility(controller.signal)
    return () => controller.abort()
  }, [loadEligibility])

  const unreviewedOrders = useMemo(
    () => (eligibility?.eligibleOrders ?? []).filter((o) => !o.reviewed),
    [eligibility]
  )

  const breakdownMap = useMemo(() => {
    const map = new Map<number, number>()
    for (const row of summary.breakdown ?? []) {
      map.set(row.rating, row.count)
    }
    return map
  }, [summary.breakdown])

  function openNewComposer() {
    setEditingReview(null)
    setFormRating(5)
    setFormComment("")
    setFormImages([])
    if (unreviewedOrders.length > 0 && !selectedOrderId) {
      setSelectedOrderId(unreviewedOrders[0].orderId)
    }
    setComposerOpen(true)
  }

  function openEditComposer(review: ProductReview) {
    setEditingReview(review)
    setFormRating(review.rating)
    setFormComment(review.comment)
    setFormImages(review.images ?? [])
    setComposerOpen(true)
  }

  async function handleFileSelect(files: FileList | null) {
    if (!files || files.length === 0) return
    const remaining = MAX_REVIEW_IMAGES - formImages.length
    if (remaining <= 0) {
      toast.error(`Tối đa ${MAX_REVIEW_IMAGES} ảnh cho mỗi đánh giá.`)
      return
    }
    const picked = Array.from(files).slice(0, remaining)
    for (const file of picked) {
      if (!ALLOWED_IMAGE_TYPES.has(file.type)) {
        toast.error("Chỉ hỗ trợ ảnh định dạng JPEG, PNG, WebP hoặc AVIF.")
        return
      }
      if (file.size > MAX_IMAGE_BYTES) {
        toast.error("Mỗi ảnh đánh giá không được vượt quá 5 MB.")
        return
      }
    }

    setUploading(true)
    try {
      const uploadedUrls: string[] = []
      for (const file of picked) {
        const publicUrl = await uploadReviewMediaFile(file)
        uploadedUrls.push(publicUrl)
      }
      setFormImages((prev) => [...prev, ...uploadedUrls].slice(0, MAX_REVIEW_IMAGES))
    } catch (err) {
      toast.error(resolveErrorMessage(err, { fallback: "Không thể tải ảnh lên." }))
    } finally {
      setUploading(false)
      if (fileInputRef.current) {
        fileInputRef.current.value = ""
      }
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (submitting || uploading) return
    setSubmitting(true)
    try {
      if (editingReview) {
        const res = await updateProductReview(editingReview.id, {
          rating: formRating,
          comment: formComment.trim(),
          images: formImages,
        })
        if (res?.summary) {
          setSummary(res.summary)
          onSummaryChange?.(res.summary)
        }
      } else {
        const orderId = selectedOrderId || unreviewedOrders[0]?.orderId || ""
        const res = await createProductReview({
          productId,
          orderId,
          authorName: user?.fullName || undefined,
          rating: formRating,
          comment: formComment.trim(),
          images: formImages,
        })
        if (res?.summary) {
          setSummary(res.summary)
          onSummaryChange?.(res.summary)
        }
      }
      setComposerOpen(false)
      setEditingReview(null)
      setFormComment("")
      setFormImages([])
      if (page !== 1) {
        setPage(1)
        await loadEligibility()
      } else {
        await Promise.all([loadReviews(), loadEligibility()])
      }
    } catch {
      // Error toast is already handled by axios interceptor
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(reviewId: string) {
    if (deletingId) return
    setDeletingId(reviewId)
    try {
      const res = await deleteProductReview(reviewId)
      if (res?.summary) {
        setSummary(res.summary)
        onSummaryChange?.(res.summary)
      }
      if (editingReview?.id === reviewId) {
        setComposerOpen(false)
        setEditingReview(null)
      }
      await Promise.all([loadReviews(), loadEligibility()])
    } catch {
      // Error toast handled by interceptor
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <section
      id="reviews"
      data-testid="product-reviews-section"
      className="mt-10 rounded-3xl bg-card p-6 ring-1 ring-foreground/10"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold">Đánh giá từ khách hàng</h2>
          <p className="mt-0.5 text-xs text-muted-foreground">
            Chỉ khách hàng đã nhận hàng thành công mới có thể gửi đánh giá xác thực.
          </p>
        </div>
        {eligibility?.canReview && !composerOpen && (
          <Button
            type="button"
            data-testid="open-review-composer"
            onClick={openNewComposer}
          >
            Viết đánh giá
          </Button>
        )}
      </div>

      <Separator className="my-5" />

      <div className="grid gap-6 md:grid-cols-[240px_1fr]">
        <div className="rounded-2xl bg-muted/40 p-5 text-center">
          <div className="text-4xl font-extrabold tracking-tight" data-testid="reviews-avg-rating">
            {`${formatRating(summary.averageRating)} / 5`}
          </div>
          <div className="mt-2 flex justify-center">
            <StarRow rating={summary.averageRating} size="md" />
          </div>
          <p className="mt-2 text-xs text-muted-foreground" data-testid="reviews-total-count">
            {summary.totalReviews.toLocaleString("vi-VN")} đánh giá
          </p>
        </div>

        <div className="flex flex-col justify-center gap-2">
          {[5, 4, 3, 2, 1].map((star) => {
            const count = breakdownMap.get(star) ?? 0
            const pct =
              summary.totalReviews > 0
                ? Math.round((count / summary.totalReviews) * 100)
                : 0
            const active = ratingFilter === star
            return (
              <button
                key={star}
                type="button"
                onClick={() => {
                  setRatingFilter(active ? 0 : star)
                  setPage(1)
                }}
                className={cn(
                  "flex items-center gap-3 rounded-lg px-2 py-1 text-xs transition hover:bg-muted/60",
                  active && "bg-primary/10 font-semibold text-primary"
                )}
              >
                <span className="w-10 shrink-0 text-left">{star} sao</span>
                <div className="h-2 flex-1 overflow-hidden rounded-full bg-muted">
                  <div
                    className="h-full rounded-full bg-amber-400 transition-all"
                    style={{ width: `${pct}%` }}
                  />
                </div>
                <span className="w-10 shrink-0 text-right tabular-nums text-muted-foreground">
                  {count}
                </span>
              </button>
            )
          })}
        </div>
      </div>

      {composerOpen && (
        <form
          onSubmit={(e) => void handleSubmit(e)}
          data-testid="review-composer-form"
          className="mt-6 rounded-2xl border bg-background p-5"
        >
          <div className="flex items-center justify-between gap-2">
            <h3 className="text-sm font-bold">
              {editingReview ? "Chỉnh sửa đánh giá của bạn" : "Gửi đánh giá sản phẩm"}
            </h3>
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              aria-label="Đóng khung đánh giá"
              onClick={() => {
                setComposerOpen(false)
                setEditingReview(null)
              }}
            >
              <X className="size-4" />
            </Button>
          </div>

          {!editingReview && unreviewedOrders.length > 1 && (
            <div className="mt-4">
              <label
                htmlFor="review-order-select"
                className="mb-1.5 block text-xs font-medium text-muted-foreground"
              >
                Chọn đơn hàng đã giao
              </label>
              <select
                id="review-order-select"
                value={selectedOrderId}
                onChange={(e) => setSelectedOrderId(e.target.value)}
                className="h-9 w-full rounded-lg border bg-background px-3 text-sm"
              >
                {unreviewedOrders.map((ord) => (
                  <option key={ord.orderId} value={ord.orderId}>
                    {ord.orderCode || ord.orderId}
                    {ord.variantLabel ? ` · ${ord.variantLabel}` : ""}
                  </option>
                ))}
              </select>
            </div>
          )}

          <div className="mt-4">
            <span className="mb-1.5 block text-xs font-medium text-muted-foreground">
              Mức độ hài lòng
            </span>
            <div className="flex items-center gap-1.5" role="radiogroup" aria-label="Chọn số sao">
              {[1, 2, 3, 4, 5].map((star) => (
                <button
                  key={star}
                  type="button"
                  role="radio"
                  aria-checked={formRating === star}
                  aria-label={`${star} sao`}
                  onClick={() => setFormRating(star)}
                  className="rounded-md p-1 transition hover:scale-110"
                >
                  <Star
                    className={cn(
                      "size-6",
                      star <= formRating
                        ? "fill-amber-400 text-amber-400"
                        : "text-muted-foreground/30"
                    )}
                  />
                </button>
              ))}
              <span className="ml-2 text-xs font-medium text-muted-foreground">
                {formRating} / 5 sao
              </span>
            </div>
          </div>

          <div className="mt-4">
            <label
              htmlFor="review-comment-input"
              className="mb-1.5 block text-xs font-medium text-muted-foreground"
            >
              Nhận xét của bạn
            </label>
            <textarea
              id="review-comment-input"
              rows={3}
              maxLength={2000}
              value={formComment}
              onChange={(e) => setFormComment(e.target.value)}
              placeholder="Chia sẻ cảm nhận về chất lượng, đóng gói và trải nghiệm sử dụng..."
              className="w-full rounded-xl border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/30"
            />
          </div>

          <div className="mt-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <span className="text-xs font-medium text-muted-foreground">
                Hình ảnh thực tế ({formImages.length}/{MAX_REVIEW_IMAGES})
              </span>
              <input
                ref={fileInputRef}
                type="file"
                accept="image/jpeg,image/png,image/webp,image/avif"
                multiple
                className="hidden"
                data-testid="review-image-input"
                onChange={(e) => void handleFileSelect(e.target.files)}
              />
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={uploading || formImages.length >= MAX_REVIEW_IMAGES}
                onClick={() => fileInputRef.current?.click()}
              >
                {uploading ? (
                  <Loader2 className="size-3.5 animate-spin" />
                ) : (
                  <Camera className="size-3.5" />
                )}
                {uploading ? "Đang tải ảnh..." : "Thêm ảnh"}
              </Button>
            </div>
            {formImages.length > 0 && (
              <div className="mt-3 flex flex-wrap gap-2">
                {formImages.map((url, idx) => (
                  <div
                    key={`${url}-${idx}`}
                    className="relative size-16 overflow-hidden rounded-lg border bg-muted"
                  >
                    <img
                      src={url}
                      alt={`Ảnh đánh giá ${idx + 1}`}
                      className="size-full object-cover"
                    />
                    <button
                      type="button"
                      aria-label={`Xóa ảnh ${idx + 1}`}
                      onClick={() =>
                        setFormImages((prev) => prev.filter((_, i) => i !== idx))
                      }
                      className="absolute right-0.5 top-0.5 flex size-5 items-center justify-center rounded-full bg-black/70 text-white"
                    >
                      <X className="size-3" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="mt-5 flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setComposerOpen(false)
                setEditingReview(null)
              }}
            >
              Hủy
            </Button>
            <Button type="submit" disabled={submitting || uploading}>
              {submitting ? "Đang gửi..." : editingReview ? "Lưu thay đổi" : "Gửi đánh giá"}
            </Button>
          </div>
        </form>
      )}

      {eligibility && eligibility.myReviews.length > 0 && (
        <div className="mt-6 rounded-2xl border border-primary/20 bg-primary/5 p-4">
          <p className="text-xs font-semibold uppercase tracking-wide text-primary">
            Đánh giá của bạn ({eligibility.myReviews.length})
          </p>
          <div className="mt-3 space-y-3">
            {eligibility.myReviews.map((rev) => (
              <div
                key={rev.id}
                data-testid={`my-review-${rev.id}`}
                className="flex flex-wrap items-start justify-between gap-3 rounded-xl bg-background p-3.5 ring-1 ring-foreground/10"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <StarRow rating={rev.rating} />
                    {rev.variantLabel && (
                      <span className="text-xs text-muted-foreground">
                        {rev.variantLabel}
                      </span>
                    )}
                    {rev.hidden && (
                      <Badge variant="secondary" className="text-[11px]">
                        Đang tạm ẩn bởi quản trị viên
                      </Badge>
                    )}
                  </div>
                  {rev.comment && (
                    <p className="mt-1.5 text-sm text-foreground">{rev.comment}</p>
                  )}
                </div>
                <div className="flex items-center gap-1.5">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => openEditComposer(rev)}
                  >
                    <Edit3 className="size-3.5" />
                    Sửa
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    className="text-destructive hover:text-destructive"
                    disabled={deletingId === rev.id}
                    onClick={() => void handleDelete(rev.id)}
                  >
                    <Trash2 className="size-3.5" />
                    Xóa
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="mt-6 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            size="sm"
            variant={ratingFilter === 0 && !hasImagesOnly ? "default" : "outline"}
            onClick={() => {
              setRatingFilter(0)
              setHasImagesOnly(false)
              setPage(1)
            }}
          >
            Tất cả
          </Button>
          {[5, 4, 3, 2, 1].map((star) => (
            <Button
              key={star}
              type="button"
              size="sm"
              variant={ratingFilter === star ? "default" : "outline"}
              onClick={() => {
                setRatingFilter(ratingFilter === star ? 0 : star)
                setPage(1)
              }}
            >
              {star} sao
            </Button>
          ))}
          <Button
            type="button"
            size="sm"
            variant={hasImagesOnly ? "default" : "outline"}
            onClick={() => {
              setHasImagesOnly((prev) => !prev)
              setPage(1)
            }}
          >
            Có hình ảnh
          </Button>
        </div>

        <select
          aria-label="Sắp xếp đánh giá"
          value={sortBy}
          onChange={(e) => {
            setSortBy(e.target.value as "newest" | "highest" | "lowest")
            setPage(1)
          }}
          className="h-8 rounded-lg border bg-background px-2.5 text-xs"
        >
          <option value="newest">Mới nhất</option>
          <option value="highest">Sao cao đến thấp</option>
          <option value="lowest">Sao thấp đến cao</option>
        </select>
      </div>

      <div className="mt-5 divide-y">
        {loading ? (
          <div className="space-y-4 py-4">
            <Skeleton className="h-20 w-full rounded-xl" />
            <Skeleton className="h-20 w-full rounded-xl" />
          </div>
        ) : reviews.length === 0 ? (
          <div className="py-10 text-center text-sm text-muted-foreground">
            Chưa có đánh giá nào phù hợp bộ lọc hiện tại.
          </div>
        ) : (
          reviews.map((review) => (
            <article
              key={review.id}
              data-testid={`review-card-${review.id}`}
              className="py-5 first:pt-2 last:pb-0"
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-sm font-semibold text-foreground">
                    {review.authorName}
                  </span>
                  {review.verifiedPurchase && (
                    <Badge
                      variant="secondary"
                      className="gap-1 bg-emerald-500/10 text-[11px] font-medium text-emerald-700 dark:text-emerald-300"
                    >
                      <CheckCircle2 className="size-3" />
                      Đã mua hàng
                    </Badge>
                  )}
                  {review.variantLabel && (
                    <span className="text-xs text-muted-foreground">
                      Phân loại: {review.variantLabel}
                    </span>
                  )}
                </div>
                <span className="text-xs text-muted-foreground">
                  {formatDate(review.createdAt)}
                </span>
              </div>

              <div className="mt-1.5">
                <StarRow rating={review.rating} />
              </div>

              {review.comment && (
                <p className="mt-2.5 whitespace-pre-line text-sm leading-relaxed text-foreground/90">
                  {review.comment}
                </p>
              )}

              {review.images && review.images.length > 0 && (
                <div className="mt-3 flex flex-wrap gap-2">
                  {review.images.map((url, idx) => (
                    <button
                      key={`${url}-${idx}`}
                      type="button"
                      onClick={() => setPreviewImage(url)}
                      className="size-16 overflow-hidden rounded-lg border bg-muted transition hover:ring-2 hover:ring-primary"
                    >
                      <img
                        src={url}
                        alt={`Hình đánh giá ${idx + 1}`}
                        className="size-full object-cover"
                      />
                    </button>
                  ))}
                </div>
              )}

              {review.adminReply && (
                <div className="mt-3 rounded-xl bg-muted/50 p-3.5 text-sm">
                  <div className="flex items-center gap-1.5 text-xs font-semibold text-primary">
                    <MessageSquareReply className="size-3.5" />
                    Phản hồi từ DreamShop
                    {review.adminRepliedAt && (
                      <span className="font-normal text-muted-foreground">
                        · {formatDate(review.adminRepliedAt)}
                      </span>
                    )}
                  </div>
                  <p className="mt-1 whitespace-pre-line text-foreground/85">
                    {review.adminReply}
                  </p>
                </div>
              )}
            </article>
          ))
        )}
      </div>

      {totalPages > 1 && (
        <div className="mt-6 flex items-center justify-center gap-2">
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={page <= 1}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
          >
            Trang trước
          </Button>
          <span className="text-xs text-muted-foreground">
            Trang {page} / {totalPages}
          </span>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
          >
            Trang sau
          </Button>
        </div>
      )}

      {previewImage && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Xem ảnh đánh giá"
          onClick={() => setPreviewImage(null)}
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4"
        >
          <div className="relative max-h-[85vh] max-w-3xl" onClick={(e) => e.stopPropagation()}>
            <Button
              type="button"
              size="icon"
              variant="secondary"
              aria-label="Đóng ảnh"
              onClick={() => setPreviewImage(null)}
              className="absolute -right-3 -top-3 rounded-full"
            >
              <X className="size-4" />
            </Button>
            <img
              src={previewImage}
              alt="Ảnh đánh giá phóng to"
              className="max-h-[85vh] rounded-xl object-contain"
            />
          </div>
        </div>
      )}
    </section>
  )
}
