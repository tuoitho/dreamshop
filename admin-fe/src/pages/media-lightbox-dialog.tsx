import { Dialog as DialogPrimitive } from "radix-ui"
import { ChevronLeft, ChevronRight, Film, Play, X } from "lucide-react"
import { useEffect } from "react"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

export interface MediaPreviewItem {
  url: string
  kind: "IMAGE" | "VIDEO"
}

interface MediaLightboxDialogProps {
  items: MediaPreviewItem[]
  activeIndex: number | null
  productTitle?: string
  onClose: () => void
  onSelectIndex: (index: number) => void
}

export function MediaLightboxDialog({
  items,
  activeIndex,
  productTitle,
  onClose,
  onSelectIndex,
}: MediaLightboxDialogProps) {
  const open = activeIndex !== null && activeIndex >= 0 && activeIndex < items.length
  const activeItem = open ? items[activeIndex] : null
  const firstImageIdx = items.findIndex((item) => item.kind === "IMAGE")

  useEffect(() => {
    if (!open || items.length <= 1 || activeIndex === null) return
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "ArrowLeft") {
        event.preventDefault()
        onSelectIndex((activeIndex - 1 + items.length) % items.length)
      } else if (event.key === "ArrowRight") {
        event.preventDefault()
        onSelectIndex((activeIndex + 1) % items.length)
      }
    }
    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [open, items.length, activeIndex, onSelectIndex])

  return (
    <DialogPrimitive.Root open={open} onOpenChange={(nextOpen) => !nextOpen && onClose()}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-60 bg-black/80 backdrop-blur-xs" />
        <DialogPrimitive.Content
          data-slot="media-lightbox-content"
          className="fixed top-1/2 left-1/2 z-60 flex max-h-[92vh] w-[min(94vw,960px)] -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-2xl border border-white/15 bg-slate-950 text-white shadow-2xl outline-none"
        >
          <div className="flex items-center justify-between gap-3 border-b border-white/10 px-5 py-3.5">
            <div className="flex min-w-0 items-center gap-2.5">
              <DialogPrimitive.Title className="truncate text-sm font-semibold text-white">
                {productTitle || "Xem chi tiết media"}
              </DialogPrimitive.Title>
              {activeItem && activeIndex !== null && (
                <>
                  <Badge variant={activeItem.kind === "VIDEO" ? "default" : "secondary"}>
                    {activeItem.kind === "VIDEO"
                      ? "Video"
                      : activeIndex === firstImageIdx
                        ? "Ảnh bìa"
                        : "Ảnh"}
                  </Badge>
                  <span className="font-mono text-xs text-slate-300 tabular-nums">
                    {activeIndex + 1} / {items.length}
                  </span>
                </>
              )}
            </div>
            <DialogPrimitive.Description className="sr-only">
              Xem chi tiết hình ảnh độ phân giải đầy đủ và phát video sản phẩm.
            </DialogPrimitive.Description>
            <DialogPrimitive.Close
              aria-label="Đóng xem trước media"
              className="inline-flex size-8 cursor-pointer items-center justify-center rounded-lg text-slate-300 transition-colors hover:bg-white/10 hover:text-white focus-visible:ring-2 focus-visible:ring-emerald-400 focus-visible:outline-none"
            >
              <X className="size-4" />
            </DialogPrimitive.Close>
          </div>

          <div className="relative flex min-h-[320px] flex-1 items-center justify-center bg-black/70 p-4 sm:min-h-[440px]">
            {activeItem?.kind === "IMAGE" && activeIndex !== null && (
              <img
                data-testid="lightbox-image-viewer"
                src={activeItem.url}
                alt={`${productTitle || "Sản phẩm"} - ảnh ${activeIndex + 1}`}
                className="max-h-[66vh] max-w-full rounded-lg object-contain select-none"
              />
            )}

            {activeItem?.kind === "VIDEO" && (
              <video
                key={activeItem.url}
                data-testid="lightbox-video-player"
                src={activeItem.url}
                controls
                autoPlay
                playsInline
                className="max-h-[66vh] w-full rounded-lg bg-black object-contain"
              />
            )}

            {items.length > 1 && activeIndex !== null && (
              <>
                <button
                  type="button"
                  aria-label="Media trước"
                  onClick={() => onSelectIndex((activeIndex - 1 + items.length) % items.length)}
                  className="absolute top-1/2 left-3 inline-flex size-10 -translate-y-1/2 cursor-pointer items-center justify-center rounded-full border border-white/15 bg-slate-900/85 text-white shadow-lg transition-colors hover:bg-slate-800 focus-visible:ring-2 focus-visible:ring-emerald-400 focus-visible:outline-none"
                >
                  <ChevronLeft className="size-5" />
                </button>
                <button
                  type="button"
                  aria-label="Media tiếp theo"
                  onClick={() => onSelectIndex((activeIndex + 1) % items.length)}
                  className="absolute top-1/2 right-3 inline-flex size-10 -translate-y-1/2 cursor-pointer items-center justify-center rounded-full border border-white/15 bg-slate-900/85 text-white shadow-lg transition-colors hover:bg-slate-800 focus-visible:ring-2 focus-visible:ring-emerald-400 focus-visible:outline-none"
                >
                  <ChevronRight className="size-5" />
                </button>
              </>
            )}
          </div>

          {items.length > 1 && (
            <div className="flex items-center gap-2 overflow-x-auto border-t border-white/10 bg-slate-950 px-5 py-3">
              {items.map((item, idx) => {
                const isSelected = idx === activeIndex
                return (
                  <button
                    key={`${item.url}-${idx}`}
                    type="button"
                    aria-label={`Chuyển đến media ${idx + 1}`}
                    onClick={() => onSelectIndex(idx)}
                    className={cn(
                      "relative flex size-14 shrink-0 cursor-pointer items-center justify-center overflow-hidden rounded-lg border transition-all",
                      isSelected
                        ? "border-emerald-400 ring-2 ring-emerald-400/60"
                        : "border-white/15 opacity-70 hover:opacity-100"
                    )}
                  >
                    {item.kind === "IMAGE" ? (
                      <img
                        src={item.url}
                        alt=""
                        className="h-full w-full bg-slate-900 object-cover"
                      />
                    ) : (
                      <div className="flex h-full w-full flex-col items-center justify-center gap-0.5 bg-slate-900 text-emerald-400">
                        <Film className="size-4" />
                        <Play className="size-2.5 fill-current" />
                      </div>
                    )}
                  </button>
                )
              })}
            </div>
          )}
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  )
}
