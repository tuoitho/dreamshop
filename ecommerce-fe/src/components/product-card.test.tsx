import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import type { ProductSummary } from "@/lib/types"
import { ProductCard } from "./product-card"

const BASE_PRODUCT: ProductSummary = {
  id: "dt-1",
  name: "Điện thoại Nova X15",
  brand: "Nova",
  categoryId: "dien-thoai",
  categoryName: "Điện thoại",
  price: 18990000,
  originalPrice: 21990000,
  rating: 4.900000095367432,
  reviewCount: 1200,
  soldCount: 2300,
  emoji: "📱",
  imageHue: [220, 260],
}

function renderCard(overrides: Partial<ProductSummary> = {}) {
  return render(
    <MemoryRouter>
      <ProductCard product={{ ...BASE_PRODUCT, ...overrides }} />
    </MemoryRouter>
  )
}

describe("ProductCard", () => {
  it("formats float32 rating precision to one decimal place", () => {
    renderCard()

    expect(screen.getByText("4.9")).toBeInTheDocument()
    expect(screen.queryByText("4.900000095367432")).not.toBeInTheDocument()
  })

  it("displays 'Tạm hết hàng' badge while keeping product link navigable when OUT_OF_STOCK", () => {
    renderCard({ status: "OUT_OF_STOCK", availableQuantity: 0 })

    expect(screen.getByText("Tạm hết hàng")).toBeInTheDocument()
    const links = screen.getAllByRole("link", { name: BASE_PRODUCT.name })
    expect(links.length).toBeGreaterThan(0)
    expect(links[0]).toHaveAttribute("href", "/san-pham/dt-1")
  })

  it("displays 'Ngừng kinh doanh' badge when UNAVAILABLE", () => {
    renderCard({ status: "UNAVAILABLE", availableQuantity: 0 })

    expect(screen.getByText("Ngừng kinh doanh")).toBeInTheDocument()
  })

  it("displays low-stock note when status is LIMITED", () => {
    renderCard({ status: "LIMITED", availableQuantity: 2 })

    expect(screen.getByText("Chỉ còn 2")).toBeInTheDocument()
  })
})
