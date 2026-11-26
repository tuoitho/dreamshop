import { cva, type VariantProps } from "class-variance-authority"
import type { ComponentProps } from "react"
import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex w-fit shrink-0 items-center gap-1 rounded-full border px-2.5 py-0.5 text-[11px] font-medium whitespace-nowrap transition-colors",
  {
    variants: {
      variant: {
        default: "border-teal-600/15 bg-teal-50 text-teal-700",
        secondary: "border-border bg-secondary text-secondary-foreground",
        outline: "border-border bg-card text-foreground",
        success: "border-emerald-600/20 bg-emerald-50 text-emerald-700",
        warning: "border-amber-500/25 bg-amber-50 text-amber-800",
        destructive: "border-red-500/20 bg-red-50 text-red-700",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

export interface BadgeProps
  extends ComponentProps<"span">,
    VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return (
    <span
      data-slot="badge"
      className={cn(badgeVariants({ variant, className }))}
      {...props}
    />
  )
}
