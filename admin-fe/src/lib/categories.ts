export interface CategoryPreset {
  id: string
  name: string
  emoji: string
}

export const CATEGORY_PRESETS: readonly CategoryPreset[] = [
  { id: "dien-thoai", name: "Điện thoại", emoji: "📱" },
  { id: "laptop", name: "Laptop", emoji: "💻" },
  { id: "tai-nghe", name: "Tai nghe", emoji: "🎧" },
  { id: "dong-ho", name: "Đồng hồ", emoji: "⌚" },
  { id: "thoi-trang", name: "Thời trang", emoji: "👕" },
  { id: "gia-dung", name: "Gia dụng", emoji: "🏠" },
  { id: "lam-dep", name: "Làm đẹp", emoji: "💄" },
  { id: "sach", name: "Sách", emoji: "📚" },
]
