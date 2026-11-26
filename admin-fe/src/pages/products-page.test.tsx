import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, describe, expect, it, vi } from "vitest"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import * as api from "@/lib/api"
import * as mediaUpload from "@/lib/media-upload"
import type { AdminProductDetail, AdminProductsPage } from "@/lib/types"
import { ProductsPage } from "./products-page"

function makeAuth(permissions: string[]): AuthContextValue {
  const permSet = new Set(permissions)
  return {
    status: "authenticated",
    user: {
      userId: "u-1",
      email: "admin@sshop.vn",
      fullName: "Admin DreamShop",
      roles: ["ADMIN"],
    },
    permissions: permSet,
    hasAnyAdminAccess: true,
    can: (perm: string) => permSet.has(perm),
    login: vi.fn(),
    logout: vi.fn(),
    refreshPermissions: vi.fn(),
  }
}

const samplePage: AdminProductsPage = {
  items: [
    {
      id: "p-macbook",
      name: "MacBook Pro 14 M4",
      brand: "Apple",
      categoryId: "laptop",
      categoryName: "Laptop",
      price: 39990000,
      originalPrice: 42990000,
      maxPrice: 49990000,
      images: ["http://localhost:9000/product-media/images/macbook.webp"],
      rating: 4.9,
      reviewCount: 34,
      soldCount: 120,
      isNew: true,
      badge: "Mới",
      featured: true,
      emoji: "💻",
      imageHue: [210, 20],
      deleted: false,
      variantCount: 2,
      videoCount: 1,
      createdAt: "2026-10-20T00:00:00Z",
      updatedAt: "2026-10-25T00:00:00Z",
      stock: {
        quantity: 20,
        reserved: 2,
        availableQuantity: 18,
      },
    },
  ],
  page: 1,
  pageSize: 12,
  totalItems: 1,
  totalPages: 1,
  activeCount: 1,
  deletedCount: 2,
  categories: [{ categoryId: "laptop", categoryName: "Laptop", activeProductCount: 1 }],
  stockDegraded: false,
}

const sampleDetail: AdminProductDetail = {
  id: "p-macbook",
  name: "MacBook Pro 14 M4",
  shortDescription: "Chip Apple M4 hiệu năng cao",
  description: "Máy tính xách tay chuyên nghiệp màn hình Liquid Retina XDR.",
  brand: "Apple",
  categoryId: "laptop",
  categoryName: "Laptop",
  price: 39990000,
  originalPrice: 42990000,
  maxPrice: 49990000,
  images: [
    "http://localhost:9000/product-media/images/macbook-1.webp",
    "http://localhost:9000/product-media/images/macbook-2.webp",
  ],
  videos: ["http://localhost:9000/product-media/videos/macbook-tour.mp4"],
  rating: 4.9,
  reviewCount: 34,
  soldCount: 120,
  isNew: true,
  badge: "Mới",
  featured: true,
  emoji: "💻",
  imageHue: [210, 20],
  variants: [
    {
      skuId: "p-macbook-v1",
      label: "Space Black · 16GB/512GB",
      colorHex: "#1c1c1e",
      price: 39990000,
      originalPrice: 42990000,
      stock: {
        quantity: 12,
        reserved: 2,
        availableQuantity: 10,
      },
    },
    {
      skuId: "p-macbook-v2",
      label: "Silver · 24GB/1TB",
      colorHex: "#cbd5e1",
      price: 49990000,
      originalPrice: 49990000,
      stock: {
        quantity: 8,
        reserved: 0,
        availableQuantity: 8,
      },
    },
  ],
  details: ["Màn hình 120Hz ProMotion"],
  deleted: false,
  stock: {
    quantity: 20,
    reserved: 2,
    availableQuantity: 18,
  },
}

