export interface ProductVariant {
  label: string
  colorHex?: string
}

export interface Product {
  id: string
  name: string
  brand: string
  categoryId: string
  categoryName: string
  price: number
  originalPrice: number
  rating: number
  reviewCount: number
  soldCount: number
  stock: number
  emoji: string
  description: string
  details: string[]
  variants: ProductVariant[]
  featured?: boolean
  isNew?: boolean
  badge?: "sale" | "hot" | "new"
  imageHue: [number, number]
}

export interface Category {
  id: string
  name: string
  emoji: string
  description: string
}

export const categories: Category[] = [
  { id: "dien-thoai", name: "Điện thoại", emoji: "📱", description: "Smartphone chính hãng" },
  { id: "laptop", name: "Laptop", emoji: "💻", description: "Laptop văn phòng & gaming" },
  { id: "tai-nghe", name: "Tai nghe", emoji: "🎧", description: "Tai nghe không dây, chụp tai" },
  { id: "dong-ho", name: "Đồng hồ", emoji: "⌚", description: "Đồng hồ thông minh & cơ" },
  { id: "thoi-trang", name: "Thời trang", emoji: "👕", description: "Quần áo, giày dép" },
  { id: "gia-dung", name: "Gia dụng", emoji: "🏠", description: "Đồ dùng nhà bếp, nhà cửa" },
  { id: "lam-dep", name: "Làm đẹp", emoji: "💄", description: "Mỹ phẩm & chăm sóc da" },
  { id: "sach", name: "Sách", emoji: "📚", description: "Sách hay mỗi ngày" },
]

