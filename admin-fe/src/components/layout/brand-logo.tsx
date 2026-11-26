import { useId } from "react"
import { cn } from "@/lib/utils"

interface BrandLogoProps {
  className?: string
}

export function BrandLogo({ className }: BrandLogoProps) {
  const gradId = useId()

  return (
    <svg
      viewBox="0 0 32 32"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
      className={cn("size-9 shrink-0", className)}
    >
      <defs>
        <linearGradient id={gradId} x1="2" y1="2" x2="30" y2="30" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#27272a" />
          <stop offset="100%" stopColor="#09090b" />
        </linearGradient>
      </defs>
      <rect
        x="0.75"
        y="0.75"
        width="30.5"
        height="30.5"
        rx="9"
        fill={`url(#${gradId})`}
        stroke="#27272a"
        strokeWidth="1.25"
      />
      <path
        d="M15 8.75H9.75C9.2 8.75 8.75 9.2 8.75 9.75V22.25C8.75 22.8 9.2 23.25 9.75 23.25H14.6A7.25 7.25 0 0 0 21.75 14.85"
        stroke="#ffffff"
        strokeWidth="2.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M21.6 6.2L22.4 8.2L24.4 9L22.4 9.8L21.6 11.8L20.8 9.8L18.8 9L20.8 8.2L21.6 6.2Z"
        fill="#2dd4bf"
      />
    </svg>
  )
}
