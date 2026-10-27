import { afterEach, describe, expect, it, vi } from "vitest"
import * as api from "./api"
import { uploadProductMediaFile, validateProductMediaFile } from "./media-upload"

describe("media-upload", () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it("rejects unsupported SVG MIME types and oversized files before presigning", async () => {
    const presignSpy = vi.spyOn(api, "presignProductMediaUpload")

    const svgFile = new File(["<svg></svg>"], "banner.svg", { type: "image/svg+xml" })
    expect(validateProductMediaFile(svgFile)).not.toBeNull()
    await expect(uploadProductMediaFile(svgFile)).rejects.toThrow(/Định dạng/)

    const hugeImage = new File(["x"], "photo.png", { type: "image/png" })
    Object.defineProperty(hugeImage, "size", { value: 6 * 1024 * 1024 })
    expect(validateProductMediaFile(hugeImage)).not.toBeNull()
    await expect(uploadProductMediaFile(hugeImage)).rejects.toThrow(/5 MB/)

    const hugeVideo = new File(["x"], "demo.mp4", { type: "video/mp4" })
    Object.defineProperty(hugeVideo, "size", { value: 31 * 1024 * 1024 })
    expect(validateProductMediaFile(hugeVideo)).not.toBeNull()
    await expect(uploadProductMediaFile(hugeVideo)).rejects.toThrow(/30 MB/)

    expect(presignSpy).not.toHaveBeenCalled()
  })

  it("uploads directly to MinIO presigned PUT URL with Content-Type only and zero Authorization header", async () => {
    const presignSpy = vi.spyOn(api, "presignProductMediaUpload").mockResolvedValue({
      uploadId: "up-1",
      mediaType: "VIDEO",
      uploadUrl: "http://localhost:9000/product-media/products/2026/10/up-1-demo.mp4?X-Amz-Signature=sig",
      publicUrl: "http://localhost:9000/product-media/products/2026/10/up-1-demo.mp4",
      objectKey: "products/2026/10/up-1-demo.mp4",
      expiresInSeconds: 300,
    })

    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
    })
    vi.stubGlobal("fetch", fetchMock)

    const videoFile = new File(["mp4-binary"], "demo.mp4", { type: "VIDEO/MP4" })
    const uploaded = await uploadProductMediaFile(videoFile)

    expect(presignSpy).toHaveBeenCalledWith({
      filename: "demo.mp4",
      contentType: "video/mp4",
      sizeBytes: videoFile.size,
    })
    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [calledUrl, calledInit] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(calledUrl).toContain("X-Amz-Signature=sig")
    expect(calledInit.method).toBe("PUT")
    expect(calledInit.headers).toEqual({ "Content-Type": "video/mp4" })
    expect((calledInit.headers as Record<string, string>).Authorization).toBeUndefined()
    expect(calledInit.body).toBe(videoFile)
    expect(uploaded).toEqual({
      uploadId: "up-1",
      mediaType: "VIDEO",
      publicUrl: "http://localhost:9000/product-media/products/2026/10/up-1-demo.mp4",
      objectKey: "products/2026/10/up-1-demo.mp4",
    })
  })
})
