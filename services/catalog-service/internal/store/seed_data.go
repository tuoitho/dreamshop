package store

import (
	"fmt"

	"ecommerce.local/catalog-service/internal/domain"
)

// seedItem is a compact representation of one seed product. The data mirrors
// ecommerce-fe/src/data/products.ts exactly (ids, names, brands, categories,
// prices, rating, reviewCount, soldCount, emoji, description, details,
// variants, featured/isNew/badge, imageHue) so frontend cart and detail flows
// keep working against the served catalog. Stock is intentionally absent:
// Inventory owns availability.
type seedItem struct {
	id                 string
	name               string
	brand              string
	categoryID         string
	categoryName       string
	priceMinor         int64
	originalPriceMinor int64
	rating             float64
	reviewCount        int64
	soldCount          int64
	emoji              string
	description        string
	details            []string
	variants           []domain.ProductVariant
	featured           bool
	isNew              bool
	badge              string
	imageHue           []uint32
}

func (s seedItem) product() *domain.Product {
	images := make([]string, 0, 4)
	for n := 1; n <= 4; n++ {
		images = append(images, fmt.Sprintf("https://picsum.photos/seed/%s-%d/800/800", s.id, n))
	}
	return &domain.Product{
		ID:                 s.id,
		Name:               s.name,
		Brand:              s.brand,
		CategoryID:         s.categoryID,
		CategoryName:       s.categoryName,
		PriceMinor:         s.priceMinor,
		OriginalPriceMinor: s.originalPriceMinor,
		Images:             images,
		Rating:             s.rating,
		ReviewCount:        s.reviewCount,
		SoldCount:          s.soldCount,
		IsNew:              s.isNew,
		Badge:              s.badge,
		Description:        s.description,
		Featured:           s.featured,
		Emoji:              s.emoji,
		ImageHue:           s.imageHue,
		Variants:           s.variants,
		Details:            s.details,
	}
}

