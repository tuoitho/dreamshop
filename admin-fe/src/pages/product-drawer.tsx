import { zodResolver } from "@hookform/resolvers/zod"
import {
  AlertCircle,
  CheckCircle2,
  Film,
  Loader2,
  Maximize2,
  Palette,
  Play,
  Plus,
  RefreshCw,
  UploadCloud,
  X,
} from "lucide-react"
import { useEffect, useState, type ChangeEvent } from "react"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import {
  createAdminProduct,
  getAdminProduct,
  setProductStock,
  toClientError,
  updateAdminProduct,
} from "@/lib/api"
import { CATEGORY_PRESETS } from "@/lib/categories"
import { formatNumber } from "@/lib/format"
import { ALLOWED_VIDEO_MIME_TYPES, uploadProductMediaFile } from "@/lib/media-upload"
import type {
  AdminCategoryMetric,
  AdminProductMutationPayload,
  ProductVariant,
} from "@/lib/types"
import { cn } from "@/lib/utils"
import { MediaLightboxDialog, type MediaPreviewItem } from "./media-lightbox-dialog"

const HEX_COLOR_PATTERN = /^#[0-9a-fA-F]{6}$/
const MAX_IMAGES_COUNT = 10
const MAX_VIDEOS_COUNT = 3
const MAX_VARIANTS_COUNT = 20

const VARIANT_COLOR_PRESETS = [
  { name: "Đen Titan", hex: "#1c1c1e" },
  { name: "Trắng Sứ", hex: "#f8fafc" },
  { name: "Bạc Ánh Kim", hex: "#cbd5e1" },
  { name: "Xám Không Gian", hex: "#64748b" },
  { name: "Xanh Ngọc", hex: "#0d9488" },
  { name: "Xanh Dương", hex: "#2563eb" },
  { name: "Vàng Sa Mạc", hex: "#d6c7b5" },
  { name: "Cam Đất", hex: "#ea580c" },
  { name: "Đỏ Ruby", hex: "#dc2626" },
  { name: "Hồng Đào", hex: "#db2777" },
  { name: "Nâu Da", hex: "#7c4a21" },
] as const

const productFormSchema = z.object({
  name: z.string().trim().min(1, "Vui lòng nhập tên sản phẩm."),
  brand: z.string().trim().min(1, "Vui lòng nhập thương hiệu."),
  categoryId: z.string().trim().min(1, "Vui lòng chọn danh mục."),
  categoryName: z.string().trim().min(1, "Vui lòng nhập tên danh mục."),
  price: z.string().trim().min(1, "Vui lòng nhập giá bán."),
  originalPrice: z.string().trim(),
  shortDescription: z.string(),
  description: z.string(),
  badge: z.string(),
  emoji: z.string(),
  isNew: z.boolean(),
  featured: z.boolean(),
  detailsText: z.string(),
  initialStock: z.string().trim(),
})

type ProductFormValues = z.infer<typeof productFormSchema>

interface PendingStockRetry {
  productId: string
  productName: string
  quantity: number
  errorMessage: string
}

interface ProductDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  productId: string | null
  canUpdateInventory: boolean
  categories?: AdminCategoryMetric[]
  onSaved: () => void
}

const DEFAULT_VALUES: ProductFormValues = {
  name: "",
  brand: "",
  categoryId: CATEGORY_PRESETS[0].id,
  categoryName: CATEGORY_PRESETS[0].name,
  price: "0",
  originalPrice: "",
  shortDescription: "",
  description: "",
  badge: "",
  emoji: CATEGORY_PRESETS[0].emoji,
  isNew: false,
  featured: false,
  detailsText: "",
  initialStock: "",
}

