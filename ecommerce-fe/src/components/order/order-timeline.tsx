import { actorLabel, orderStatusLabel } from "@/lib/order-status"
import type { OrderStatusChange } from "@/lib/types"

const timeFormatter = new Intl.DateTimeFormat("vi-VN", { dateStyle: "short", timeStyle: "short" })

function formatTime(iso: string): string {
  const parsed = new Date(iso)
  return Number.isNaN(parsed.getTime()) ? "" : timeFormatter.format(parsed)
}

export function OrderTimeline({ history }: { history: OrderStatusChange[] }) {
  if (history.length === 0) {
    return null
  }
  return (
    <section aria-labelledby="order-timeline-heading">
      <h2 id="order-timeline-heading" className="mb-3 text-sm font-bold">
        Lịch sử đơn hàng
      </h2>
      <ol aria-labelledby="order-timeline-heading" className="space-y-3 border-l-2 border-border pl-4">
        {history.map((change, index) => (
          <li key={`${change.createdAt}-${index}`} className="relative text-sm">
            <span className="absolute top-1.5 -left-[21px] size-2.5 rounded-full bg-primary" aria-hidden="true" />
            <p className="font-semibold">{orderStatusLabel(change.toStatus)}</p>
            <p className="text-xs text-muted-foreground">
              {actorLabel(change.actorType)}
              {change.createdAt ? ` • ${formatTime(change.createdAt)}` : ""}
            </p>
            {(change.shippingCarrier || change.trackingNumber) && (
              <p className="text-xs text-muted-foreground">
                {[change.shippingCarrier, change.trackingNumber].filter(Boolean).join(" • ")}
              </p>
            )}
            {change.reason && <p className="text-xs text-muted-foreground">{change.reason}</p>}
          </li>
        ))}
      </ol>
    </section>
  )
}