describe("ProductsPage & ProductDrawer", () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it("renders TanStack Table rows with range price prefix and filters by status tab and search query", async () => {
    const listSpy = vi.spyOn(api, "listAdminProducts").mockResolvedValue(samplePage)

    render(
      <MemoryRouter>
        <AuthContext.Provider
          value={makeAuth([
            "product.view",
            "product.create",
            "product.update",
            "product.delete",
            "inventory.update",
          ])}
        >
          <ProductsPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    expect(await screen.findByText("MacBook Pro 14 M4")).toBeInTheDocument()
    expect(screen.getByText(/Từ\s+39\.990\.000/i)).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: /đã ẩn/i }))
    await waitFor(() =>
      expect(listSpy).toHaveBeenCalledWith(
        expect.objectContaining({ status: "deleted", page: 1 }),
        expect.anything()
      )
    )

    const searchInput = screen.getByPlaceholderText(/tìm theo tên/i)
    await userEvent.type(searchInput, "MacBook")
    await userEvent.click(screen.getByRole("button", { name: /^lọc$/i }))
    await waitFor(() =>
      expect(listSpy).toHaveBeenCalledWith(
        expect.objectContaining({ q: "MacBook" }),
        expect.anything()
      )
    )

    const sortSelect = screen.getByLabelText(/sắp xếp sản phẩm/i)
    await userEvent.selectOptions(sortSelect, "gia-cao")
    await waitFor(() =>
      expect(listSpy).toHaveBeenCalledWith(
        expect.objectContaining({ sort: "gia-cao", page: 1 }),
        expect.anything()
      )
    )
  })

  it("uploads image & video via MinIO presigned PUT, omits root price, and writes SKU initial stock", async () => {
    vi.spyOn(api, "listAdminProducts").mockResolvedValue(samplePage)
    const uploadSpy = vi
      .spyOn(mediaUpload, "uploadProductMediaFile")
      .mockResolvedValueOnce({
        uploadId: "up-img-1",
        mediaType: "IMAGE",
        publicUrl: "http://localhost:9000/product-media/images/new-1.webp",
        objectKey: "images/new-1.webp",
      })
      .mockResolvedValueOnce({
        uploadId: "up-img-2",
        mediaType: "IMAGE",
        publicUrl: "http://localhost:9000/product-media/images/new-2.webp",
        objectKey: "images/new-2.webp",
      })
      .mockResolvedValueOnce({
        uploadId: "up-vid-1",
        mediaType: "VIDEO",
        publicUrl: "http://localhost:9000/product-media/videos/demo.mp4",
        objectKey: "videos/demo.mp4",
      })

    const createSpy = vi.spyOn(api, "createAdminProduct").mockResolvedValue({
      ...sampleDetail,
      id: "p-created-1",
      name: "Bàn phím cơ Keychron Q1",
      variants: [
        {
          skuId: "p-created-1-v1",
          label: "Tiêu chuẩn",
          price: 4290000,
          originalPrice: 4290000,
          stock: null,
        },
      ],
    })
    const setStockSpy = vi.spyOn(api, "setSkuStock").mockResolvedValue({
      skuId: "p-created-1-v1",
      productId: "p-created-1",
      quantity: 25,
      reserved: 0,
      availableQuantity: 25,
    })

    render(
      <MemoryRouter>
        <AuthContext.Provider
          value={makeAuth([
            "product.view",
            "product.create",
            "product.update",
            "product.delete",
            "inventory.update",
          ])}
        >
          <ProductsPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    await screen.findByText("MacBook Pro 14 M4")
    await userEvent.click(screen.getByRole("button", { name: /thêm sản phẩm/i }))

    await userEvent.type(await screen.findByLabelText(/tên sản phẩm/i), "Bàn phím cơ Keychron Q1")
    await userEvent.type(screen.getByLabelText(/thương hiệu/i), "Keychron")

    const row0 = screen.getByTestId("variant-row-0")
    const priceInput = within(row0).getByLabelText(/^giá bán/i)
    await userEvent.clear(priceInput)
    await userEvent.type(priceInput, "4290000")

    const initialStockInput = within(row0).getByLabelText(/tồn kho ban đầu/i)
    await userEvent.clear(initialStockInput)
    await userEvent.type(initialStockInput, "25")

    const fileInput = screen.getByLabelText(/tải lên ảnh hoặc video/i)
    const img1 = new File(["img1"], "new-1.webp", { type: "image/webp" })
    const img2 = new File(["img2"], "new-2.webp", { type: "image/webp" })
    const vid1 = new File(["vid1"], "demo.mp4", { type: "video/mp4" })
    await userEvent.upload(fileInput, [img1, img2, vid1])

    await waitFor(() => expect(uploadSpy).toHaveBeenCalledTimes(3))

    await userEvent.click(screen.getByRole("button", { name: /xem chi tiết ảnh 1/i }))
    expect(await screen.findByText("1 / 3")).toBeInTheDocument()
    await userEvent.click(screen.getByRole("button", { name: /media tiếp theo/i }))
    await userEvent.click(screen.getByRole("button", { name: /media tiếp theo/i }))
    expect(await screen.findByText("3 / 3")).toBeInTheDocument()
    await userEvent.click(screen.getByRole("button", { name: /đóng xem trước media/i }))

    const removeButtons = screen.getAllByRole("button", { name: /gỡ media/i })
    expect(removeButtons.length).toBe(3)
    await userEvent.click(removeButtons[1]!)

    await userEvent.click(screen.getByRole("button", { name: /lưu sản phẩm/i }))

    await waitFor(() => expect(createSpy).toHaveBeenCalledTimes(1))
    const submittedPayload = createSpy.mock.calls[0]![0]
    expect(submittedPayload).not.toHaveProperty("price")
    expect(submittedPayload).not.toHaveProperty("originalPrice")
    expect(submittedPayload).toMatchObject({
      name: "Bàn phím cơ Keychron Q1",
      brand: "Keychron",
      images: ["http://localhost:9000/product-media/images/new-1.webp"],
      videos: ["http://localhost:9000/product-media/videos/demo.mp4"],
      variants: [{ label: "Tiêu chuẩn", price: 4290000 }],
    })
    await waitFor(() =>
      expect(setStockSpy).toHaveBeenCalledWith("p-created-1-v1", "p-created-1", 25)
    )
  })

  it("rejects zero variant price and originalPrice below price before calling createAdminProduct", async () => {
    vi.spyOn(api, "listAdminProducts").mockResolvedValue(samplePage)
    const createSpy = vi.spyOn(api, "createAdminProduct")

    render(
      <MemoryRouter>
        <AuthContext.Provider value={makeAuth(["product.view", "product.create"])}>
          <ProductsPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    await screen.findByText("MacBook Pro 14 M4")
    await userEvent.click(screen.getByRole("button", { name: /thêm sản phẩm/i }))

    await userEvent.type(await screen.findByLabelText(/tên sản phẩm/i), "Sản phẩm lỗi giá")
    await userEvent.type(screen.getByLabelText(/thương hiệu/i), "DreamShop")

    await userEvent.click(screen.getByRole("button", { name: /lưu sản phẩm/i }))
    expect(await screen.findByRole("alert")).toHaveTextContent(/lớn hơn 0/i)
    expect(createSpy).not.toHaveBeenCalled()

    const row0 = screen.getByTestId("variant-row-0")
    const priceInput = within(row0).getByLabelText(/^giá bán/i)
    await userEvent.clear(priceInput)
    await userEvent.type(priceInput, "500000")
    await userEvent.type(within(row0).getByLabelText(/giá niêm yết gốc/i), "300000")
    await userEvent.click(screen.getByRole("button", { name: /lưu sản phẩm/i }))
    expect(await screen.findByRole("alert")).toHaveTextContent(/giá niêm yết gốc/i)
    expect(createSpy).not.toHaveBeenCalled()
  })

  it("creates multi-variant product with per-variant pricing and representative range summary without color pickers", async () => {
    vi.spyOn(api, "listAdminProducts").mockResolvedValue(samplePage)
    const createSpy = vi.spyOn(api, "createAdminProduct").mockResolvedValue({
      ...sampleDetail,
      id: "p-var-1",
      name: "iPhone 16 Pro",
      variants: [
        { skuId: "p-var-1-v1", label: "256GB · Tiêu chuẩn", price: 28990000 },
        { skuId: "p-var-1-v2", label: "512GB · Cao cấp", price: 34990000 },
        { skuId: "p-var-1-v3", label: "Bản Quốc Tế", price: 29990000 },
      ],
    })

    render(
      <MemoryRouter>
        <AuthContext.Provider value={makeAuth(["product.view", "product.create"])}>
          <ProductsPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    await screen.findByText("MacBook Pro 14 M4")
    await userEvent.click(screen.getByRole("button", { name: /thêm sản phẩm/i }))

    await userEvent.type(await screen.findByLabelText(/tên sản phẩm/i), "iPhone 16 Pro")
    await userEvent.type(screen.getByLabelText(/thương hiệu/i), "Apple")

    // Configure variant row 0 (generic variant only, no color picker UI)
    const row0 = screen.getByTestId("variant-row-0")
    expect(within(row0).queryByText(/màu sắc biến thể/i)).not.toBeInTheDocument()
    const label0 = within(row0).getByLabelText(/tên phân loại/i)
    await userEvent.clear(label0)
    await userEvent.type(label0, "256GB · Tiêu chuẩn")
    const price0 = within(row0).getByLabelText(/^giá bán/i)
    await userEvent.clear(price0)
    await userEvent.type(price0, "28990000")
    await userEvent.type(within(row0).getByLabelText(/giá niêm yết gốc/i), "30990000")

    // Add variant row 1
    await userEvent.click(screen.getByRole("button", { name: /thêm biến thể/i }))
    const row1 = screen.getByTestId("variant-row-1")
    await userEvent.type(within(row1).getByLabelText(/tên phân loại/i), "512GB · Cao cấp")
    const price1 = within(row1).getByLabelText(/^giá bán/i)
    await userEvent.clear(price1)
    await userEvent.type(price1, "34990000")

    // Add variant row 2
    await userEvent.click(screen.getByRole("button", { name: /thêm biến thể/i }))
    const row2 = screen.getByTestId("variant-row-2")
    await userEvent.type(within(row2).getByLabelText(/tên phân loại/i), "Bản Quốc Tế")
    const price2 = within(row2).getByLabelText(/^giá bán/i)
    await userEvent.clear(price2)
    await userEvent.type(price2, "29990000")

    expect(
      screen.getByText(/Từ\s+28\.990\.000.*đến\s+34\.990\.000/i)
    ).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: /lưu sản phẩm/i }))

    await waitFor(() => expect(createSpy).toHaveBeenCalledTimes(1))
    expect(createSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        name: "iPhone 16 Pro",
        variants: [
          {
            label: "256GB · Tiêu chuẩn",
            price: 28990000,
            originalPrice: 30990000,
          },
          { label: "512GB · Cao cấp", price: 34990000 },
          { label: "Bản Quốc Tế", price: 29990000 },
        ],
      })
    )
  })

  it("retries only failed SKU stock writes when multi-SKU initial stock partially fails", async () => {
    vi.spyOn(api, "listAdminProducts").mockResolvedValue(samplePage)
    const createSpy = vi.spyOn(api, "createAdminProduct").mockResolvedValue({
      ...sampleDetail,
      id: "p-partial-99",
      name: "Chuột Logitech MX Master 3S",
      variants: [
        { skuId: "p-partial-99-v1", label: "Đen", price: 2490000 },
        { skuId: "p-partial-99-v2", label: "Trắng", price: 2590000 },
      ],
    })
    const setStockSpy = vi
      .spyOn(api, "setSkuStock")
      .mockResolvedValueOnce({
        skuId: "p-partial-99-v1",
        productId: "p-partial-99",
        quantity: 10,
        reserved: 0,
        availableQuantity: 10,
      })
      .mockRejectedValueOnce(new api.ApiClientError(502, "Dịch vụ kho tạm thời gián đoạn"))
      .mockResolvedValueOnce({
        skuId: "p-partial-99-v2",
        productId: "p-partial-99",
        quantity: 15,
        reserved: 0,
        availableQuantity: 15,
      })

    render(
      <MemoryRouter>
        <AuthContext.Provider
          value={makeAuth(["product.view", "product.create", "inventory.update"])}
        >
          <ProductsPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    await screen.findByText("MacBook Pro 14 M4")
    await userEvent.click(screen.getByRole("button", { name: /thêm sản phẩm/i }))

    await userEvent.type(await screen.findByLabelText(/tên sản phẩm/i), "Chuột Logitech MX Master 3S")
    await userEvent.type(screen.getByLabelText(/thương hiệu/i), "Logitech")

    const row0 = screen.getByTestId("variant-row-0")
    await userEvent.clear(within(row0).getByLabelText(/tên phân loại/i))
    await userEvent.type(within(row0).getByLabelText(/tên phân loại/i), "Đen")
    await userEvent.clear(within(row0).getByLabelText(/^giá bán/i))
    await userEvent.type(within(row0).getByLabelText(/^giá bán/i), "2490000")
    await userEvent.type(within(row0).getByLabelText(/tồn kho ban đầu/i), "10")

    await userEvent.click(screen.getByRole("button", { name: /thêm biến thể/i }))
    const row1 = screen.getByTestId("variant-row-1")
    await userEvent.type(within(row1).getByLabelText(/tên phân loại/i), "Trắng")
    await userEvent.clear(within(row1).getByLabelText(/^giá bán/i))
    await userEvent.type(within(row1).getByLabelText(/^giá bán/i), "2590000")
    await userEvent.type(within(row1).getByLabelText(/tồn kho ban đầu/i), "15")

    await userEvent.click(screen.getByRole("button", { name: /lưu sản phẩm/i }))

    await waitFor(() => expect(createSpy).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(setStockSpy).toHaveBeenCalledTimes(2))

    const retryStockBtn = await screen.findByRole("button", { name: /thử lại cập nhật tồn kho/i })
    await userEvent.click(retryStockBtn)

    await waitFor(() => expect(setStockSpy).toHaveBeenCalledTimes(3))
    expect(setStockSpy).toHaveBeenLastCalledWith("p-partial-99-v2", "p-partial-99", 15)
    expect(createSpy).toHaveBeenCalledTimes(1)
  })

  it("preserves existing skuId on edit, disables deleting reserved variants, and toggles soft-delete", async () => {
    vi.spyOn(api, "listAdminProducts").mockResolvedValue(samplePage)
    vi.spyOn(api, "getAdminProduct").mockResolvedValue(sampleDetail)
    const updateSpy = vi.spyOn(api, "updateAdminProduct").mockResolvedValue(sampleDetail)
    const deleteSpy = vi
      .spyOn(api, "toggleAdminProductDeleted")
      .mockResolvedValue({ id: "p-macbook", deleted: true })

    render(
      <MemoryRouter>
        <AuthContext.Provider
          value={makeAuth(["product.view", "product.update", "product.delete"])}
        >
          <ProductsPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    await screen.findByText("MacBook Pro 14 M4")
    await userEvent.click(screen.getByRole("button", { name: /chỉnh sửa macbook pro 14 m4/i }))

    const nameInput = await screen.findByLabelText(/tên sản phẩm/i)
    await waitFor(() => expect(nameInput).toHaveValue("MacBook Pro 14 M4"))

    // Reserved variant (reserved = 2) shows skuId and disables deletion
    const row0 = screen.getByTestId("variant-row-0")
    expect(within(row0).getByText("p-macbook-v1")).toBeInTheDocument()
    expect(within(row0).getByText(/đang có đơn giữ chỗ/i)).toBeInTheDocument()
    expect(within(row0).getByRole("button", { name: /xóa biến thể/i })).toBeDisabled()

    // Unreserved variant (reserved = 0) can be deleted or edited while preserving skuId
    const row1 = screen.getByTestId("variant-row-1")
    expect(within(row1).getByText("p-macbook-v2")).toBeInTheDocument()
    const deleteRow1Btn = within(row1).getByRole("button", { name: /xóa biến thể/i })
    expect(deleteRow1Btn).not.toBeDisabled()

    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, "MacBook Pro 14 M4 Pro")
    await userEvent.click(screen.getByRole("button", { name: /lưu sản phẩm/i }))

    await waitFor(() => expect(updateSpy).toHaveBeenCalledTimes(1))
    expect(updateSpy).toHaveBeenCalledWith(
      "p-macbook",
      expect.objectContaining({
        name: "MacBook Pro 14 M4 Pro",
        variants: [
          {
            skuId: "p-macbook-v1",
            label: "Space Black · 16GB/512GB",
            price: 39990000,
            originalPrice: 42990000,
          },
          {
            skuId: "p-macbook-v2",
            label: "Silver · 24GB/1TB",
            price: 49990000,
          },
        ],
      })
    )

    await userEvent.click(screen.getByRole("button", { name: /ẩn macbook pro 14 m4/i }))
    await waitFor(() => expect(deleteSpy).toHaveBeenCalledWith("p-macbook", true))
  })

  it("opens admin product detail sheet with SKU pricing & stock table and interactive media lightbox", async () => {
    vi.spyOn(api, "listAdminProducts").mockResolvedValue(samplePage)
    vi.spyOn(api, "getAdminProduct").mockResolvedValue(sampleDetail)

    render(
      <MemoryRouter>
        <AuthContext.Provider value={makeAuth(["product.view"])}>
          <ProductsPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    await screen.findByText("MacBook Pro 14 M4")
    await userEvent.click(screen.getByRole("button", { name: /xem chi tiết macbook pro 14 m4/i }))

    expect(await screen.findByText("Chip Apple M4 hiệu năng cao")).toBeInTheDocument()
    const detailDialog = screen.getByRole("dialog")
    expect(within(detailDialog).getByText("p-macbook-v1")).toBeInTheDocument()
    expect(within(detailDialog).getByText("p-macbook-v2")).toBeInTheDocument()
    expect(within(detailDialog).getByText("Space Black · 16GB/512GB")).toBeInTheDocument()
    expect(within(detailDialog).getByText("Silver · 24GB/1TB")).toBeInTheDocument()
    expect(within(detailDialog).getByText(/49\.990\.000/i)).toBeInTheDocument()
    expect(within(detailDialog).getByText("Màn hình 120Hz ProMotion")).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: /chọn media 3 \(video\)/i }))
    const inlineVideo = await screen.findByTestId("detail-video-player")
    expect(inlineVideo).toHaveAttribute(
      "src",
      "http://localhost:9000/product-media/videos/macbook-tour.mp4"
    )

    await userEvent.click(screen.getByRole("button", { name: /xem toàn màn hình/i }))
    expect(await screen.findByText("3 / 3")).toBeInTheDocument()
    expect(screen.getByTestId("lightbox-video-player")).toHaveAttribute(
      "src",
      "http://localhost:9000/product-media/videos/macbook-tour.mp4"
    )

    await userEvent.click(screen.getByRole("button", { name: /media trước/i }))
    expect(await screen.findByText("2 / 3")).toBeInTheDocument()
    expect(screen.getByTestId("lightbox-image-viewer")).toHaveAttribute(
      "src",
      "http://localhost:9000/product-media/images/macbook-2.webp"
    )
  })
})
