import { AlertCircle, ArrowUpDown, PackageCheck } from "lucide-react"
import { useEffect, useState, type FormEvent } from "react"
import { Button } from "@/components/ui/button"
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { adjustSkuStock, setSkuStock } from "@/lib/api"
import { resolveErrorMessage } from "@/lib/error-messages"
import { formatNumber } from "@/lib/format"

export interface StockDialogTarget {
  skuId: string
  productId: string
  name: string
  variantLabel?: string
  brand?: string
  quantity: number
  reserved: number
  availableQuantity: number
}

interface StockDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  item: StockDialogTarget | null
  onSaved: () => void
}

const QUICK_DELTAS = [-10, -5, -1, 1, 5, 10, 50]

export function StockDialog({ open, onOpenChange, item, onSaved }: StockDialogProps) {
  const [mode, setMode] = useState<"set" | "adjust">("set")
  const [setQtyInput, setSetQtyInput] = useState("0")
  const [deltaInput, setDeltaInput] = useState("0")
  const [submitting, setSubmitting] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  useEffect(() => {
    if (open && item) {
      setMode("set")
      setSetQtyInput(String(item.quantity))
      setDeltaInput("0")
      setErrorMessage(null)
    }
  }, [open, item])

  if (!item) return null

  const parsedSetQty = Number.parseInt(setQtyInput, 10)
  const validSetQty = !Number.isNaN(parsedSetQty) && parsedSetQty >= 0
  const projectedSetAvailable = validSetQty ? parsedSetQty - item.reserved : null

  const parsedDelta = Number.parseInt(deltaInput, 10)
  const validDelta = !Number.isNaN(parsedDelta)
  const projectedAdjustQty = validDelta ? item.quantity + parsedDelta : null
  const projectedAdjustAvailable = validDelta ? item.availableQuantity + parsedDelta : null

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault()
    setErrorMessage(null)

    if (mode === "set") {
      if (!validSetQty) {
        setErrorMessage("Tổng tồn kho mới phải là số nguyên không âm.")
        return
      }
      setSubmitting(true)
      try {
        await setSkuStock(item.skuId, item.productId, parsedSetQty)
        onSaved()
        onOpenChange(false)
      } catch (error) {
        setErrorMessage(resolveErrorMessage(error))
      } finally {
        setSubmitting(false)
      }
      return
    }

    if (!validDelta || parsedDelta === 0) {
      setErrorMessage("Vui lòng nhập biến động số lượng khác 0 (ví dụ: 10 hoặc -5).")
      return
    }

    setSubmitting(true)
    try {
      await adjustSkuStock(item.skuId, item.productId, parsedDelta)
      onSaved()
      onOpenChange(false)
    } catch (error) {
      setErrorMessage(resolveErrorMessage(error))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>Điều phối tồn kho</DialogTitle>
          <DialogDescription>
            {item.name}
            {item.variantLabel ? ` · ${item.variantLabel}` : ""} · SKU{" "}
            <span className="font-mono">{item.skuId}</span>
          </DialogDescription>
        </DialogHeader>

        <div className="grid grid-cols-3 gap-2.5">
          <div className="rounded-lg border border-border bg-background p-3">
            <div className="text-[11px] font-medium text-muted-foreground">Tổng tồn hiện tại</div>
            <div className="mt-1 font-mono text-lg font-semibold text-foreground tabular-nums">
              {formatNumber(item.quantity)}
            </div>
          </div>
          <div className="rounded-lg border border-border bg-background p-3">
            <div className="text-[11px] font-medium text-muted-foreground">Đang giữ chỗ</div>
            <div className="mt-1 font-mono text-lg font-semibold text-amber-800 tabular-nums">
              {formatNumber(item.reserved)}
            </div>
          </div>
          <div className="rounded-lg border border-border bg-background p-3">
            <div className="text-[11px] font-medium text-muted-foreground">Khả dụng bán</div>
            <div className="mt-1 font-mono text-lg font-semibold text-emerald-700 tabular-nums">
              {formatNumber(item.availableQuantity)}
            </div>
          </div>
        </div>

        {errorMessage && (
          <div
            role="alert"
            className="flex items-start gap-2.5 rounded-lg border border-red-200 bg-red-50 px-3.5 py-2.5 text-xs font-medium text-red-900"
          >
            <AlertCircle className="mt-0.5 size-4 shrink-0 text-red-600" />
            <span>{errorMessage}</span>
          </div>
        )}

        <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
          <Tabs
            value={mode}
            onValueChange={(next) => {
              setMode(next as "set" | "adjust")
              setErrorMessage(null)
            }}
          >
            <TabsList className="w-full">
              <TabsTrigger value="set" className="flex-1">
                <PackageCheck className="mr-1.5 size-3.5" />
                Đặt mức tồn
              </TabsTrigger>
              <TabsTrigger value="adjust" className="flex-1">
                <ArrowUpDown className="mr-1.5 size-3.5" />
                Điều chỉnh +/-
              </TabsTrigger>
            </TabsList>

            <TabsContent value="set" className="space-y-3 pt-2">
              <div className="space-y-1.5">
                <Label htmlFor="stock-set-quantity">Tổng tồn kho mới</Label>
                <Input
                  id="stock-set-quantity"
                  type="number"
                  min={0}
                  step={1}
                  className="font-mono tabular-nums"
                  value={setQtyInput}
                  onChange={(e) => setSetQtyInput(e.target.value)}
                />
                <p className="text-xs text-muted-foreground">
                  Lượng hàng đang giữ chỗ ({formatNumber(item.reserved)}) được bảo toàn nguyên tử.
                </p>
              </div>

              {projectedSetAvailable !== null && (
                <div
                  className={
                    projectedSetAvailable < 0
                      ? "flex items-center justify-between rounded-lg border border-amber-300 bg-amber-50 px-3.5 py-2.5 text-xs text-amber-900"
                      : "flex items-center justify-between rounded-lg border border-emerald-200 bg-emerald-50 px-3.5 py-2.5 text-xs text-emerald-900"
                  }
                >
                  <span>Khả dụng dự kiến sau cập nhật:</span>
                  <span className="font-mono font-semibold tabular-nums">
                    {formatNumber(projectedSetAvailable)} đơn vị
                  </span>
                </div>
              )}
            </TabsContent>

            <TabsContent value="adjust" className="space-y-3 pt-2">
              <div className="space-y-1.5">
                <Label htmlFor="stock-adjust-delta">Biến động số lượng (+/-)</Label>
                <Input
                  id="stock-adjust-delta"
                  type="number"
                  step={1}
                  className="font-mono tabular-nums"
                  value={deltaInput}
                  onChange={(e) => setDeltaInput(e.target.value)}
                />
              </div>

              <div className="flex flex-wrap gap-1.5">
                {QUICK_DELTAS.map((step) => (
                  <Button
                    key={step}
                    type="button"
                    variant="outline"
                    size="sm"
                    className="font-mono text-xs tabular-nums"
                    onClick={() => {
                      const base = Number.parseInt(deltaInput, 10)
                      const next = (Number.isNaN(base) ? 0 : base) + step
                      setDeltaInput(String(next))
                    }}
                  >
                    {step > 0 ? `+${step}` : step}
                  </Button>
                ))}
              </div>

              {projectedAdjustQty !== null && projectedAdjustAvailable !== null && (
                <div
                  className={
                    projectedAdjustAvailable < 0
                      ? "flex items-center justify-between rounded-lg border border-amber-300 bg-amber-50 px-3.5 py-2.5 text-xs text-amber-900"
                      : "flex items-center justify-between rounded-lg border border-emerald-200 bg-emerald-50 px-3.5 py-2.5 text-xs text-emerald-900"
                  }
                >
                  <span>Dự kiến tổng tồn / khả dụng:</span>
                  <span className="font-mono font-semibold tabular-nums">
                    {formatNumber(projectedAdjustQty)} tổng · {formatNumber(projectedAdjustAvailable)}{" "}
                    khả dụng
                  </span>
                </div>
              )}
            </TabsContent>
          </Tabs>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Hủy
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? "Đang lưu…" : "Xác nhận cập nhật"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
