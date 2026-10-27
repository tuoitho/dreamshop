import { AlertCircle, Film, Loader2, Maximize2, Pencil, Play } from "lucide-react"
import { useEffect, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { getAdminProduct, toClientError } from "@/lib/api"
import { formatCurrencyVnd, formatNumber } from "@/lib/format"
import type { AdminProductDetail } from "@/lib/types"
import { cn } from "@/lib/utils"
import { MediaLightboxDialog, type MediaPreviewItem } from "./media-lightbox-dialog"

interface ProductDetailSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  productId: string | null
  canUpdate: boolean
  onEdit: (productId: string) => void
}

export function ProductDetailSheet({
  open,
  onOpenChange,
  productId,
  canUpdate,
  onEdit,
}: ProductDetailSheetProps) {
  const [detail, setDetail] = useState<AdminProductDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [selectedMediaIdx, setSelectedMediaIdx] = useState(0)
  const [lightboxIdx, setLightboxIdx] = useState<number | null>(null)

  useEffect(() => {
    if (!open || !productId) {
      setDetail(null)
      setLightboxIdx(null)
      return
    }
    const controller = new AbortController()
    setDetail(null)
    setLoading(true)
    setErrorMessage(null)
    setSelectedMediaIdx(0)
    setLightboxIdx(null)

    void (async () => {
      try {
        const res = await getAdminProduct(productId, controller.signal)
        if (controller.signal.aborted) return
        setDetail(res)
      } catch (error) {
        if (!controller.signal.aborted) {
          setErrorMessage(toClientError(error).message)
        }
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false)
        }
      }
    })()

    return () => controller.abort()
  }, [open, productId])

  const mediaItems: MediaPreviewItem[] = detail
    ? [
        ...(detail.images ?? []).map((url) => ({ url, kind: "IMAGE" as const })),
        ...(detail.videos ?? []).map((url) => ({ url, kind: "VIDEO" as const })),
      ]
    : []
  const firstImageIdx = mediaItems.findIndex((item) => item.kind === "IMAGE")
  const activeMedia = mediaItems[selectedMediaIdx] ?? mediaItems[0] ?? null

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent className="sm:max-w-[640px]">
          <SheetHeader>
            <SheetTitle>Chi tiết sản phẩm</SheetTitle>
            <SheetDescription>
              {detail
                ? `${detail.brand} · Mã ${detail.id}`
                : productId
                  ? `Đang tra cứu mã sản phẩm ${productId}…`
                  : "Thông tin thương mại và bộ sưu tập hình ảnh/video."}
            </SheetDescription>
          </SheetHeader>

          {loading ? (
            <div className="flex flex-1 items-center justify-center p-10 text-sm text-muted-foreground">
              <Loader2 className="mr-2 size-4 animate-spin" />
              Đang tải chi tiết sản phẩm…
            </div>
          ) : errorMessage ? (
            <div className="p-6">
              <div
                role="alert"
                className="flex items-start gap-2.5 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-xs font-medium text-red-900"
              >
                <AlertCircle className="mt-0.5 size-4 shrink-0 text-red-600" />
                <span>{errorMessage}</span>
              </div>
            </div>
          ) : detail ? (
            <>
              <div className="flex-1 space-y-5 overflow-y-auto px-6 py-5">
                {mediaItems.length > 0 ? (
                  <div className="space-y-3">
                    <div className="relative flex h-64 items-center justify-center overflow-hidden rounded-xl border border-border bg-slate-950/5 p-2 sm:h-72">
                      {activeMedia?.kind === "IMAGE" ? (
                        <button
                          type="button"
                          onClick={() => setLightboxIdx(selectedMediaIdx)}
                          className="flex h-full w-full cursor-zoom-in items-center justify-center"
                        >
                          <img
                            src={activeMedia.url}
                            alt={detail.name}
                            className="max-h-full max-w-full rounded-lg object-contain"
                          />
                        </button>
                      ) : activeMedia?.kind === "VIDEO" ? (
                        <video
                          key={activeMedia.url}
                          data-testid="detail-video-player"
                          src={activeMedia.url}
                          controls
                          playsInline
                          className="h-full w-full rounded-lg bg-black object-contain"
                        />
                      ) : null}

                      <Button
                        type="button"
                        size="sm"
                        variant="secondary"
                        aria-label="Xem toàn màn hình"
                        onClick={() => setLightboxIdx(selectedMediaIdx)}
                        className="absolute top-3 right-3 shadow-sm"
                      >
                        <Maximize2 className="size-3.5" />
                        Toàn màn hình
                      </Button>
                    </div>

                    {mediaItems.length > 1 && (
                      <div className="flex flex-wrap gap-2">
                        {mediaItems.map((item, idx) => {
                          const label =
                            item.kind === "VIDEO"
                              ? "Video"
                              : idx === firstImageIdx
                                ? "Ảnh bìa"
                                : "Ảnh"
                          const isSelected = idx === selectedMediaIdx
                          return (
                            <button
                              key={`${item.url}-${idx}`}
                              type="button"
                              aria-label={`Chọn media ${idx + 1} (${label})`}
                              onClick={() => setSelectedMediaIdx(idx)}
                              className={cn(
                                "relative flex size-16 cursor-pointer items-center justify-center overflow-hidden rounded-lg border bg-card p-1 transition-all",
                                isSelected
                                  ? "border-primary ring-2 ring-primary/40"
                                  : "border-border opacity-80 hover:opacity-100"
                              )}
                            >
                              {item.kind === "IMAGE" ? (
                                <img
                                  src={item.url}
                                  alt={`${detail.name} ${idx + 1}`}
                                  className="max-h-full max-w-full object-contain"
                                />
                              ) : (
                                <div className="flex h-full w-full flex-col items-center justify-center gap-0.5 rounded bg-sidebar text-emerald-400">
                                  <Film className="size-4" />
                                  <Play className="size-2.5 fill-current" />
                                </div>
                              )}
                            </button>
                          )
                        })}
                      </div>
                    )}
                  </div>
                ) : (
                  <div className="flex h-36 items-center justify-center rounded-xl border border-dashed border-border bg-muted/40 text-3xl">
                    {detail.emoji || "📦"}
                  </div>
                )}

                <div className="space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge variant="outline">{detail.categoryName || detail.categoryId}</Badge>
                    {detail.deleted ? (
                      <Badge variant="secondary">Đã ẩn</Badge>
                    ) : (
                      <Badge variant="success">Đang bán</Badge>
                    )}
                    {detail.isNew && <Badge variant="default">Sản phẩm mới</Badge>}
                    {detail.featured && <Badge variant="outline">Ưu tiên trang chủ</Badge>}
                    {detail.badge && <Badge variant="warning">{detail.badge}</Badge>}
                  </div>

                  <h3 className="text-lg font-semibold text-foreground">{detail.name}</h3>
                  {detail.shortDescription && (
                    <p className="text-sm text-muted-foreground">{detail.shortDescription}</p>
                  )}
                </div>

                <div className="grid grid-cols-2 gap-3 rounded-xl border border-border bg-background p-4 sm:grid-cols-3">
                  <div>
                    <div className="text-xs text-muted-foreground">Giá bán</div>
                    <div className="mt-0.5 font-mono text-base font-semibold text-foreground tabular-nums">
                      {formatCurrencyVnd(detail.price)}
                    </div>
                    {detail.originalPrice > detail.price && (
                      <div className="font-mono text-xs text-muted-foreground line-through tabular-nums">
                        {formatCurrencyVnd(detail.originalPrice)}
                      </div>
                    )}
                  </div>

                  <div>
                    <div className="text-xs text-muted-foreground">Tồn khả dụng</div>
                    <div className="mt-0.5 font-mono text-base font-semibold text-foreground tabular-nums">
                      {detail.stock ? formatNumber(detail.stock.availableQuantity) : "—"}
                    </div>
                    {detail.stock && (
                      <div className="font-mono text-[11px] text-muted-foreground tabular-nums">
                        Tổng {formatNumber(detail.stock.quantity)} · Giữ chỗ{" "}
                        {formatNumber(detail.stock.reserved)}
                      </div>
                    )}
                  </div>

                  <div>
                    <div className="text-xs text-muted-foreground">Tài nguyên media</div>
                    <div className="mt-0.5 font-mono text-sm font-semibold text-foreground tabular-nums">
                      {detail.images?.length ?? 0} ảnh · {detail.videos?.length ?? 0} video
                    </div>
                  </div>
                </div>

                {detail.variants && detail.variants.length > 0 && (
                  <div className="space-y-2">
                    <div className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                      Phân loại / Biến thể ({detail.variants.length})
                    </div>
                    <div className="flex flex-wrap gap-1.5">
                      {detail.variants.map((variant, idx) => (
                        <span
                          key={`${variant.label}-${idx}`}
                          className="inline-flex items-center gap-1.5 rounded-full border border-border bg-background px-2.5 py-1 text-xs"
                        >
                          {variant.colorHex && (
                            <span
                              className="size-2.5 rounded-full border border-black/15"
                              style={{ backgroundColor: variant.colorHex }}
                            />
                          )}
                          <span>{variant.label}</span>
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {detail.description && (
                  <div className="space-y-1.5">
                    <div className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                      Mô tả chi tiết
                    </div>
                    <p className="text-sm leading-relaxed whitespace-pre-line text-foreground">
                      {detail.description}
                    </p>
                  </div>
                )}

                {detail.details && detail.details.length > 0 && (
                  <div className="space-y-1.5">
                    <div className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                      Thông số nổi bật
                    </div>
                    <ul className="list-inside list-disc space-y-1 text-sm text-foreground">
                      {detail.details.map((line, idx) => (
                        <li key={`${line}-${idx}`}>{line}</li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>

              <SheetFooter>
                <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                  Đóng
                </Button>
                {canUpdate && (
                  <Button
                    type="button"
                    onClick={() => {
                      onOpenChange(false)
                      onEdit(detail.id)
                    }}
                  >
                    <Pencil className="size-4" />
                    Chỉnh sửa sản phẩm
                  </Button>
                )}
              </SheetFooter>
            </>
          ) : null}
        </SheetContent>
      </Sheet>

      <MediaLightboxDialog
        items={mediaItems}
        activeIndex={lightboxIdx}
        productTitle={detail?.name}
        onClose={() => setLightboxIdx(null)}
        onSelectIndex={setLightboxIdx}
      />
    </>
  )
}
