import type { ComponentProps } from "react"
import { cn } from "@/lib/utils"

export function Card({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="card"
      className={cn(
        "rounded-2xl border border-border/90 bg-card text-card-foreground shadow-[0_1px_2px_rgba(16,24,40,0.04),0_4px_12px_-6px_rgba(16,24,40,0.05)] transition-shadow duration-200",
        className
      )}
      {...props}
    />
  )
}

export function CardHeader({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="card-header"
      className={cn("flex flex-col gap-1 p-5 sm:p-6", className)}
      {...props}
    />
  )
}

export function CardTitle({ className, ...props }: ComponentProps<"h3">) {
  return (
    <h3
      data-slot="card-title"
      className={cn("text-[15px] font-semibold tracking-tight text-foreground", className)}
      {...props}
    />
  )
}

export function CardDescription({ className, ...props }: ComponentProps<"p">) {
  return (
    <p
      data-slot="card-description"
      className={cn("text-xs leading-relaxed text-muted-foreground", className)}
      {...props}
    />
  )
}

export function CardContent({ className, ...props }: ComponentProps<"div">) {
  return (
    <div data-slot="card-content" className={cn("px-5 pb-5 sm:px-6 sm:pb-6", className)} {...props} />
  )
}
