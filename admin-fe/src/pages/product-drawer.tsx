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
  Trash2,
  UploadCloud,
  X,
} from "lucide-react"
import { useEffect, useRef, useState, type ChangeEvent } from "react"
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
  setSkuStock,
  toClientError,
  updateAdminProduct,
} from "@/lib/api"
import { CATEGORY_PRESETS } from "@/lib/categories"
import { formatCurrencyVnd, formatNumber } from "@/lib/format"
import { ALLOWED_VIDEO_MIME_TYPES, uploadProductMediaFile } from "@/lib/media-upload"
import type {
  AdminCategoryMetric,
  AdminProductMutationPayload,
  AdminProductVariantPayload,
  AdminStockSnapshot,
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
  shortDescription: z.string(),
  description: z.string(),
  badge: z.string(),
  emoji: z.string(),
  isNew: z.boolean(),
  featured: z.boolean(),
  detailsText: z.string(),
})

type ProductFormValues = z.infer<typeof productFormSchema>

interface VariantFormRow {
  key: string
  skuId?: string
  label: string
  colorHex: string
  priceInput: string
  originalPriceInput: string
  initialStockInput: string
  stock?: AdminStockSnapshot | null
}

interface PendingSkuStockWrite {
  skuId: string
  label: string
  quantity: number
}

interface PendingStockRetry {
  isEdit: boolean
  productId: string
  productName: string
  items: PendingSkuStockWrite[]
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
  shortDescription: "",
  description: "",
  badge: "",
  emoji: CATEGORY_PRESETS[0].emoji,
  isNew: false,
  featured: false,
  detailsText: "",
}