export const products: Product[] = [
  {
    id: "dt-1",
    name: 'Điện thoại thông minh Nova X15 Pro 5G (16GB/512GB)',
    brand: "Nova",
    categoryId: "dien-thoai",
    categoryName: "Điện thoại",
    price: 18990000,
    originalPrice: 21990000,
    rating: 4.9,
    reviewCount: 1284,
    soldCount: 2310,
    stock: 45,
    emoji: "📱",
    description:
      "Chip xử lý mạnh mẽ thế hệ mới, camera 200MP chụp đêm sắc nét, pin 5500mAh sạc nhanh 120W. Màn hình AMOLED 6.8 inch 144Hz với viền siêu mỏng.",
    details: [
      "Màn hình: 6.8\" AMOLED 144Hz, 3200x1440",
      "Chip: Nova X15 5G (4nm)",
      "Camera: 200MP OIS + 50MP ultra + 12MP tele",
      "Pin: 5500mAh, sạc nhanh 120W",
      "RAM 16GB + ROM 512GB",
    ],
    variants: [
      { label: "Đen", colorHex: "#1f2937" },
      { label: "Xanh", colorHex: "#1e40af" },
      { label: "Bạc", colorHex: "#9ca3af" },
    ],
    featured: true,
    badge: "hot",
    imageHue: [220, 260],
  },
  {
    id: "dt-2",
    name: "Điện thoại thông minh Lotus 12A (6GB/128GB)",
    brand: "Lotus",
    categoryId: "dien-thoai",
    categoryName: "Điện thoại",
    price: 5490000,
    originalPrice: 6990000,
    rating: 4.7,
    reviewCount: 3521,
    soldCount: 8920,
    stock: 120,
    emoji: "📲",
    description:
      "Lựa chọn tối ưu cho người dùng phổ thông: pin trâu, màn hình lớn, hiệu năng ổn định và camera đủ dùng. Hoàn hảo cho học tập, công việc mỗi ngày.",
    details: [
      "Màn hình: 6.6\" IPS 90Hz",
      "Chip: Lotus A15",
      "Camera: 50MP AI + 2MP macro",
      "Pin: 5000mAh, sạc nhanh 33W",
      "RAM 6GB + ROM 128GB",
    ],
    variants: [
      { label: "Đen", colorHex: "#111827" },
      { label: "Xanh lá", colorHex: "#166534" },
    ],
    badge: "sale",
    imageHue: [150, 190],
  },
  {
    id: "lt-1",
    name: "Laptop UltraBook 14 Air Max (i7/16GB/1TB SSD)",
    brand: "MaxBook",
    categoryId: "laptop",
    categoryName: "Laptop",
    price: 26990000,
    originalPrice: 29990000,
    rating: 4.8,
    reviewCount: 763,
    soldCount: 1240,
    stock: 28,
    emoji: "💻",
    description:
      "Siêu mỏng nhẹ chỉ 1.2kg, màn hình OLED 14 inch 2.8K, bàn phím backlit. Làm việc suốt ngày dài với pin 18 tiếng và cổng Thunderbolt 4.",
    details: [
      "Màn hình: 14\" OLED 2.8K 90Hz",
      "CPU: Core i7-1370P",
      "RAM: 16GB LPDDR5",
      "SSD: 1TB NVMe",
      "Pin: 70Wh, sạc nhanh 65W",
    ],
    variants: [
      { label: "Bạc", colorHex: "#cbd5e1" },
      { label: "Xám", colorHex: "#374151" },
    ],
    featured: true,
    imageHue: [0, 30],
  },
  {
    id: "lt-2",
    name: "Laptop Gaming Raptor G16 (i9/RTX 4070/32GB)",
    brand: "Raptor",
    categoryId: "laptop",
    categoryName: "Laptop",
    price: 44990000,
    originalPrice: 48990000,
    rating: 4.9,
    reviewCount: 412,
    soldCount: 610,
    stock: 15,
    emoji: "🎮",
    description:
      "Sức mạnh gaming đỉnh cao với RTX 4070 8GB, màn hình 16 inch 240Hz QHD. Tản nhiệt buồng hơi giữ máy mát mẻ trong mọi trận đấu.",
    details: [
      "Màn hình: 16\" QHD 240Hz",
      "GPU: RTX 4070 8GB",
      "CPU: Core i9-14900HX",
      "RAM: 32GB DDR5",
      "SSD: 1TB NVMe Gen4",
    ],
    variants: [
      { label: "Đen", colorHex: "#0f172a" },
    ],
    badge: "hot",
    imageHue: [330, 360],
  },
  {
    id: "tn-1",
    name: "Tai nghe không dây AirSonic Pro (ANC)",
    brand: "Sonic",
    categoryId: "tai-nghe",
    categoryName: "Tai nghe",
    price: 2890000,
    originalPrice: 3490000,
    rating: 4.8,
    reviewCount: 2987,
    soldCount: 12400,
    stock: 200,
    emoji: "🎧",
    description:
      "Chống ồn chủ động -45dB, âm thanh Hi-Res, chống nước IPX5. Thời lượng pin lên tới 36 giờ cùng hộp sạc không dây nhỏ gọn.",
    details: [
      "ANC: -45dB, xuyên âm thông minh",
      "Pin: 8h + 28h (hộp sạc)",
      "Bluetooth 5.4, chống nước IPX5",
      "Driver: 11mm, Hi-Res Audio",
      "Sạc nhanh: 10 phút = 3 giờ nghe",
    ],
    variants: [
      { label: "Trắng", colorHex: "#f1f5f9" },
      { label: "Đen", colorHex: "#0f172a" },
      { label: "Xanh mint", colorHex: "#0d9488" },
    ],
    featured: true,
    badge: "sale",
    imageHue: [170, 210],
  },
  {
    id: "tn-2",
    name: "Tai nghe chụp tai StudioSound Max",
    brand: "StudioSound",
    categoryId: "tai-nghe",
    categoryName: "Tai nghe",
    price: 4590000,
    originalPrice: 5290000,
    rating: 4.7,
    reviewCount: 1542,
    soldCount: 4830,
    stock: 60,
    emoji: "🎶",
    description:
      "Trải nghiệm âm thanh vòm 360 độ, đệm tai mềm mại, gập gọn tiện lợi. Lý tưởng cho dân sáng tạo nội dung và nghe nhạc chất lượng cao.",
    details: [
      "Driver: 40mm động, Hi-Res",
      "ANC + chế độ trong suốt",
      "Pin: 40h, sạc USB-C",
      "Kết nối đa thiết bị (2 màn hình)",
      "Micro 4 con khử tiếng ồn",
    ],
    variants: [
      { label: "Đen", colorHex: "#111827" },
      { label: "Be", colorHex: "#d6c7b5" },
    ],
    imageHue: [25, 55],
  },
  {
    id: "dh-1",
    name: "Đồng hồ thông minh FitTrack GT4",
    brand: "FitTrack",
    categoryId: "dong-ho",
    categoryName: "Đồng hồ",
    price: 3490000,
    originalPrice: 3990000,
    rating: 4.6,
    reviewCount: 876,
    soldCount: 3210,
    stock: 85,
    emoji: "⌚",
    description:
      "Theo dõi sức khỏe toàn diện: nhịp tim, SpO2, giấc ngủ, GPS. Chống nước 5ATM, màn hình AMOLED luôn bật, hơn 120 chế độ thể thao.",
    details: [
      "Màn hình: 1.43\" AMOLED",
      "GPS kép, chống nước 5ATM",
      "Theo dõi nhịp tim 24/7, SpO2",
      "Pin: 14 ngày sử dụng",
      "100+ chế độ thể thao",
    ],
    variants: [
      { label: "Đen", colorHex: "#111827" },
      { label: "Xám", colorHex: "#6b7280" },
    ],
    imageHue: [210, 250],
  },
  {
    id: "dh-2",
    name: "Đồng hồ cơ Zenith Automatic 42mm",
    brand: "Zenith",
    categoryId: "dong-ho",
    categoryName: "Đồng hồ",
    price: 8790000,
    originalPrice: 8790000,
    rating: 4.9,
    reviewCount: 198,
    soldCount: 450,
    stock: 12,
    emoji: "🕰️",
    description:
      "Bộ máy automatic tinh xảo, mặt kính sapphire chống trầy, dây da cao cấp. Món phụ kiện kinh điển cho quý ông thành đạt.",
    details: [
      "Máy: Automatic Nhật Bản",
      "Kính: Sapphire chống trầy",
      "Vỏ: Thép không gỉ 316L",
      "Chống nước: 50m",
      "Dây da thật, khóa gập",
    ],
    variants: [
      { label: "Đen", colorHex: "#1f2937" },
      { label: "Nâu", colorHex: "#7c4a21" },
    ],
    badge: "new",
    imageHue: [30, 60],
  },
  {
    id: "tt-1",
    name: "Áo sơ mi công sở Unisex Cotton 260",
    brand: "UrbanWear",
    categoryId: "thoi-trang",
    categoryName: "Thời trang",
    price: 399000,
    originalPrice: 550000,
    rating: 4.5,
    reviewCount: 4210,
    soldCount: 21300,
    stock: 350,
    emoji: "👔",
    description:
      "Chất vải cotton cao cấp co giãn 4 chiều, thoáng mát không nhăn. Thiết kế ôm vừa vặn, phù hợp môi trường công sở và đi chơi.",
    details: [
      "Chất liệu: Cotton 260gsm",
      "Co giãn 4 chiều, không nhăn",
      "Form ôm, nhiều size S-XXL",
      "Màu không phai sau giặt",
    ],
    variants: [
      { label: "Trắng", colorHex: "#f8fafc" },
      { label: "Xanh dương", colorHex: "#1d4ed8" },
      { label: "Hồng", colorHex: "#db2777" },
    ],
    badge: "sale",
    imageHue: [200, 230],
  },
  {
    id: "tt-2",
    name: "Giày thể thao chạy bộ AirRun Flex",
    brand: "AirRun",
    categoryId: "thoi-trang",
    categoryName: "Thời trang",
    price: 1890000,
    originalPrice: 2390000,
    rating: 4.7,
    reviewCount: 1865,
    soldCount: 8760,
    stock: 140,
    emoji: "👟",
    description:
      "Đế giảm sốc siêu nhẹ, lưới thoáng khí, độ bám tốt trên mọi địa hình. Đồng hành cùng bạn trên mọi cung đường.",
    details: [
      "Đế: EVA + cao su bền bỉ",
      "Thân giày: lưới thoáng khí",
      "Trọng lượng: 240g",
      "Size: 36-45",
    ],
    variants: [
      { label: "Đen", colorHex: "#111827" },
      { label: "Trắng", colorHex: "#f8fafc" },
    ],
    imageHue: [260, 290],
  },
  {
    id: "gd-1",
    name: "Máy xay sinh tố đa năng Blender Max 2.0",
    brand: "HomeMax",
    categoryId: "gia-dung",
    categoryName: "Gia dụng",
    price: 1290000,
    originalPrice: 1690000,
    rating: 4.6,
    reviewCount: 2314,
    soldCount: 11200,
    stock: 95,
    emoji: "🥤",
    description:
      "Công suất 1200W, lưỡi dao 6 cánh, cối thủy tinh 1.75L. Xay đá, sinh tố, hạt khô đều mượt mà. An toàn, dễ vệ sinh.",
    details: [
      "Công suất: 1200W",
      "Cối thủy tinh 1.75L",
      "6 cánh dao, 8 tốc độ + turbo",
      "Chức năng tự làm sạch",
    ],
    variants: [
      { label: "Trắng", colorHex: "#f1f5f9" },
      { label: "Xanh dương", colorHex: "#2563eb" },
    ],
    badge: "sale",
    imageHue: [140, 175],
  },
  {
    id: "gd-2",
    name: "Bộ nồi inox cao cấp 5 món (chảo + nồi)",
    brand: "HomeMax",
    categoryId: "gia-dung",
    categoryName: "Gia dụng",
    price: 2490000,
    originalPrice: 2990000,
    rating: 4.8,
    reviewCount: 1092,
    soldCount: 5320,
    stock: 70,
    emoji: "🍳",
    description:
      "Inox 304 dày 3 lớp, đáy từ dùng được cho bếp từ. Không dính, không độc hại, phân phối nhiệt đều giúp món ăn thơm ngon hơn.",
    details: [
      "Chất liệu: Inox 304 3 lớp",
      "Dùng được bếp từ, gas, hồng ngoại",
      "Gồm: 2 nồi, 1 chảo sâu, 1 chảo, 1 xoong",
      "Nắp kính chịu nhiệt",
    ],
    variants: [
      { label: "Bạc", colorHex: "#cbd5e1" },
    ],
    imageHue: [0, 20],
  },
  {
    id: "ld-1",
    name: "Bộ dưỡng da Vitamin C + Hyaluronic",
    brand: "GlowLab",
    categoryId: "lam-dep",
    categoryName: "Làm đẹp",
    price: 649000,
    originalPrice: 890000,
    rating: 4.7,
    reviewCount: 3421,
    soldCount: 18900,
    stock: 260,
    emoji: "✨",
    description:
      "Serum Vitamin C 15% kết hợp Hyaluronic Acid giúp làm sáng da, giảm thâm, cấp ẩm sâu. An toàn cho mọi loại da, không gây kích ứng.",
    details: [
      "Serum Vitamin C 15% - 30ml",
      "Kem dưỡng HA - 50ml",
      "Không parabens, không hương liệu",
      "Phù hợp mọi loại da",
    ],
    variants: [
      { label: "Cam", colorHex: "#ea580c" },
    ],
    badge: "hot",
    imageHue: [20, 50],
  },
  {
    id: "ld-2",
    name: "Kem chống nắng SPF 50+ PA++++ toàn thân",
    brand: "GlowLab",
    categoryId: "lam-dep",
    categoryName: "Làm đẹp",
    price: 329000,
    originalPrice: 399000,
    rating: 4.8,
    reviewCount: 5678,
    soldCount: 30400,
    stock: 420,
    emoji: "🧴",
    description:
      "Chống nắng quang phổ rộng, thấm nhanh không nhờn dính, không để lại vệt trắng. An toàn cho da nhạy cảm và da dầu.",
    details: [
      "SPF 50+ PA++++",
      "Quang phổ rộng (UVA/UVB)",
      "Không nhờn, không vệt trắng",
      "Thích hợp da nhạy cảm",
    ],
    variants: [
      { label: "Trắng", colorHex: "#f8fafc" },
    ],
    imageHue: [45, 75],
  },
  {
    id: "sach-1",
    name: "Sách Nhà Giả Kim - Paulo Coelho (bìa cứng)",
    brand: "NXB Hội Nhà Văn",
    categoryId: "sach",
    categoryName: "Sách",
    price: 89000,
    originalPrice: 110000,
    rating: 4.9,
    reviewCount: 8902,
    soldCount: 45200,
    stock: 500,
    emoji: "📖",
    description:
      "Cuốn sách truyền cảm hứng được yêu thích nhất mọi thời đại. Hành trình của chàng chăn cừu Santiago trong tìm kiếm kho báu và ý nghĩa cuộc sống.",
    details: [
      "Tác giả: Paulo Coelho",
      "Dịch giả: Lê Chu Cầu",
      "Số trang: 224",
      "Bìa cứng, chữ rõ ràng",
    ],
    variants: [
      { label: "Bìa cứng", colorHex: "#b45309" },
      { label: "Bìa mềm", colorHex: "#92400e" },
    ],
    badge: "hot",
    imageHue: [30, 50],
  },
  {
    id: "sach-2",
    name: "Sách Tư Duy Nhanh Và Chậm - Daniel Kahneman",
    brand: "NXB Trẻ",
    categoryId: "sach",
    categoryName: "Sách",
    price: 165000,
    originalPrice: 195000,
    rating: 4.8,
    reviewCount: 2314,
    soldCount: 15900,
    stock: 380,
    emoji: "🧠",
    description:
      "Giải Nobel Kinh tế Daniel Kahneman hé lộ hai hệ thống tư duy điều khiển mọi quyết định của bạn. Cuốn sách kinh điển về tâm lý học hành vi.",
    details: [
      "Tác giả: Daniel Kahneman",
      "Số trang: 600",
      "Nhà xuất bản Trẻ",
      "Dịch thuật chính xác, dễ đọc",
    ],
    variants: [
      { label: "Bìa mềm", colorHex: "#1e3a8a" },
    ],
    imageHue: [220, 250],
  },
]

export function getProductById(id: string): Product | undefined {
  return products.find((p) => p.id === id)
}

export function getRelatedProducts(product: Product, limit = 4): Product[] {
  return products.filter((p) => p.id !== product.id && p.categoryId === product.categoryId).slice(0, limit)
}

export { formatVND } from "@/lib/utils"
