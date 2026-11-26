import type { ComponentProps } from "react"
import { cn } from "@/lib/utils"

export function Table({ className, ...props }: ComponentProps<"table">) {
  return (
    <div className="relative w-full overflow-x-auto">
      <table
        data-slot="table"
        className={cn("w-full caption-bottom text-sm", className)}
        {...props}
      />
    </div>
  )
}

export function TableHeader({ className, ...props }: ComponentProps<"thead">) {
  return (
    <thead
      data-slot="table-header"
      className={cn("border-y border-border/80 bg-muted/45 [&_tr]:border-b", className)}
      {...props}
    />
  )
}

export function TableBody({ className, ...props }: ComponentProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={cn("[&_tr:last-child]:border-0", className)}
      {...props}
    />
  )
}

export function TableRow({ className, ...props }: ComponentProps<"tr">) {
  return (
    <tr
      data-slot="table-row"
      className={cn(
        "border-b border-border/65 transition-colors hover:bg-muted/35",
        className
      )}
      {...props}
    />
  )
}

export function TableHead({ className, ...props }: ComponentProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        "h-10 px-4 text-left align-middle text-[11px] font-semibold tracking-wider text-muted-foreground uppercase whitespace-nowrap",
        className
      )}
      {...props}
    />
  )
}

export function TableCell({ className, ...props }: ComponentProps<"td">) {
  return (
    <td
      data-slot="table-cell"
      className={cn("px-4 py-3.5 align-middle text-sm", className)}
      {...props}
    />
  )
}