function makeDefaultVariantRow(key: string, label = "Tiêu chuẩn"): VariantFormRow {
  return {
    key,
    label,
    colorHex: "",
    priceInput: "0",
    originalPriceInput: "",
    initialStockInput: "",
  }
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
  const rowCounterRef = useRef(1)
  const [loadingDetail, setLoadingDetail] = useState(false)
  const [uploadingCount, setUploadingCount] = useState(0)
  const [mediaList, setMediaList] = useState<MediaPreviewItem[]>([])
  const [previewMediaIndex, setPreviewMediaIndex] = useState<number | null>(null)
  const [variants, setVariants] = useState<VariantFormRow[]>(() => [
    makeDefaultVariantRow("var-0"),
  ])
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
    rowCounterRef.current = 1

    if (!productId) {
      reset(DEFAULT_VALUES)
      setMediaList([])
      setVariants([makeDefaultVariantRow("var-0")])
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
          shortDescription: detail.shortDescription ?? "",
          description: detail.description ?? "",
          badge: detail.badge ?? "",
          emoji: detail.emoji || "📦",
          isNew: detail.isNew,
          featured: detail.featured,
          detailsText: (detail.details ?? []).join("\n"),
        })
        const mappedVariants: VariantFormRow[] =
          detail.variants && detail.variants.length > 0
            ? detail.variants.map((v, idx) => ({
                key: v.skuId ?? `existing-${idx}`,
                skuId: v.skuId,
                label: v.label,
                colorHex: v.colorHex?.toLowerCase() ?? "",
                priceInput: String(v.price),
                originalPriceInput:
                  v.originalPrice && v.originalPrice > v.price ? String(v.originalPrice) : "",
                initialStockInput: "",
                stock: v.stock ?? null,
              }))
            : [makeDefaultVariantRow("var-0")]
        rowCounterRef.current = mappedVariants.length + 1
        setVariants(mappedVariants)
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

  const handleAddVariantRow = () => {
    if (variants.length >= MAX_VARIANTS_COUNT) {
      setServerError(`Tối đa ${MAX_VARIANTS_COUNT} biến thể cho mỗi sản phẩm.`)
      return
    }
    const nextKey = `var-${rowCounterRef.current++}`
    setServerError(null)
    setVariants((prev) => [...prev, makeDefaultVariantRow(nextKey, "")])
  }

  const updateVariantRow = (rowKey: string, patch: Partial<VariantFormRow>) => {
    setVariants((prev) =>
      prev.map((row) => (row.key === rowKey ? { ...row, ...patch } : row))
    )
  }

  const handleRemoveVariantRow = (rowKey: string) => {
    setVariants((prev) => {
      if (prev.length <= 1) return prev
      const target = prev.find((row) => row.key === rowKey)
      if ((target?.stock?.reserved ?? 0) > 0) return prev
      return prev.filter((row) => row.key !== rowKey)
    })
  }

  const handleRetryStock = async () => {
    if (!pendingStockRetry) return
    setRetryingStock(true)
    setServerError(null)

    const remaining: PendingSkuStockWrite[] = []
    let lastError = ""
    for (const entry of pendingStockRetry.items) {
      try {
        await setSkuStock(entry.skuId, pendingStockRetry.productId, entry.quantity)
      } catch (error) {
        remaining.push(entry)
        lastError = toClientError(error).message
      }
    }

    setRetryingStock(false)
    if (remaining.length === 0) {
      setPendingStockRetry(null)
      onSaved()
      onOpenChange(false)
      return
    }

    setPendingStockRetry({
      ...pendingStockRetry,
      items: remaining,
      errorMessage: lastError,
    })
  }

  const onSubmit = async (values: ProductFormValues) => {
    setServerError(null)

    if (variants.length === 0 || variants.length > MAX_VARIANTS_COUNT) {
      setServerError(`Sản phẩm cần có từ 1 đến ${MAX_VARIANTS_COUNT} biến thể.`)
      return
    }

    const seenLabels = new Set<string>()
    const variantPayloads: AdminProductVariantPayload[] = []
    const newVariantStocks: Array<{ normalizedLabel: string; label: string; quantity: number }> = []

    for (let idx = 0; idx < variants.length; idx++) {
      const row = variants[idx]
      if (!row) continue

      const trimmedLabel = row.label.trim()
      if (!trimmedLabel) {
        setServerError(`Vui lòng nhập tên phân loại cho biến thể #${idx + 1}.`)
        return
      }
      const normalizedLabel = trimmedLabel.toLowerCase()
      if (seenLabels.has(normalizedLabel)) {
        setServerError(`Tên phân loại “${trimmedLabel}” đang bị trùng lặp.`)
        return
      }
      seenLabels.add(normalizedLabel)

      const normalizedColor = row.colorHex.trim().toLowerCase()
      if (normalizedColor && !HEX_COLOR_PATTERN.test(normalizedColor)) {
        setServerError("Mã màu biến thể phải theo định dạng hex 6 ký tự (ví dụ: #1c1c1e).")
        return
      }

      const parsedPrice = Number(row.priceInput.trim())
      if (!Number.isFinite(parsedPrice) || parsedPrice <= 0) {
        setServerError(`Giá bán của biến thể “${trimmedLabel}” phải lớn hơn 0.`)
        return
      }

      const rawOriginal = row.originalPriceInput.trim()
      let parsedOriginal: number | undefined
      if (rawOriginal !== "") {
        const candidate = Number(rawOriginal)
        if (!Number.isFinite(candidate) || candidate < parsedPrice) {
          setServerError(
            `Giá niêm yết gốc của biến thể “${trimmedLabel}” phải để trống hoặc lớn hơn / bằng giá bán.`
          )
          return
        }
        parsedOriginal = Math.round(candidate)
      }

      if (!row.skuId && canUpdateInventory && row.initialStockInput.trim() !== "") {
        const parsedStock = Number.parseInt(row.initialStockInput.trim(), 10)
        if (Number.isNaN(parsedStock) || parsedStock < 0) {
          setServerError(`Tồn kho ban đầu của biến thể “${trimmedLabel}” phải là số nguyên không âm.`)
          return
        }
        if (parsedStock > 0) {
          newVariantStocks.push({
            normalizedLabel,
            label: trimmedLabel,
            quantity: parsedStock,
          })
        }
      }

      variantPayloads.push({
        ...(row.skuId ? { skuId: row.skuId } : {}),
        label: trimmedLabel,
        ...(normalizedColor ? { colorHex: normalizedColor } : {}),
        price: Math.round(parsedPrice),
        ...(parsedOriginal !== undefined ? { originalPrice: parsedOriginal } : {}),
      })
    }

    const payload: AdminProductMutationPayload = {
      name: values.name.trim(),
      brand: values.brand.trim(),
      categoryId: values.categoryId.trim(),
      categoryName: values.categoryName.trim(),
      shortDescription: values.shortDescription.trim(),
      description: values.description.trim(),
      images: mediaList.filter((m) => m.kind === "IMAGE").map((m) => m.url),
      videos: mediaList.filter((m) => m.kind === "VIDEO").map((m) => m.url),
      isNew: values.isNew,
      badge: values.badge.trim(),
      featured: values.featured,
      emoji: values.emoji.trim() || "📦",
      imageHue,
      variants: variantPayloads,
      details: values.detailsText
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean),
    }

    try {
      const saved =
        isEdit && productId
          ? await updateAdminProduct(productId, payload)
          : await createAdminProduct(payload)
      onSaved()

      if (newVariantStocks.length > 0) {
        const failedItems: PendingSkuStockWrite[] = []
        let lastErrorMessage = ""

        for (const pending of newVariantStocks) {
          const matched = (saved.variants ?? []).find(
            (v) => v.label.trim().toLowerCase() === pending.normalizedLabel && Boolean(v.skuId)
          )
          if (!matched?.skuId) {
            continue
          }
          try {
            await setSkuStock(matched.skuId, saved.id, pending.quantity)
          } catch (stockError) {
            failedItems.push({
              skuId: matched.skuId,
              label: pending.label,
              quantity: pending.quantity,
            })
            lastErrorMessage = toClientError(stockError).message
          }
        }

        if (failedItems.length > 0) {
          setPendingStockRetry({
            isEdit,
            productId: saved.id,
            productName: saved.name,
            items: failedItems,
            errorMessage: lastErrorMessage,
          })
          return
        }

        onSaved()
      }

      onOpenChange(false)
    } catch (error) {
      setServerError(toClientError(error).message)
    }
  }

  const firstImageIdx = mediaList.findIndex((m) => m.kind === "IMAGE")
  const validPrices = variants
    .map((row) => Number(row.priceInput.trim()))
    .filter((val) => Number.isFinite(val) && val > 0)
  const minPreviewPrice = validPrices.length > 0 ? Math.min(...validPrices) : 0
  const maxPreviewPrice = validPrices.length > 0 ? Math.max(...validPrices) : 0

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent className="sm:max-w-[680px]">
          <SheetHeader>
            <SheetTitle>{isEdit ? "Chỉnh sửa sản phẩm" : "Thêm sản phẩm mới"}</SheetTitle>
            <SheetDescription>
              {isEdit
                ? `Cập nhật thông tin thương mại, bảng giá SKU và tài nguyên media cho mã ${productId}.`
                : "Khởi tạo sản phẩm mới cùng bảng phân loại SKU, giá bán riêng và tài nguyên S3/MinIO."}
            </SheetDescription>
          </SheetHeader>

          {pendingStockRetry ? (
            <div className="flex flex-1 flex-col justify-between p-6">
              <div className="space-y-4">
                <div className="flex items-start gap-3 rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-900">
                  <CheckCircle2 className="mt-0.5 size-5 shrink-0 text-emerald-600" />
                  <div>
                    <div className="font-semibold">
                      {pendingStockRetry.isEdit ? "Đã cập nhật" : "Đã tạo"} sản phẩm “
                      {pendingStockRetry.productName}”
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
                  <div className="space-y-2">
                    <div className="font-semibold">
                      Chưa thể ghi nhận tồn kho ban đầu cho {pendingStockRetry.items.length} SKU
                    </div>
                    <ul className="space-y-1 font-mono text-xs text-amber-900">
                      {pendingStockRetry.items.map((item) => (
                        <li key={item.skuId}>
                          {item.skuId} ({item.label}): {formatNumber(item.quantity)} đơn vị
                        </li>
                      ))}
                    </ul>
                    <p className="text-xs leading-relaxed text-amber-900">
                      {pendingStockRetry.errorMessage}
                    </p>
                    <p className="text-xs text-amber-800">
                      Danh mục sản phẩm đã được lưu. Nhấn thử lại để chỉ cập nhật các SKU còn thiếu
                      mà không tạo trùng sản phẩm.
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

                  <div className="space-y-1.5 sm:col-span-2">
                    <Label htmlFor="prod-badge">Nhãn nổi bật</Label>
                    <Input
                      id="prod-badge"
                      placeholder="Mới, Bán chạy, Giảm sâu…"
                      {...register("badge")}
                    />
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

                <div className="space-y-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <div className="text-sm font-medium text-foreground">
                        Phân loại, Giá bán & Tồn kho SKU *
                      </div>
                      <p className="text-xs text-muted-foreground">
                        Mỗi phân loại là một SKU độc lập có giá bán và số lượng tồn kho riêng.
                      </p>
                    </div>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={handleAddVariantRow}
                      disabled={variants.length >= MAX_VARIANTS_COUNT}
                    >
                      <Plus className="size-4" />
                      Thêm biến thể
                    </Button>
                  </div>

                  <div className="flex items-center justify-between rounded-lg border border-border bg-muted/40 px-3.5 py-2 text-xs">
                    <span className="font-medium text-muted-foreground">
                      Giá hiển thị đại diện ({variants.length} phân loại):
                    </span>
                    <span className="font-mono font-semibold text-foreground tabular-nums">
                      {minPreviewPrice <= 0
                        ? "Chưa nhập giá hợp lệ (> 0 ₫)"
                        : maxPreviewPrice > minPreviewPrice
                          ? `Từ ${formatCurrencyVnd(minPreviewPrice)} đến ${formatCurrencyVnd(maxPreviewPrice)}`
                          : formatCurrencyVnd(minPreviewPrice)}
                    </span>
                  </div>

                  <div className="space-y-3">
                    {variants.map((row, idx) => {
                      const normalizedRowColor = row.colorHex.trim().toLowerCase()
                      const matchedPreset = VARIANT_COLOR_PRESETS.find(
                        (preset) => preset.hex === normalizedRowColor
                      )
                      const hasReserved = (row.stock?.reserved ?? 0) > 0
                      const deleteDisabled = hasReserved || variants.length <= 1

                      return (
                        <div
                          key={row.key}
                          data-testid={`variant-row-${idx}`}
                          className="space-y-3 rounded-xl border border-border bg-background p-3.5"
                        >
                          <div className="flex flex-wrap items-center justify-between gap-2">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="text-xs font-semibold text-foreground">
                                Biến thể #{idx + 1}
                              </span>
                              {row.skuId ? (
                                <Badge variant="outline" className="font-mono text-[11px]">
                                  {row.skuId}
                                </Badge>
                              ) : (
                                <Badge variant="secondary" className="text-[11px]">
                                  SKU mới
                                </Badge>
                              )}
                              {row.stock && (
                                <span className="font-mono text-[11px] text-muted-foreground tabular-nums">
                                  Khả dụng: {formatNumber(row.stock.availableQuantity)} · Giữ chỗ:{" "}
                                  {formatNumber(row.stock.reserved)}
                                </span>
                              )}
                              {hasReserved && (
                                <Badge variant="warning" className="text-[10px]">
                                  Đang có đơn giữ chỗ
                                </Badge>
                              )}
                            </div>

                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              aria-label={`Xóa biến thể ${row.label || `#${idx + 1}`}`}
                              disabled={deleteDisabled}
                              onClick={() => handleRemoveVariantRow(row.key)}
                              className="h-7 px-2 text-xs text-red-700 hover:bg-red-50 disabled:opacity-50"
                            >
                              <Trash2 className="size-3.5" />
                              Xóa
                            </Button>
                          </div>

                          <div className="grid gap-3 sm:grid-cols-2">
                            <div className="space-y-1 sm:col-span-2">
                              <Label htmlFor={`variant-label-${idx}`}>Tên phân loại *</Label>
                              <Input
                                id={`variant-label-${idx}`}
                                placeholder="Ví dụ: Đen Titan · 256GB"
                                value={row.label}
                                onChange={(e) =>
                                  updateVariantRow(row.key, { label: e.target.value })
                                }
                              />
                            </div>

                            <div className="space-y-1">
                              <Label htmlFor={`variant-price-${idx}`}>Giá bán (VND) *</Label>
                              <Input
                                id={`variant-price-${idx}`}
                                type="number"
                                min={0}
                                step={1000}
                                className="font-mono tabular-nums"
                                value={row.priceInput}
                                onChange={(e) =>
                                  updateVariantRow(row.key, { priceInput: e.target.value })
                                }
                              />
                            </div>

                            <div className="space-y-1">
                              <Label htmlFor={`variant-original-price-${idx}`}>
                                Giá niêm yết gốc (VND)
                              </Label>
                              <Input
                                id={`variant-original-price-${idx}`}
                                type="number"
                                min={0}
                                step={1000}
                                placeholder="Để trống nếu bằng giá bán"
                                className="font-mono tabular-nums"
                                value={row.originalPriceInput}
                                onChange={(e) =>
                                  updateVariantRow(row.key, {
                                    originalPriceInput: e.target.value,
                                  })
                                }
                              />
                            </div>

                            {!row.skuId && canUpdateInventory && (
                              <div className="space-y-1 sm:col-span-2">
                                <Label htmlFor={`variant-initial-stock-${idx}`}>
                                  Tồn kho ban đầu
                                </Label>
                                <Input
                                  id={`variant-initial-stock-${idx}`}
                                  type="number"
                                  min={0}
                                  step={1}
                                  placeholder="0"
                                  className="font-mono tabular-nums"
                                  value={row.initialStockInput}
                                  onChange={(e) =>
                                    updateVariantRow(row.key, {
                                      initialStockInput: e.target.value,
                                    })
                                  }
                                />
                              </div>
                            )}
                          </div>

                          <div className="space-y-1.5 border-t border-border pt-2.5">
                            <div className="flex items-center justify-between gap-2 text-xs">
                              <span className="font-medium text-muted-foreground">
                                Màu sắc biến thể
                              </span>
                              {normalizedRowColor ? (
                                <span className="inline-flex items-center gap-1.5 font-medium text-foreground">
                                  <span
                                    className="size-2.5 rounded-full border border-black/15"
                                    style={{ backgroundColor: normalizedRowColor }}
                                  />
                                  <span>{matchedPreset?.name ?? "Màu tùy chỉnh"}</span>
                                  <span className="font-mono text-[11px] text-muted-foreground">
                                    {normalizedRowColor}
                                  </span>
                                </span>
                              ) : (
                                <span className="text-muted-foreground">Không gắn màu</span>
                              )}
                            </div>

                            <div className="flex flex-wrap items-center gap-1.5">
                              <button
                                type="button"
                                aria-label="Không gắn màu"
                                aria-pressed={!normalizedRowColor}
                                title="Không gắn màu"
                                onClick={() => updateVariantRow(row.key, { colorHex: "" })}
                                className={cn(
                                  "relative inline-flex size-6 cursor-pointer items-center justify-center rounded-full border border-border bg-card text-[10px] text-muted-foreground transition-transform hover:scale-105",
                                  !normalizedRowColor &&
                                    "ring-2 ring-primary ring-offset-2 ring-offset-background"
                                )}
                              >
                                <span className="h-px w-3.5 rotate-45 bg-muted-foreground/70" />
                              </button>

                              {VARIANT_COLOR_PRESETS.map((preset) => {
                                const active = normalizedRowColor === preset.hex
                                return (
                                  <button
                                    key={preset.hex}
                                    type="button"
                                    aria-label={`Chọn màu ${preset.name}`}
                                    aria-pressed={active}
                                    title={`${preset.name} (${preset.hex})`}
                                    onClick={() =>
                                      updateVariantRow(row.key, { colorHex: preset.hex })
                                    }
                                    style={{ backgroundColor: preset.hex }}
                                    className={cn(
                                      "size-6 cursor-pointer rounded-full border border-black/15 transition-transform hover:scale-105",
                                      active &&
                                        "ring-2 ring-primary ring-offset-2 ring-offset-background"
                                    )}
                                  />
                                )
                              })}

                              <div
                                className={cn(
                                  "relative inline-flex h-6 items-center gap-1 rounded-full border border-input bg-card px-2 text-[11px] font-medium text-foreground shadow-2xs transition-colors hover:bg-accent focus-within:ring-2 focus-within:ring-ring/40",
                                  normalizedRowColor &&
                                    !matchedPreset &&
                                    "border-primary ring-2 ring-primary ring-offset-2 ring-offset-background"
                                )}
                              >
                                {normalizedRowColor && !matchedPreset ? (
                                  <span
                                    className="size-2.5 rounded-full border border-black/15"
                                    style={{ backgroundColor: normalizedRowColor }}
                                  />
                                ) : (
                                  <Palette className="size-3 text-primary" />
                                )}
                                <span>Khác…</span>
                                <input
                                  type="color"
                                  aria-label="Bảng màu tùy chỉnh"
                                  value={
                                    HEX_COLOR_PATTERN.test(normalizedRowColor)
                                      ? normalizedRowColor
                                      : "#0d9488"
                                  }
                                  onChange={(e) =>
                                    updateVariantRow(row.key, {
                                      colorHex: e.target.value.toLowerCase(),
                                    })
                                  }
                                  className="absolute inset-0 size-full cursor-pointer opacity-0"
                                />
                              </div>
                            </div>
                          </div>
                        </div>
                      )
                    })}
                  </div>
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
                                  className="max-h-full max-w-full object-contain p-1 transition-transform group-hover:scale-105"
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
