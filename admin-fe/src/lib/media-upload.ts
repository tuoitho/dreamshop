import { presignProductMediaUpload } from "./api"
import type { MediaAssetType } from "./types"

export const ALLOWED_IMAGE_MIME_TYPES = [
  "image/jpeg",
  "image/png",
  "image/webp",
  "image/avif",
] as const

export const ALLOWED_VIDEO_MIME_TYPES = ["video/mp4", "video/webm"] as const

export const MAX_IMAGE_SIZE_BYTES = 5 * 1024 * 1024
export const MAX_VIDEO_SIZE_BYTES = 30 * 1024 * 1024

export interface UploadedProductMedia {
  uploadId: string
  mediaType: MediaAssetType
  publicUrl: string
  objectKey: string
}

export function validateProductMediaFile(file: File): string | null {
  const mime = file.type.trim().toLowerCase()
  const isImage = (ALLOWED_IMAGE_MIME_TYPES as readonly string[]).includes(mime)
  const isVideo = (ALLOWED_VIDEO_MIME_TYPES as readonly string[]).includes(mime)

  if (!isImage && !isVideo) {
    return "Định dạng không hỗ trợ. Chỉ chấp nhận ảnh JPEG, PNG, WebP, AVIF hoặc video MP4, WebM."
  }
  if (file.size <= 0) {
    return "Tệp tải lên đang rỗng."
  }
  if (isImage && file.size > MAX_IMAGE_SIZE_BYTES) {
    return "Ảnh vượt quá giới hạn 5 MB."
  }
  if (isVideo && file.size > MAX_VIDEO_SIZE_BYTES) {
    return "Video vượt quá giới hạn 30 MB."
  }
  return null
}

export async function uploadProductMediaFile(file: File): Promise<UploadedProductMedia> {
  const validationError = validateProductMediaFile(file)
  if (validationError) {
    throw new Error(validationError)
  }

  const contentType = file.type.trim().toLowerCase()
  const presigned = await presignProductMediaUpload({
    filename: file.name,
    contentType,
    sizeBytes: file.size,
  })

  // Direct browser PUT to MinIO S3 presigned URL: send ONLY Content-Type (never Authorization).
  const uploadResponse = await fetch(presigned.uploadUrl, {
    method: "PUT",
    headers: {
      "Content-Type": contentType,
    },
    body: file,
  })

  if (!uploadResponse.ok) {
    throw new Error(`Tải tệp lên kho lưu trữ thất bại (HTTP ${uploadResponse.status}).`)
  }

  return {
    uploadId: presigned.uploadId,
    mediaType: presigned.mediaType,
    publicUrl: presigned.publicUrl,
    objectKey: presigned.objectKey,
  }
}
