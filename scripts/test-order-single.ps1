param(
    [ValidateSet("COD", "MOMO")]
    [string]$Method = "COD",
    [string]$BaseUrl = "http://localhost:8000/api/v1",
    [string]$CustomerEmail = "customer1@gmail.com",
    [string]$CustomerPassword = "123456123456"
)

$ErrorActionPreference = "Stop"

Write-Host "=== 1. ĐĂNG KÝ / ĐĂNG NHẬP KHÁCH HÀNG ===" -ForegroundColor Cyan
try {
    $registerBody = @{
        email    = $CustomerEmail
        password = $CustomerPassword
        fullName = "Test Customer"
    } | ConvertTo-Json -Compress
    Invoke-RestMethod -Method Post -Uri "$BaseUrl/auth/register" -ContentType "application/json" -Body $registerBody -TimeoutSec 5 | Out-Null
} catch {}

$loginBody = @{
    email    = $CustomerEmail
    password = $CustomerPassword
} | ConvertTo-Json -Compress

$loginResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/auth/login" -ContentType "application/json" -Body $loginBody
$token = $loginResp.data.accessToken
Write-Host " Đăng nhập thành công! Token: $($token.Substring(0, 15))..." -ForegroundColor Green

Write-Host "`n=== 2. LẤY SẢN PHẨM & SKU TỪ GIAN HÀNG ===" -ForegroundColor Cyan
$prodResp = Invoke-RestMethod -Method Get -Uri "$BaseUrl/storefront/products?page=1&pageSize=1"
if (-not $prodResp.data.items -or $prodResp.data.items.Count -eq 0) {
    Write-Host "❌ Không tìm thấy sản phẩm nào trong Catalog. Cần seed dữ liệu sản phẩm trước." -ForegroundColor Red
    exit 1
}

# The list response omits variants; the detail response carries the SKUs.
$prod = (Invoke-RestMethod -Method Get -Uri "$BaseUrl/storefront/products/$($prodResp.data.items[0].id)").data
$sku = $prod.variants[0]
Write-Host " Sản phẩm: $($prod.name) (ID: $($prod.id), SKU: $($sku.skuId), Giá: $($sku.price) VND)" -ForegroundColor Green

$idemKey = "single-test-$(Get-Date -Format 'yyyyMMddHHmmss')-$(Get-Random -Minimum 1000 -Maximum 9999)"
Write-Host "`n=== 3. GỬI YÊU CẦU TẠO ĐƠN HÀNG (Method: $Method, Idempotency-Key: $idemKey) ===" -ForegroundColor Cyan

$orderPayload = @{
    paymentMethod   = $Method
    note            = "Test đơn hàng Hybrid thủ công"
    shippingAddress = @{
        recipientName = "Nguyễn Văn Test"
        phone         = "0987654321"
        provinceCode  = "01"
        provinceName  = "Thành phố Hà Nội"
        wardCode      = "00001"
        wardName      = "Phường Phúc Xá"
        addressLine   = "123 Đường Phố Mới"
        label         = "Nhà riêng"
    }
    items           = @(
        @{
            productId              = $prod.id
            skuId                  = $sku.skuId
            quantity               = 1
            unitPrice              = $sku.price
            expectedName           = $prod.name
            brand                  = $(if ($prod.brand) { $prod.brand } else { "DreamShop" })
            categoryName           = $(if ($prod.categoryName) { $prod.categoryName } else { "Thiết bị" })
            variantLabel           = $(if ($sku.label) { $sku.label } else { "Mặc định" })
            imageUrl               = ""
            imageEmoji             = "📦"
            expectedProductVersion = $(if ($prod.version) { $prod.version } else { 1 })
        }
    )
} | ConvertTo-Json -Depth 5

$headers = @{
    "Authorization"   = "Bearer $token"
    "Idempotency-Key" = $idemKey
}

try {
    $orderResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/orders" -Headers $headers -ContentType "application/json" -Body $orderPayload
    Write-Host " TẠO ĐƠN THÀNH CÔNG (HTTP 201 Created)!" -ForegroundColor Green
    Write-Host "Mã đơn (OrderCode): $($orderResp.data.order.orderCode)"
    Write-Host "Trạng thái (Status): $($orderResp.data.order.status)"
    Write-Host "Tổng tiền: $($orderResp.data.order.totalAmount) VND"
    if ($orderResp.data.payUrl) {
        Write-Host "MoMo PayURL: $($orderResp.data.payUrl)" -ForegroundColor Yellow
    }

    Write-Host "`n=== 4. TEST LŨY ĐẲNG (Gửi lại cùng key để xác nhận replay) ===" -ForegroundColor Cyan
    $replayResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/orders" -Headers $headers -ContentType "application/json" -Body $orderPayload
    Write-Host " Replay thành công, nhận lại đúng đơn: $($replayResp.data.order.orderCode)" -ForegroundColor Green
} catch {
    Write-Host "❌ Lỗi tạo đơn: $($_.Exception.Message)" -ForegroundColor Red
}
