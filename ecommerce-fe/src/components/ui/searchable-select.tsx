import * as React from "react"
import { Popover } from "radix-ui"
import { Check, ChevronDown, Loader2, RotateCw } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"
import type { LocationOption } from "@/lib/types"

interface SearchableSelectProps {
  id?: string
  value: string
  placeholder: string
  options: LocationOption[]
  onValueChange: (code: string) => void
  disabled?: boolean
  loading?: boolean
  error?: string | null
  onRetry?: () => void
  searchPlaceholder?: string
  searchLabel?: string
  notFoundText?: string
  "aria-invalid"?: boolean
  "aria-describedby"?: string
}

function SearchableSelect({
  id,
  value,
  placeholder,
  options,
  onValueChange,
  disabled = false,
  loading = false,
  error = null,
  onRetry,
  searchPlaceholder = "Nhập tên để tìm kiếm",
  searchLabel = "Tìm kiếm",
  notFoundText = "Không tìm thấy kết quả",
  "aria-invalid": ariaInvalid,
  "aria-describedby": ariaDescribedBy,
}: SearchableSelectProps) {
  const [open, setOpen] = React.useState(false)
  const [query, setQuery] = React.useState("")
  const [activeIndex, setActiveIndex] = React.useState(0)

  const triggerRef = React.useRef<HTMLButtonElement>(null)
  const listboxId = React.useId()

  const selected = options.find((option) => option.code === value)

  React.useEffect(() => {
    if (open) setQuery("")
  }, [open])

  React.useEffect(() => {
    if (open) setActiveIndex(0)
  }, [open])

  const normalizedQuery = query.trim().toLocaleLowerCase()
  const filtered = normalizedQuery
    ? options.filter((option) => option.name.toLocaleLowerCase().includes(normalizedQuery))
    : options

  const safeActiveIndex = filtered.length === 0 ? 0 : Math.min(activeIndex, filtered.length - 1)
  const activeOption = filtered[safeActiveIndex]
  const activeId = activeOption ? `${listboxId}-option-${activeOption.code}` : undefined

  function handleTriggerKeyDown(event: React.KeyboardEvent<HTMLButtonElement>) {
    if (disabled) return
    if (event.key === "ArrowDown") {
      event.preventDefault()
      setOpen(true)
    }
  }

  function handleInputKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (filtered.length === 0) return
    switch (event.key) {
      case "ArrowDown": {
        event.preventDefault()
        setActiveIndex((index) => Math.min(index + 1, filtered.length - 1))
        break
      }
      case "ArrowUp": {
        event.preventDefault()
        setActiveIndex((index) => Math.max(index - 1, 0))
        break
      }
      case "Enter": {
        event.preventDefault()
        const option = filtered[safeActiveIndex]
        if (option) {
          onValueChange(option.code)
          setOpen(false)
          triggerRef.current?.focus()
        }
        break
      }
      case "Escape": {
        event.preventDefault()
        setOpen(false)
        triggerRef.current?.focus()
        break
      }
    }
  }

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button
          ref={triggerRef}
          id={id}
          type="button"
          disabled={disabled}
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-controls={listboxId}
          aria-invalid={ariaInvalid}
          aria-describedby={ariaDescribedBy}
          onKeyDown={handleTriggerKeyDown}
          className={cn(
            "flex h-8 w-full items-center justify-between gap-2 rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm whitespace-nowrap transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
            !selected && "text-muted-foreground"
          )}
        >
          <span className="truncate">{selected ? selected.name : placeholder}</span>
          <ChevronDown className="opacity-60" />
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={4}
          className="z-50 min-w-(--radix-popover-trigger-width) overflow-hidden rounded-lg border border-border bg-popover text-popover-foreground shadow-md data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95"
        >
          {loading ? (
            <div className="flex items-center gap-2 px-3 py-2.5 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              <span>Đang tải...</span>
            </div>
          ) : error ? (
            <div className="flex items-center justify-between gap-2 px-3 py-2.5">
              <p role="alert" className="text-sm text-destructive">
                {error}
              </p>
              {onRetry ? (
                <Button variant="outline" size="sm" type="button" onClick={onRetry}>
                  <RotateCw />
                  Thử lại
                </Button>
              ) : null}
            </div>
          ) : (
            <>
              <div className="border-b border-border p-1.5">
                <Input
                  autoFocus
                  role="combobox"
                  aria-expanded="true"
                  aria-controls={listboxId}
                  aria-activedescendant={activeId}
                  aria-autocomplete="list"
                  value={query}
                  onChange={(event) => {
                    setQuery(event.target.value)
                    setActiveIndex(0)
                  }}
                  onKeyDown={handleInputKeyDown}
                  placeholder={searchPlaceholder}
                  aria-label={searchLabel}
                  className="h-7"
                />
              </div>
              <div
                id={listboxId}
                role="listbox"
                aria-label={searchLabel}
                className="max-h-60 overflow-y-auto p-1"
              >
                {filtered.length > 0 ? (
                  filtered.map((option) => (
                    <button
                      key={option.code}
                      id={`${listboxId}-option-${option.code}`}
                      type="button"
                      role="option"
                      tabIndex={-1}
                      aria-selected={option.code === value}
                      data-selected={option.code === value || undefined}
                      onClick={() => {
                        onValueChange(option.code)
                        setOpen(false)
                        triggerRef.current?.focus()
                      }}
                      className={cn(
                        "relative flex w-full cursor-default items-center gap-2 rounded-sm px-2 py-1.5 pr-8 text-sm text-left outline-none focus:bg-accent focus:text-accent-foreground hover:bg-accent hover:text-accent-foreground",
                        option.code === value && "bg-accent text-accent-foreground"
                      )}
                    >
                      <span className="truncate">{option.name}</span>
                      {option.code === value ? (
                        <span className="absolute right-2 flex size-3.5 items-center justify-center">
                          <Check className="size-4" />
                        </span>
                      ) : null}
                    </button>
                  ))
                ) : (
                  <p className="px-2 py-3 text-sm text-muted-foreground">{notFoundText}</p>
                )}
              </div>
            </>
          )}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}

export { SearchableSelect }