func seedProducts() []*domain.Product {
	items := []seedItem{
		{
			id: "dt-1", name: "Điện thoại thông minh Nova X15 Pro 5G (16GB/512GB)",
			brand: "Nova", categoryID: "dien-thoai", categoryName: "Điện thoại",
			priceMinor: 18990000, originalPriceMinor: 21990000,
			rating: 4.9, reviewCount: 1284, soldCount: 2310, emoji: "📱",
			description: "Chip xử lý mạnh mẽ thế hệ mới, camera 200MP chụp đêm sắc nét, pin 5500mAh sạc nhanh 120W. Màn hình AMOLED 6.8 inch 144Hz với viền siêu mỏng.",
			details: []string{
				"Màn hình: 6.8\" AMOLED 144Hz, 3200x1440",
				"Chip: Nova X15 5G (4nm)",
				"Camera: 200MP OIS + 50MP ultra + 12MP tele",
				"Pin: 5500mAh, sạc nhanh 120W",
				"RAM 16GB + ROM 512GB",
			},
			variants: []domain.ProductVariant{
				{Color: "Đen", ColorHex: "#1f2937"},
				{Color: "Xanh", ColorHex: "#1e40af"},
				{Color: "Bạc", ColorHex: "#9ca3af"},
			},
			featured: true, badge: "hot", imageHue: []uint32{220, 260},
		},
		{
			id: "dt-2", name: "Điện thoại thông minh Lotus 12A (6GB/128GB)",
			brand: "Lotus", categoryID: "dien-thoai", categoryName: "Điện thoại",
			priceMinor: 5490000, originalPriceMinor: 6990000,
			rating: 4.7, reviewCount: 3521, soldCount: 8920, emoji: "📲",
			description: "Lựa chọn tối ưu cho người dùng phổ thông: pin trâu, màn hình lớn, hiệu năng ổn định và camera đủ dùng. Hoàn hảo cho học tập, công việc mỗi ngày.",
			details: []string{
				"Màn hình: 6.6\" IPS 90Hz",
				"Chip: Lotus A15",
				"Camera: 50MP AI + 2MP macro",
				"Pin: 5000mAh, sạc nhanh 33W",
				"RAM 6GB + ROM 128GB",
			},
			variants: []domain.ProductVariant{
				{Color: "Đen", ColorHex: "#111827"},
				{Color: "Xanh lá", ColorHex: "#166534"},
			},
			badge: "sale", imageHue: []uint32{150, 190},
		},
		{
			id: "lt-1", name: "Laptop UltraBook 14 Air Max (i7/16GB/1TB SSD)",
			brand: "MaxBook", categoryID: "laptop", categoryName: "Laptop",
			priceMinor: 26990000, originalPriceMinor: 29990000,
			rating: 4.8, reviewCount: 763, soldCount: 1240, emoji: "💻",
			description: "Siêu mỏng nhẹ chỉ 1.2kg, màn hình OLED 14 inch 2.8K, bàn phím backlit. Làm việc suốt ngày dài với pin 18 tiếng và cổng Thunderbolt 4.",
			details: []string{
				"Màn hình: 14\" OLED 2.8K 90Hz",
				"CPU: Core i7-1370P",
				"RAM: 16GB LPDDR5",
				"SSD: 1TB NVMe",
				"Pin: 70Wh, sạc nhanh 65W",
			},
			variants: []domain.ProductVariant{
				{Color: "Bạc", ColorHex: "#cbd5e1"},
				{Color: "Xám", ColorHex: "#374151"},
			},
			featured: true, imageHue: []uint32{0, 30},
		},
		{
			id: "lt-2", name: "Laptop Gaming Raptor G16 (i9/RTX 4070/32GB)",
			brand: "Raptor", categoryID: "laptop", categoryName: "Laptop",
			priceMinor: 44990000, originalPriceMinor: 48990000,
			rating: 4.9, reviewCount: 412, soldCount: 610, emoji: "🎮",
			description: "Sức mạnh gaming đỉnh cao với RTX 4070 8GB, màn hình 16 inch 240Hz QHD. Tản nhiệt buồng hơi giữ máy mát mẻ trong mọi trận đấu.",
			details: []string{
				"Màn hình: 16\" QHD 240Hz",
				"GPU: RTX 4070 8GB",
				"CPU: Core i9-14900HX",
				"RAM: 32GB DDR5",
				"SSD: 1TB NVMe Gen4",
			},
			variants: []domain.ProductVariant{
				{Color: "Đen", ColorHex: "#0f172a"},
			},
			badge: "hot", imageHue: []uint32{330, 360},
		},
		{
			id: "tn-1", name: "Tai nghe không dây AirSonic Pro (ANC)",
			brand: "Sonic", categoryID: "tai-nghe", categoryName: "Tai nghe",
			priceMinor: 2890000, originalPriceMinor: 3490000,
			rating: 4.8, reviewCount: 2987, soldCount: 12400, emoji: "🎧",
			description: "Chống ồn chủ động -45dB, âm thanh Hi-Res, chống nước IPX5. Thời lượng pin lên tới 36 giờ cùng hộp sạc không dây nhỏ gọn.",
			details: []string{
				"ANC: -45dB, xuyên âm thông minh",
				"Pin: 8h + 28h (hộp sạc)",
				"Bluetooth 5.4, chống nước IPX5",
				"Driver: 11mm, Hi-Res Audio",
				"Sạc nhanh: 10 phút = 3 giờ nghe",
			},
			variants: []domain.ProductVariant{
				{Color: "Trắng", ColorHex: "#f1f5f9"},
				{Color: "Đen", ColorHex: "#0f172a"},
				{Color: "Xanh mint", ColorHex: "#0d9488"},
			},
			featured: true, badge: "sale", imageHue: []uint32{170, 210},
		},
		{
			id: "tn-2", name: "Tai nghe chụp tai StudioSound Max",
			brand: "StudioSound", categoryID: "tai-nghe", categoryName: "Tai nghe",
			priceMinor: 4590000, originalPriceMinor: 5290000,
			rating: 4.7, reviewCount: 1542, soldCount: 4830, emoji: "🎶",
			description: "Trải nghiệm âm thanh vòm 360 độ, đệm tai mềm mại, gập gọn tiện lợi. Lý tưởng cho dân sáng tạo nội dung và nghe nhạc chất lượng cao.",
			details: []string{
				"Driver: 40mm động, Hi-Res",
				"ANC + chế độ trong suốt",
				"Pin: 40h, sạc USB-C",
				"Kết nối đa thiết bị (2 màn hình)",
				"Micro 4 con khử tiếng ồn",
			},
			variants: []domain.ProductVariant{
				{Color: "Đen", ColorHex: "#111827"},
				{Color: "Be", ColorHex: "#d6c7b5"},
			},
			imageHue: []uint32{25, 55},
		},
		{
			id: "dh-1", name: "Đồng hồ thông minh FitTrack GT4",
			brand: "FitTrack", categoryID: "dong-ho", categoryName: "Đồng hồ",
			priceMinor: 3490000, originalPriceMinor: 3990000,
			rating: 4.6, reviewCount: 876, soldCount: 3210, emoji: "⌚",
			description: "Theo dõi sức khỏe toàn diện: nhịp tim, SpO2, giấc ngủ, GPS. Chống nước 5ATM, màn hình AMOLED luôn bật, hơn 120 chế độ thể thao.",
			details: []string{
				"Màn hình: 1.43\" AMOLED",
				"GPS kép, chống nước 5ATM",
				"Theo dõi nhịp tim 24/7, SpO2",
				"Pin: 14 ngày sử dụng",
				"100+ chế độ thể thao",
			},
			variants: []domain.ProductVariant{
				{Color: "Đen", ColorHex: "#111827"},
				{Color: "Xám", ColorHex: "#6b7280"},
			},
			imageHue: []uint32{210, 250},
		},
		{
			id: "dh-2", name: "Đồng hồ cơ Zenith Automatic 42mm",
			brand: "Zenith", categoryID: "dong-ho", categoryName: "Đồng hồ",
			priceMinor: 8790000, originalPriceMinor: 8790000,
			rating: 4.9, reviewCount: 198, soldCount: 450, emoji: "🕰️",
			description: "Bộ máy automatic tinh xảo, mặt kính sapphire chống trầy, dây da cao cấp. Món phụ kiện kinh điển cho quý ông thành đạt.",
			details: []string{
				"Máy: Automatic Nhật Bản",
				"Kính: Sapphire chống trầy",
				"Vỏ: Thép không gỉ 316L",
				"Chống nước: 50m",
				"Dây da thật, khóa gập",
			},
			variants: []domain.ProductVariant{
				{Color: "Đen", ColorHex: "#1f2937"},
				{Color: "Nâu", ColorHex: "#7c4a21"},
			},
			badge: "new", imageHue: []uint32{30, 60},
		},
		{
			id: "tt-1", name: "Áo sơ mi công sở Unisex Cotton 260",
			brand: "UrbanWear", categoryID: "thoi-trang", categoryName: "Thời trang",
			priceMinor: 399000, originalPriceMinor: 550000,
			rating: 4.5, reviewCount: 4210, soldCount: 21300, emoji: "👔",
			description: "Chất vải cotton cao cấp co giãn 4 chiều, thoáng mát không nhăn. Thiết kế ôm vừa vặn, phù hợp môi trường công sở và đi chơi.",
			details: []string{
				"Chất liệu: Cotton 260gsm",
				"Co giãn 4 chiều, không nhăn",
				"Form ôm, nhiều size S-XXL",
				"Màu không phai sau giặt",
			},
			variants: []domain.ProductVariant{
				{Color: "Trắng", ColorHex: "#f8fafc"},
				{Color: "Xanh dương", ColorHex: "#1d4ed8"},
				{Color: "Hồng", ColorHex: "#db2777"},
			},
			badge: "sale", imageHue: []uint32{200, 230},
		},
		{
			id: "tt-2", name: "Giày thể thao chạy bộ AirRun Flex",
			brand: "AirRun", categoryID: "thoi-trang", categoryName: "Thời trang",
			priceMinor: 1890000, originalPriceMinor: 2390000,
			rating: 4.7, reviewCount: 1865, soldCount: 8760, emoji: "👟",
			description: "Đế giảm sốc siêu nhẹ, lưới thoáng khí, độ bám tốt trên mọi địa hình. Đồng hành cùng bạn trên mọi cung đường.",
			details: []string{
				"Đế: EVA + cao su bền bỉ",
				"Thân giày: lưới thoáng khí",
				"Trọng lượng: 240g",
				"Size: 36-45",
			},
			variants: []domain.ProductVariant{
				{Color: "Đen", ColorHex: "#111827"},
				{Color: "Trắng", ColorHex: "#f8fafc"},
			},
			imageHue: []uint32{260, 290},
		},
		{
			id: "gd-1", name: "Máy xay sinh tố đa năng Blender Max 2.0",
			brand: "HomeMax", categoryID: "gia-dung", categoryName: "Gia dụng",
			priceMinor: 1290000, originalPriceMinor: 1690000,
			rating: 4.6, reviewCount: 2314, soldCount: 11200, emoji: "🥤",
			description: "Công suất 1200W, lưỡi dao 6 cánh, cối thủy tinh 1.75L. Xay đá, sinh tố, hạt khô đều mượt mà. An toàn, dễ vệ sinh.",
			details: []string{
				"Công suất: 1200W",
				"Cối thủy tinh 1.75L",
				"6 cánh dao, 8 tốc độ + turbo",
				"Chức năng tự làm sạch",
			},
			variants: []domain.ProductVariant{
				{Color: "Trắng", ColorHex: "#f1f5f9"},
				{Color: "Xanh dương", ColorHex: "#2563eb"},
			},
			badge: "sale", imageHue: []uint32{140, 175},
		},
		{
			id: "gd-2", name: "Bộ nồi inox cao cấp 5 món (chảo + nồi)",
			brand: "HomeMax", categoryID: "gia-dung", categoryName: "Gia dụng",
			priceMinor: 2490000, originalPriceMinor: 2990000,
			rating: 4.8, reviewCount: 1092, soldCount: 5320, emoji: "🍳",
			description: "Inox 304 dày 3 lớp, đáy từ dùng được cho bếp từ. Không dính, không độc hại, phân phối nhiệt đều giúp món ăn thơm ngon hơn.",
			details: []string{
				"Chất liệu: Inox 304 3 lớp",
				"Dùng được bếp từ, gas, hồng ngoại",
				"Gồm: 2 nồi, 1 chảo sâu, 1 chảo, 1 xoong",
				"Nắp kính chịu nhiệt",
			},
			variants: []domain.ProductVariant{
				{Color: "Bạc", ColorHex: "#cbd5e1"},
			},
			imageHue: []uint32{0, 20},
		},
		{
			id: "ld-1", name: "Bộ dưỡng da Vitamin C + Hyaluronic",
			brand: "GlowLab", categoryID: "lam-dep", categoryName: "Làm đẹp",
			priceMinor: 649000, originalPriceMinor: 890000,
			rating: 4.7, reviewCount: 3421, soldCount: 18900, emoji: "✨",
			description: "Serum Vitamin C 15% kết hợp Hyaluronic Acid giúp làm sáng da, giảm thâm, cấp ẩm sâu. An toàn cho mọi loại da, không gây kích ứng.",
			details: []string{
				"Serum Vitamin C 15% - 30ml",
				"Kem dưỡng HA - 50ml",
				"Không parabens, không hương liệu",
				"Phù hợp mọi loại da",
			},
			variants: []domain.ProductVariant{
				{Color: "Cam", ColorHex: "#ea580c"},
			},
			badge: "hot", imageHue: []uint32{20, 50},
		},
		{
			id: "ld-2", name: "Kem chống nắng SPF 50+ PA++++ toàn thân",
			brand: "GlowLab", categoryID: "lam-dep", categoryName: "Làm đẹp",
			priceMinor: 329000, originalPriceMinor: 399000,
			rating: 4.8, reviewCount: 5678, soldCount: 30400, emoji: "🧴",
			description: "Chống nắng quang phổ rộng, thấm nhanh không nhờn dính, không để lại vệt trắng. An toàn cho da nhạy cảm và da dầu.",
			details: []string{
				"SPF 50+ PA++++",
				"Quang phổ rộng (UVA/UVB)",
				"Không nhờn, không vệt trắng",
				"Thích hợp da nhạy cảm",
			},
			variants: []domain.ProductVariant{
				{Color: "Trắng", ColorHex: "#f8fafc"},
			},
			imageHue: []uint32{45, 75},
		},
		{
			id: "sach-1", name: "Sách Nhà Giả Kim - Paulo Coelho (bìa cứng)",
			brand: "NXB Hội Nhà Văn", categoryID: "sach", categoryName: "Sách",
			priceMinor: 89000, originalPriceMinor: 110000,
			rating: 4.9, reviewCount: 8902, soldCount: 45200, emoji: "📖",
			description: "Cuốn sách truyền cảm hứng được yêu thích nhất mọi thời đại. Hành trình của chàng chăn cừu Santiago trong tìm kiếm kho báu và ý nghĩa cuộc sống.",
			details: []string{
				"Tác giả: Paulo Coelho",
				"Dịch giả: Lê Chu Cầu",
				"Số trang: 224",
				"Bìa cứng, chữ rõ ràng",
			},
			variants: []domain.ProductVariant{
				{Color: "Bìa cứng", ColorHex: "#b45309"},
				{Color: "Bìa mềm", ColorHex: "#92400e"},
			},
			badge: "hot", imageHue: []uint32{30, 50},
		},
		{
			id: "sach-2", name: "Sách Tư Duy Nhanh Và Chậm - Daniel Kahneman",
			brand: "NXB Trẻ", categoryID: "sach", categoryName: "Sách",
			priceMinor: 165000, originalPriceMinor: 195000,
			rating: 4.8, reviewCount: 2314, soldCount: 15900, emoji: "🧠",
			description: "Giải Nobel Kinh tế Daniel Kahneman hé lộ hai hệ thống tư duy điều khiển mọi quyết định của bạn. Cuốn sách kinh điển về tâm lý học hành vi.",
			details: []string{
				"Tác giả: Daniel Kahneman",
				"Số trang: 600",
				"Nhà xuất bản Trẻ",
				"Dịch thuật chính xác, dễ đọc",
			},
			variants: []domain.ProductVariant{
				{Color: "Bìa mềm", ColorHex: "#1e3a8a"},
			},
			imageHue: []uint32{220, 250},
		},
	}

	products := make([]*domain.Product, 0, len(items))
	for _, item := range items {
		products = append(products, item.product())
	}
	return products
}