export function ProductDrawer({
  open,
  onOpenChange,
  productId,
  canUpdateInventory,
  categories,
  onSaved,
}: ProductDrawerProps) {
  const isEdit = Boolean(productId)
  const [loadingDetail, setLoadingDetail] = useState(false)
  const [uploadingCount, setUploadingCount] = useState(0)
  const [mediaList, setMediaList] = useState<MediaPreviewItem[]>([])
  const [previewMediaIndex, setPreviewMediaIndex] = useState<number | null>(null)
  const [variants, setVariants] = useState<ProductVariant[]>([{ label: "Tiêu chuẩn" }])
  const [variantLabelInput, setVariantLabelInput] = useState("")
  const [variantColorInput, setVariantColorInput] = useState("")
  const [imageHue, setImageHue] = useState<number[]>([165, 35])
  const [serverError, setServerError] = useState<string | null>(null)
  const [pendingStockRetry, setPendingStockRetry] = useState<PendingStockRetry | null>(null)
  const [retryingStock, setRetryingStock] = useState(false)

  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<ProductFormValues>({
    resolver: zodResolver(productFormSchema),
    defaultValues: DEFAULT_VALUES,
  })

  const watchedIsNew = watch("isNew")
  const watchedFeatured = watch("featured")
  const watchedCategoryId = watch("categoryId")
  const watchedCategoryName = watch("categoryName")

  const categoryOptions = (() => {
    const map = new Map<string, { id: string; name: string; emoji: string }>()
    for (const preset of CATEGORY_PRESETS) {
      map.set(preset.id, preset)
    }
    for (const cat of categories ?? []) {
      if (!map.has(cat.categoryId)) {
        map.set(cat.categoryId, {
          id: cat.categoryId,
          name: cat.categoryName || cat.categoryId,
          emoji: "📦",
        })
      }
    }
    if (watchedCategoryId && !map.has(watchedCategoryId)) {
      map.set(watchedCategoryId, {
        id: watchedCategoryId,
        name: watchedCategoryName || watchedCategoryId,
        emoji: "📦",
      })
    }
    return Array.from(map.values())
  })()

  useEffect(() => {
    if (!open) {
      setPreviewMediaIndex(null)
      return
    }
    setServerError(null)
    setPendingStockRetry(null)
    setPreviewMediaIndex(null)
    setVariantLabelInput("")
    setVariantColorInput("")

    if (!productId) {
      reset(DEFAULT_VALUES)
      setMediaList([])
      setVariants([{ label: "Tiêu chuẩn" }])
      setImageHue([165, 35])
      return
    }

    const controller = new AbortController()
    setLoadingDetail(true)
    void (async () => {
      try {
        const detail = await getAdminProduct(productId, controller.signal)
        if (controller.signal.aborted) return
        reset({
          name: detail.name,
          brand: detail.brand,
          categoryId: detail.categoryId,
          categoryName: detail.categoryName,
          price: String(detail.price),
          originalPrice: detail.originalPrice > 0 ? String(detail.originalPrice) : "",
          shortDescription: detail.shortDescription ?? "",
          description: detail.description ?? "",
          badge: detail.badge ?? "",
          emoji: detail.emoji || "📦",
          isNew: detail.isNew,
          featured: detail.featured,
          detailsText: (detail.details ?? []).join("\n"),
          initialStock: "",
        })
        setVariants(detail.variants ?? [])
        setImageHue(detail.imageHue?.length ? detail.imageHue : [165, 35])
        const combined: MediaPreviewItem[] = [
          ...(detail.images ?? []).map((url) => ({ url, kind: "IMAGE" as const })),
          ...(detail.videos ?? []).map((url) => ({ url, kind: "VIDEO" as const })),
        ]
        setMediaList(combined)
      } catch (error) {
        if (!controller.signal.aborted) {
          setServerError(toClientError(error).message)
        }
      } finally {
        if (!controller.signal.aborted) {
          setLoadingDetail(false)
        }
      }
    })()

    return () => controller.abort()
  }, [open, productId, reset])

  const handleCategoryChange = (nextCategoryId: string) => {
    setValue("categoryId", nextCategoryId, { shouldValidate: true })
    const selected = categoryOptions.find((item) => item.id === nextCategoryId)
    if (selected) {
      setValue("categoryName", selected.name, { shouldValidate: true })
      setValue("emoji", selected.emoji)
    }
  }

  const handleFilesSelected = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files ? Array.from(event.target.files) : []
    event.target.value = ""
    if (files.length === 0) return

    setServerError(null)
    let imageCount = mediaList.filter((m) => m.kind === "IMAGE").length
    let videoCount = mediaList.filter((m) => m.kind === "VIDEO").length
    const acceptedFiles: File[] = []

    for (const file of files) {
      const normalizedMime = file.type.trim().toLowerCase()
      const isVideo = (ALLOWED_VIDEO_MIME_TYPES as readonly string[]).includes(normalizedMime)
      if (isVideo) {
        if (videoCount >= MAX_VIDEOS_COUNT) {
          setServerError(`Tối đa ${MAX_VIDEOS_COUNT} video cho mỗi sản phẩm.`)
          continue
        }
        videoCount++
      } else {
        if (imageCount >= MAX_IMAGES_COUNT) {
          setServerError(`Tối đa ${MAX_IMAGES_COUNT} ảnh cho mỗi sản phẩm.`)
          continue
        }
        imageCount++
      }
      acceptedFiles.push(file)
    }

    if (acceptedFiles.length === 0) return
    setUploadingCount((c) => c + acceptedFiles.length)

    for (const file of acceptedFiles) {
      try {
        const uploaded = await uploadProductMediaFile(file)
        setMediaList((prev) => [
          ...prev,
          { url: uploaded.publicUrl, kind: uploaded.mediaType },
        ])
      } catch (error) {
        setServerError(toClientError(error).message)
      } finally {
        setUploadingCount((c) => Math.max(0, c - 1))
      }
    }
  }

  const handleRemoveMedia = (index: number) => {
    setMediaList((prev) => prev.filter((_, idx) => idx !== index))
    setPreviewMediaIndex((prev) => {
      if (prev === null) return null
      if (prev === index) return null
      return prev > index ? prev - 1 : prev
    })
  }

  const handleAddVariant = () => {
    const label = variantLabelInput.trim()
    if (!label) return
    const colorHex = variantColorInput.trim().toLowerCase()
    if (colorHex && !HEX_COLOR_PATTERN.test(colorHex)) {
      setServerError("Mã màu biến thể phải theo định dạng hex 6 ký tự (ví dụ: #1c1c1e).")
      return
    }
    if (variants.length >= MAX_VARIANTS_COUNT) {
      setServerError(`Tối đa ${MAX_VARIANTS_COUNT} biến thể cho mỗi sản phẩm.`)
      return
    }
    setServerError(null)
    setVariants((prev) => [...prev, colorHex ? { label, colorHex } : { label }])
    setVariantLabelInput("")
    setVariantColorInput("")
  }

  const handleRetryStock = async () => {
    if (!pendingStockRetry) return
    setRetryingStock(true)
    setServerError(null)
    try {
      await setProductStock(pendingStockRetry.productId, pendingStockRetry.quantity)
      setPendingStockRetry(null)
      onSaved()
      onOpenChange(false)
    } catch (error) {
      setPendingStockRetry({
        ...pendingStockRetry,
        errorMessage: toClientError(error).message,
      })
    } finally {
      setRetryingStock(false)
    }
  }

  const onSubmit = async (values: ProductFormValues) => {
    setServerError(null)
    const parsedPrice = Number(values.price)
    if (!Number.isFinite(parsedPrice) || parsedPrice <= 0) {
      setServerError("Giá bán phải lớn hơn 0.")
      return
    }
    const parsedOriginal =
      values.originalPrice !== "" ? Number(values.originalPrice) : parsedPrice
    if (!Number.isFinite(parsedOriginal) || parsedOriginal < parsedPrice) {
      setServerError("Giá niêm yết gốc phải để trống hoặc lớn hơn / bằng giá bán.")
      return
    }

    const effectiveVariants: ProductVariant[] = [...variants]
    const pendingLabel = variantLabelInput.trim()
    if (pendingLabel) {
      const pendingColor = variantColorInput.trim().toLowerCase()
      if (pendingColor && !HEX_COLOR_PATTERN.test(pendingColor)) {
        setServerError("Mã màu biến thể phải theo định dạng hex 6 ký tự (ví dụ: #1c1c1e).")
        return
      }
      effectiveVariants.push(
        pendingColor ? { label: pendingLabel, colorHex: pendingColor } : { label: pendingLabel }
      )
    }
    if (effectiveVariants.length === 0 || effectiveVariants.length > MAX_VARIANTS_COUNT) {
      setServerError(`Sản phẩm cần có từ 1 đến ${MAX_VARIANTS_COUNT} biến thể.`)
      return
    }

    const hasInitialStock =
      !isEdit && canUpdateInventory && values.initialStock.trim() !== ""
    const parsedInitialStock = hasInitialStock ? Number.parseInt(values.initialStock, 10) : 0
    if (hasInitialStock && (Number.isNaN(parsedInitialStock) || parsedInitialStock < 0)) {
      setServerError("Tồn kho ban đầu phải là số nguyên không âm.")
      return
    }

    const payload: AdminProductMutationPayload = {
      name: values.name.trim(),
      brand: values.brand.trim(),
      categoryId: values.categoryId.trim(),
      categoryName: values.categoryName.trim(),
      price: Math.round(parsedPrice),
      originalPrice: Math.round(parsedOriginal),
      shortDescription: values.shortDescription.trim(),
      description: values.description.trim(),
      images: mediaList.filter((m) => m.kind === "IMAGE").map((m) => m.url),
      videos: mediaList.filter((m) => m.kind === "VIDEO").map((m) => m.url),
      isNew: values.isNew,
      badge: values.badge.trim(),
      featured: values.featured,
      emoji: values.emoji.trim() || "📦",
      imageHue,
      variants: effectiveVariants,
      details: values.detailsText
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean),
    }

    try {
      if (isEdit && productId) {
        await updateAdminProduct(productId, payload)
        onSaved()
        onOpenChange(false)
        return
      }

      const created = await createAdminProduct(payload)
      onSaved()

      if (hasInitialStock && parsedInitialStock > 0) {
        try {
          await setProductStock(created.id, parsedInitialStock)
          onSaved()
          onOpenChange(false)
        } catch (stockError) {
          setPendingStockRetry({
            productId: created.id,
            productName: created.name,
            quantity: parsedInitialStock,
            errorMessage: toClientError(stockError).message,
          })
        }
        return
      }

      onOpenChange(false)
    } catch (error) {
      setServerError(toClientError(error).message)
    }
  }

  const firstImageIdx = mediaList.findIndex((m) => m.kind === "IMAGE")
  const normalizedVariantColor = variantColorInput.trim().toLowerCase()
  const matchedColorPreset = VARIANT_COLOR_PRESETS.find(
    (preset) => preset.hex === normalizedVariantColor
  )

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-[640px]">
        <SheetHeader>
          <SheetTitle>{isEdit ? "Chỉnh sửa sản phẩm" : "Thêm sản phẩm mới"}</SheetTitle>
          <SheetDescription>
            {isEdit
              ? `Cập nhật thông tin hiển thị và tài nguyên media cho mã ${productId}.`
              : "Khởi tạo sản phẩm mới và tải ảnh/video trực tiếp lên kho lưu trữ đối tượng S3/MinIO."}
          </SheetDescription>
        </SheetHeader>

        {pendingStockRetry ? (
          <div className="flex flex-1 flex-col justify-between p-6">
            <div className="space-y-4">
              <div className="flex items-start gap-3 rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-900">
                <CheckCircle2 className="mt-0.5 size-5 shrink-0 text-emerald-600" />
                <div>
                  <div className="font-semibold">
                    Đã tạo sản phẩm “{pendingStockRetry.productName}”
                  </div>
                  <div className="mt-0.5 font-mono text-xs text-emerald-800">
                    Mã sản phẩm: {pendingStockRetry.productId}
                  </div>
                </div>
              </div>

              <div
                role="alert"
                className="flex items-start gap-3 rounded-xl border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950"
              >
                <AlertCircle className="mt-0.5 size-5 shrink-0 text-amber-700" />
                <div className="space-y-1">
                  <div className="font-semibold">
                    Chưa thể ghi nhận mức tồn kho ban đầu ({formatNumber(pendingStockRetry.quantity)}{" "}
                    đơn vị)
                  </div>
                  <p className="text-xs leading-relaxed text-amber-900">
                    {pendingStockRetry.errorMessage}
                  </p>
                  <p className="text-xs text-amber-800">
                    Sản phẩm đã được lưu trong danh mục. Nhấn thử lại để chỉ cập nhật tồn kho mà
                    không tạo trùng sản phẩm.
                  </p>
                </div>
              </div>
            </div>

            <div className="flex items-center justify-end gap-2 border-t border-border pt-4">
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setPendingStockRetry(null)
                  onOpenChange(false)
                }}
              >
                Đóng và điều chỉnh sau
              </Button>
              <Button
                type="button"
                onClick={() => void handleRetryStock()}
                disabled={retryingStock}
              >
                <RefreshCw className={retryingStock ? "size-4 animate-spin" : "size-4"} />
                Thử lại cập nhật tồn kho
              </Button>
            </div>
          </div>
        ) : loadingDetail ? (
          <div className="flex flex-1 items-center justify-center p-10 text-sm text-muted-foreground">
            <Loader2 className="mr-2 size-4 animate-spin" />
            Đang tải chi tiết sản phẩm…
          </div>
        ) : (
          <form
            onSubmit={handleSubmit(onSubmit)}
            className="flex min-h-0 flex-1 flex-col"
            noValidate
          >
            <div className="flex-1 space-y-5 overflow-y-auto px-6 py-5">
              {serverError && (
                <div
                  role="alert"
                  className="flex items-start gap-2.5 rounded-lg border border-red-200 bg-red-50 px-3.5 py-2.5 text-xs font-medium text-red-900"
                >
                  <AlertCircle className="mt-0.5 size-4 shrink-0 text-red-600" />
                  <span>{serverError}</span>
                </div>
              )}

              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-1.5 sm:col-span-2">
                  <Label htmlFor="prod-name">Tên sản phẩm *</Label>
                  <Input
                    id="prod-name"
                    placeholder="Ví dụ: MacBook Pro 14 M4"
                    aria-invalid={Boolean(errors.name)}
                    {...register("name")}
                  />
                  {errors.name && (
                    <p className="text-xs font-medium text-red-700">{errors.name.message}</p>
                  )}
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="prod-brand">Thương hiệu *</Label>
                  <Input
                    id="prod-brand"
                    placeholder="Ví dụ: Apple"
                    aria-invalid={Boolean(errors.brand)}
                    {...register("brand")}
                  />
                  {errors.brand && (
                    <p className="text-xs font-medium text-red-700">{errors.brand.message}</p>
                  )}
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="prod-category">Danh mục *</Label>
                  <select
                    id="prod-category"
                    value={watchedCategoryId}
                    onChange={(e) => handleCategoryChange(e.target.value)}
                    className="flex h-9 w-full rounded-lg border border-input bg-card px-3 py-1.5 text-sm text-foreground outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40"
                  >
                    {categoryOptions.map((cat) => (
                      <option key={cat.id} value={cat.id}>
                        {cat.emoji} {cat.name}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="prod-price">Giá bán (VND) *</Label>
                  <Input
                    id="prod-price"
                    type="number"
                    min={0}
                    step={1000}
                    className="font-mono tabular-nums"
                    {...register("price")}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="prod-original-price">Giá niêm yết gốc (VND)</Label>
                  <Input
                    id="prod-original-price"
                    type="number"
                    min={0}
                    step={1000}
                    placeholder="Để trống nếu bằng giá bán"
                    className="font-mono tabular-nums"
                    {...register("originalPrice")}
                  />
                </div>

                {!isEdit && canUpdateInventory && (
                  <div className="space-y-1.5">
                    <Label htmlFor="prod-initial-stock">Tồn kho ban đầu</Label>
                    <Input
                      id="prod-initial-stock"
                      type="number"
                      min={0}
                      step={1}
                      placeholder="0"
                      className="font-mono tabular-nums"
                      {...register("initialStock")}
                    />
                  </div>
                )}

                <div className="space-y-1.5">
                  <Label htmlFor="prod-badge">Nhãn nổi bật</Label>
                  <Input id="prod-badge" placeholder="Mới, Bán chạy, Giảm sâu…" {...register("badge")} />
                </div>
              </div>

              <div className="flex flex-wrap gap-6 rounded-lg border border-border bg-background px-4 py-3">
                <label className="flex cursor-pointer items-center gap-2.5 text-sm font-medium">
                  <Switch
                    checked={watchedIsNew}
                    onCheckedChange={(checked) => setValue("isNew", checked)}
                  />
                  <span>Đánh dấu sản phẩm mới</span>
                </label>
                <label className="flex cursor-pointer items-center gap-2.5 text-sm font-medium">
                  <Switch
                    checked={watchedFeatured}
                    onCheckedChange={(checked) => setValue("featured", checked)}
                  />
                  <span>Ưu tiên trang chủ</span>
                </label>
              </div>

              <div className="space-y-2.5">
                <div className="flex items-center justify-between">
                  <Label htmlFor="prod-media-upload">Tải lên ảnh hoặc video</Label>
                  <span className="font-mono text-[11px] text-muted-foreground">
                    Ảnh ≤ 5MB (tối đa 10) · Video ≤ 30MB (tối đa 3)
                  </span>
                </div>

                <label
                  htmlFor="prod-media-upload"
                  className="flex cursor-pointer flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-input bg-background px-4 py-5 text-center transition-colors hover:border-primary hover:bg-accent/50"
                >
                  <UploadCloud className="size-6 text-primary" />
                  <div className="text-xs font-medium text-foreground">
                    Chọn tệp ảnh (JPEG, PNG, WebP, AVIF) hoặc video (MP4, WebM)
                  </div>
                  <div className="text-[11px] text-muted-foreground">
                    Hệ thống tự động nhận diện định dạng ảnh hoặc video khi tải tệp lên
                  </div>
                  <input
                    id="prod-media-upload"
                    type="file"
                    multiple
                    accept="image/jpeg,image/png,image/webp,image/avif,video/mp4,video/webm"
                    className="sr-only"
                    onChange={(e) => void handleFilesSelected(e)}
                  />
                </label>

                {uploadingCount > 0 && (
                  <div className="flex items-center gap-2 text-xs text-primary">
                    <Loader2 className="size-3.5 animate-spin" />
                    <span>Đang tải {uploadingCount} tệp lên kho lưu trữ…</span>
                  </div>
                )}

                {mediaList.length > 0 && (
                  <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                    {mediaList.map((entry, idx) => (
                      <div
                        key={`${entry.url}-${idx}`}
                        className="relative flex flex-col justify-between gap-2 rounded-lg border border-border bg-background p-2"
                      >
                        <button
                          type="button"
                          aria-label={`Xem chi tiết ${entry.kind === "VIDEO" ? "video" : "ảnh"} ${idx + 1}`}
                          onClick={() => setPreviewMediaIndex(idx)}
                          className="group relative flex h-24 w-full cursor-pointer items-center justify-center overflow-hidden rounded border border-border bg-card focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none"
                        >
                          {entry.kind === "IMAGE" ? (
                            <>
                              <img
                                src={entry.url}
                                alt={`Ảnh sản phẩm ${idx + 1}`}
                                className="max-h-full max-w-full p-1 object-contain transition-transform group-hover:scale-105"
                              />
                              <span className="pointer-events-none absolute right-1.5 bottom-1.5 inline-flex items-center gap-1 rounded bg-slate-950/75 px-1.5 py-0.5 text-[10px] font-medium text-white opacity-0 transition-opacity group-hover:opacity-100">
                                <Maximize2 className="size-2.5" />
                                Xem lớn
                              </span>
                            </>
                          ) : (
                            <div className="relative flex h-full w-full flex-col items-center justify-center gap-1 bg-sidebar text-white">
                              <video
                                src={entry.url}
                                muted
                                preload="metadata"
                                playsInline
                                className="absolute inset-0 h-full w-full object-cover opacity-55"
                              />
                              <span className="relative z-10 inline-flex size-8 items-center justify-center rounded-full bg-emerald-500/90 text-white shadow-sm transition-transform group-hover:scale-110">
                                <Play className="size-3.5 fill-current" />
                              </span>
                              <span className="relative z-10 inline-flex items-center gap-1 font-mono text-[10px] uppercase">
                                <Film className="size-3 text-emerald-300" />
                                Phát video
                              </span>
                            </div>
                          )}
                        </button>
                        <div className="flex items-center justify-between gap-1">
                          <Badge variant={entry.kind === "VIDEO" ? "default" : "secondary"}>
                            {entry.kind === "VIDEO"
                              ? "Video"
                              : idx === firstImageIdx
                                ? "Ảnh bìa"
                                : "Ảnh"}
                          </Badge>
                          <button
                            type="button"
                            aria-label={`Gỡ media ${idx + 1}`}
                            onClick={() => handleRemoveMedia(idx)}
                            className="inline-flex size-6 cursor-pointer items-center justify-center rounded-full border border-border bg-card text-red-700 hover:bg-red-50"
                          >
                            <X className="size-3.5" />
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="prod-short-desc">Mô tả ngắn</Label>
                <Input
                  id="prod-short-desc"
                  placeholder="Tóm tắt điểm nhấn sản phẩm trên thẻ danh sách…"
                  {...register("shortDescription")}
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="prod-desc">Mô tả chi tiết</Label>
                <Textarea
                  id="prod-desc"
                  rows={3}
                  placeholder="Thông tin chi tiết, chất liệu, công nghệ…"
                  {...register("description")}
                />
              </div>

              <div className="space-y-2.5">
                <Label htmlFor="prod-variant-label">Phân loại / Biến thể *</Label>
                <div className="flex flex-wrap gap-2">
                  <Input
                    id="prod-variant-label"
                    placeholder="Tên biến thể (VD: Đen Titan · 256GB)"
                    value={variantLabelInput}
                    onChange={(e) => setVariantLabelInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        e.preventDefault()
                        handleAddVariant()
                      }
                    }}
                    className="flex-1"
                  />
                  <Button type="button" variant="outline" onClick={handleAddVariant}>
                    <Plus className="size-4" />
                    Thêm
                  </Button>
                </div>

                <div className="space-y-2 rounded-lg border border-border bg-background px-3 py-2.5">
                  <div className="flex items-center justify-between gap-2 text-xs">
                    <span className="font-medium text-muted-foreground">Màu sắc biến thể</span>
                    {normalizedVariantColor ? (
                      <span className="inline-flex items-center gap-1.5 font-medium text-foreground">
                        <span
                          className="size-2.5 rounded-full border border-black/15"
                          style={{ backgroundColor: normalizedVariantColor }}
                        />
                        <span>{matchedColorPreset?.name ?? "Màu tùy chỉnh"}</span>
                        <span className="font-mono text-[11px] text-muted-foreground">
                          {normalizedVariantColor}
                        </span>
                      </span>
                    ) : (
                      <span className="text-muted-foreground">Không gắn màu</span>
                    )}
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    <button
                      type="button"
                      aria-label="Không gắn màu"
                      aria-pressed={!normalizedVariantColor}
                      title="Không gắn màu"
                      onClick={() => setVariantColorInput("")}
                      className={cn(
                        "relative inline-flex size-7 cursor-pointer items-center justify-center rounded-full border border-border bg-card text-[10px] text-muted-foreground transition-transform hover:scale-105",
                        !normalizedVariantColor &&
                          "ring-2 ring-primary ring-offset-2 ring-offset-background"
                      )}
                    >
                      <span className="h-px w-4 rotate-45 bg-muted-foreground/70" />
                    </button>

                    {VARIANT_COLOR_PRESETS.map((preset) => {
                      const active = normalizedVariantColor === preset.hex
                      return (
                        <button
                          key={preset.hex}
                          type="button"
                          aria-label={`Chọn màu ${preset.name}`}
                          aria-pressed={active}
                          title={`${preset.name} (${preset.hex})`}
                          onClick={() => setVariantColorInput(preset.hex)}
                          style={{ backgroundColor: preset.hex }}
                          className={cn(
                            "size-7 cursor-pointer rounded-full border border-black/15 transition-transform hover:scale-105",
                            active && "ring-2 ring-primary ring-offset-2 ring-offset-background"
                          )}
                        />
                      )
                    })}

                    <div
                      className={cn(
                        "relative inline-flex h-7 items-center gap-1.5 rounded-full border border-input bg-card px-2.5 text-xs font-medium text-foreground shadow-2xs transition-colors hover:bg-accent focus-within:ring-2 focus-within:ring-ring/40",
                        normalizedVariantColor &&
                          !matchedColorPreset &&
                          "border-primary ring-2 ring-primary ring-offset-2 ring-offset-background"
                      )}
                    >
                      {normalizedVariantColor && !matchedColorPreset ? (
                        <span
                          className="size-3 rounded-full border border-black/15"
                          style={{ backgroundColor: normalizedVariantColor }}
                        />
                      ) : (
                        <Palette className="size-3.5 text-primary" />
                      )}
                      <span>Khác…</span>
                      <input
                        type="color"
                        aria-label="Bảng màu tùy chỉnh"
                        value={
                          HEX_COLOR_PATTERN.test(normalizedVariantColor)
                            ? normalizedVariantColor
                            : "#0d9488"
                        }
                        onChange={(e) => setVariantColorInput(e.target.value.toLowerCase())}
                        className="absolute inset-0 size-full cursor-pointer opacity-0"
                      />
                    </div>
                  </div>
                </div>

                {variants.length > 0 && (
                  <div className="flex flex-wrap gap-1.5 pt-1">
                    {variants.map((variant, idx) => (
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
                        <button
                          type="button"
                          aria-label={`Xóa biến thể ${variant.label}`}
                          onClick={() =>
                            setVariants((prev) => prev.filter((_, i) => i !== idx))
                          }
                          className="cursor-pointer text-muted-foreground hover:text-foreground"
                        >
                          <X className="size-3" />
                        </button>
                      </span>
                    ))}
                  </div>
                )}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="prod-details">Thông số nổi bật (mỗi dòng 1 mục)</Label>
                <Textarea
                  id="prod-details"
                  rows={3}
                  placeholder={"Màn hình Liquid Retina XDR 120Hz\nPin thời lượng 22 giờ"}
                  {...register("detailsText")}
                />
              </div>
            </div>

            <SheetFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                Hủy
              </Button>
              <Button type="submit" disabled={isSubmitting || uploadingCount > 0}>
                {isSubmitting ? "Đang lưu…" : "Lưu sản phẩm"}
              </Button>
            </SheetFooter>
          </form>
        )}
      </SheetContent>
      </Sheet>

      <MediaLightboxDialog
        items={mediaList}
        activeIndex={previewMediaIndex}
        productTitle={watch("name") || (isEdit ? "Chi tiết sản phẩm" : "Sản phẩm mới")}
        onClose={() => setPreviewMediaIndex(null)}
        onSelectIndex={setPreviewMediaIndex}
      />
    </>
  )
}
