#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# SCRIPT CHẠY THỬ NGHIỆM ĐƠN LẺ: TẠO ĐƠN HÀNG HYBRID & KIỂM TRA LŨY ĐẲNG
# ==============================================================================

BASE_URL="${BASE_URL:-http://localhost:8000/api/v1}"
CUSTOMER_EMAIL="${CUSTOMER_EMAIL:-customer1@gmail.com}"
CUSTOMER_PASS="${CUSTOMER_PASS:-123456123456}"
METHOD="${1:-COD}" # COD hoặc MOMO

echo "=== 1. ĐĂNG KÝ / ĐĂNG NHẬP KHÁCH HÀNG ==="
curl -s -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$CUSTOMER_EMAIL\",\"password\":\"$CUSTOMER_PASS\",\"fullName\":\"Test Customer\"}" >/dev/null 2>&1 || true

LOGIN_RESP=$(curl -s -f -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$CUSTOMER_EMAIL\",\"password\":\"$CUSTOMER_PASS\"}")

TOKEN=$(echo "$LOGIN_RESP" | grep -o '"accessToken":"[^"]*' | cut -d'"' -f4)
if [ -z "$TOKEN" ]; then
  echo "❌ Đăng nhập thất bại. Kiểm tra Kong (:8000) và identity-service (:8080) đã bật chưa."
  exit 1
fi
echo " Đăng nhập thành công! Token: ${TOKEN:0:15}..."

echo -e "\n=== 2. LẤY SẢN PHẨM & SKU TỪ GIAN HÀNG ==="
PRODUCTS_RESP=$(curl -s -f "$BASE_URL/storefront/products?page=1&pageSize=1")
PRODUCT_ID=$(echo "$PRODUCTS_RESP" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
PRODUCT_NAME=$(echo "$PRODUCTS_RESP" | grep -o '"name":"[^"]*' | head -1 | cut -d'"' -f4)
PRICE=$(echo "$PRODUCTS_RESP" | grep -o '"price":[0-9]*' | head -1 | cut -d':' -f2)

DETAIL_RESP=$(curl -s -f "$BASE_URL/storefront/products/$PRODUCT_ID")
SKU_ID=$(echo "$DETAIL_RESP" | grep -o '"skuId":"[^"]*' | head -1 | cut -d'"' -f4)

if [ -z "$PRODUCT_ID" ] || [ -z "$SKU_ID" ]; then
  echo "❌ Không tìm thấy sản phẩm nào trong Catalog. Hãy đảm bảo catalog-service đã được seed dữ liệu."
  exit 1
fi
echo " Sản phẩm: $PRODUCT_NAME (ID: $PRODUCT_ID, SKU: $SKU_ID, Giá: $PRICE VND)"

IDEM_KEY="single-test-$(date +%s)-$RANDOM"
echo -e "\n=== 3. GỬI YÊU CẦU TẠO ĐƠN HÀNG (Method: $METHOD, Idempotency-Key: $IDEM_KEY) ==="

ORDER_PAYLOAD=$(cat <<EOF
{
  "paymentMethod": "$METHOD",
  "note": "Test đơn hàng Hybrid thủ công",
  "shippingAddress": {
    "recipientName": "Nguyễn Văn Test",
    "phone": "0987654321",
    "provinceCode": "01",
    "provinceName": "Thành phố Hà Nội",
    "wardCode": "00001",
    "wardName": "Phường Phúc Xá",
    "addressLine": "123 Đường Phố Mới",
    "label": "Nhà riêng"
  },
  "items": [
    {
      "productId": "$PRODUCT_ID",
      "skuId": "$SKU_ID",
      "quantity": 1,
      "unitPrice": $PRICE,
      "expectedName": "$PRODUCT_NAME",
      "brand": "TechShop",
      "categoryName": "Thiết bị",
      "variantLabel": "Mặc định",
      "imageUrl": "",
      "imageEmoji": "📦",
      "expectedProductVersion": 1
    }
  ]
}
EOF
)

RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/orders" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: $IDEM_KEY" \
  -H "Content-Type: application/json" \
  -d "$ORDER_PAYLOAD")

HTTP_STATUS=$(echo "$RESP" | grep "HTTP_STATUS" | cut -d':' -f2)
BODY=$(echo "$RESP" | grep -v "HTTP_STATUS")

echo "HTTP Response Status: $HTTP_STATUS"
echo "Response Body:"
echo "$BODY"

if [ "$HTTP_STATUS" -eq 201 ]; then
  echo -e "\n TẠO ĐƠN THÀNH CÔNG (HTTP 201 Created)!"
  
  echo -e "\n=== 4. TEST LŨY ĐẲNG (Gửi lại cùng key để xác nhận replay) ==="
  REPLAY_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/orders" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Idempotency-Key: $IDEM_KEY" \
    -H "Content-Type: application/json" \
    -d "$ORDER_PAYLOAD")
  REPLAY_STATUS=$(echo "$REPLAY_RESP" | grep "HTTP_STATUS" | cut -d':' -f2)
  echo "Replay Status: $REPLAY_STATUS (Kỳ vọng: 201 Created)"
else
  echo -e "\n❌ Tạo đơn không thành công. Kiểm tra log của bff-service, order-service, inventory-service."
fi
