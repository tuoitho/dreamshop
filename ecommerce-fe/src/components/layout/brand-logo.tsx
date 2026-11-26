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
      className={cn("size-8 shrink-0", className)}
    >
      <defs>
        <linearGradient id={gradId} x1="2" y1="2" x2="30" y2="30" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#5ea82e" />
          <stop offset="100%" stopColor="#2e741c" />
        </linearGradient>
      </defs>
      <rect width="32" height="32" rx="8.5" fill={`url(#${gradId})`} />
      <path
        d="M15.2 8.5H9.5C8.95 8.5 8.5 8.95 8.5 9.5V22.5C8.5 23.05 8.95 23.5 9.5 23.5H14.5A7.5 7.5 0 0 0 21.9 14.8"
        stroke="#ffffff"
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M21.6 5.9L22.45 8.05L24.6 8.9L22.45 9.75L21.6 11.9L20.75 9.75L18.6 8.9L20.75 8.05L21.6 5.9Z"
        fill="#fef08a"
      />
    </svg>
  )
}
