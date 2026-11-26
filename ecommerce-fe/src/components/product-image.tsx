import { useId } from "react"
import { cn } from "@/lib/utils"

function hashString(input: string): number {
  let hash = 0
  for (let i = 0; i < input.length; i++) {
    hash = (hash << 5) - hash + input.charCodeAt(i)
    hash |= 0
  }
  return Math.abs(hash)
}

function hsl(hue: number, sat: number, light: number, alpha = 1): string {
  return `hsl(${hue} ${sat}% ${light}% / ${alpha})`
}

interface ProductImageProps {
  emoji: string
  hue: [number, number] | readonly number[]
  seed: string
  label: string
  className?: string
}

const DEFAULT_HUE_START = 200
const DEFAULT_HUE_END = 240

export function ProductImage({ emoji, hue, seed, label, className }: ProductImageProps) {
  const id = useId()
  const rnd = hashString(seed)

  const startHue = Number.isFinite(hue[0]) ? hue[0] : DEFAULT_HUE_START
  const endHue = Number.isFinite(hue[1]) ? hue[1] : DEFAULT_HUE_END
  const hueA = startHue + (rnd % 20)
  const hueB = endHue - (rnd % 16)

  const blob1 = { cx: 30 + (rnd % 40), cy: 20 + (rnd % 30), r: 45 + (rnd % 25) }
  const blob2 = { cx: 60 + (rnd % 30), cy: 70 + (rnd % 20), r: 40 + (rnd % 30) }
  const blob3 = { cx: 45 + (rnd % 20), cy: 50 + (rnd % 25), r: 30 + (rnd % 20) }

  const size = 300

  return (
    <svg
      viewBox={`0 0 ${size} ${size}`}
      role="img"
      aria-label={label}
      className={cn("block h-full w-full", className)}
    >
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor={hsl(hueA, 60, 88)} />
          <stop offset="100%" stopColor={hsl(hueB, 55, 78)} />
        </linearGradient>
      </defs>
      <rect width={size} height={size} fill={`url(#${id})`} />
      <circle
        cx={blob1.cx * (size / 100)}
        cy={blob1.cy * (size / 100)}
        r={blob1.r * (size / 100)}
        fill={hsl(hueB, 55, 82, 0.45)}
      />
      <circle
        cx={blob2.cx * (size / 100)}
        cy={blob2.cy * (size / 100)}
        r={blob2.r * (size / 100)}
        fill={hsl(hueA, 50, 92, 0.5)}
      />
      <circle
        cx={blob3.cx * (size / 100)}
        cy={blob3.cy * (size / 100)}
        r={blob3.r * (size / 100)}
        fill="rgba(255,255,255,0.18)"
      />
      <circle
        cx={70 * (size / 100)}
        cy={25 * (size / 100)}
        r={30 * (size / 100)}
        fill="rgba(255,255,255,0.25)"
      />
      <text
        x="50%"
        y="52%"
        dominantBaseline="central"
        textAnchor="middle"
        fontSize={120}
      >
        {emoji}
      </text>
    </svg>
  )
}
