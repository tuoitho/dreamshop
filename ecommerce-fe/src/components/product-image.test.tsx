import { render } from "@testing-library/react"
import { describe, expect, it } from "vitest"
import { ProductImage } from "./product-image"

describe("ProductImage", () => {
  it("falls back to valid HSL hues when imageHue is empty or incomplete", () => {
    for (const hue of [[], [180], [Number.NaN, 220]]) {
      const { container, unmount } = render(
        <ProductImage emoji="📱" hue={hue} seed="dt-1" label="Nova X" />
      )

      const markup = container.innerHTML
      expect(markup).not.toContain("NaN")
      expect(markup).toMatch(/hsl\(\d+ 60% 88% \/ 1\)/)
      expect(markup).toMatch(/hsl\(\d+ 55% 78% \/ 1\)/)
      unmount()
    }
  })
})
