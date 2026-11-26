# ==============================================================================
# SCRIPT RESET DỮ LIỆU SAGA: CHỐNG XUNG ĐỘT KHI CHUYỂN HOẶC CHẠY LUỒNG HYBRID
# Giữ nguyên 100% dữ liệu Identity (users/addresses), Catalog (sản phẩm), Cart (giỏ hàng)
# ==============================================================================

$ErrorActionPreference = "Stop"

Write-Host " Đang dừng kết nối cũ và tạo mới sạch order_db & payment_db..." -ForegroundColor Cyan
docker exec -i ecommerce-postgres psql -U ecommerce -d postgres -v ON_ERROR_STOP=1 -c @"
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname IN ('order_db', 'payment_db') AND pid <> pg_backend_pid();
"@

docker exec -i ecommerce-postgres psql -U ecommerce -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS order_db;"
docker exec -i ecommerce-postgres psql -U ecommerce -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE order_db;"
docker exec -i ecommerce-postgres psql -U ecommerce -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS payment_db;"
docker exec -i ecommerce-postgres psql -U ecommerce -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE payment_db;"

Write-Host " Đang reset trạng thái giữ kho inventory_db (giữ nguyên bảng stock, đưa reserved = 0)..." -ForegroundColor Cyan
docker exec -i ecommerce-postgres psql -U ecommerce -d inventory_db -v ON_ERROR_STOP=1 -c @"
DROP TABLE IF EXISTS outbox_events, stock_reservation_items, stock_reservations, processed_events CASCADE;
DO `$`$
BEGIN
    IF to_regclass('public.stock') IS NOT NULL THEN
        UPDATE stock SET reserved = 0;
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        -- 000004 alters the stock_reservations CHECK, so it must re-run with 000003.
        DELETE FROM schema_migrations WHERE version LIKE '000003_%' OR version LIKE '000004_%';
    END IF;
END `$`$;
"@

Write-Host " Đang reset processed_events trong cart_db..." -ForegroundColor Cyan
docker exec -i ecommerce-postgres psql -U ecommerce -d cart_db -v ON_ERROR_STOP=1 -c @"
DROP TABLE IF EXISTS processed_events CASCADE;
DO `$`$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        DELETE FROM schema_migrations WHERE version LIKE '000004_%';
    END IF;
END `$`$;
"@

Write-Host "🧹 Đang xóa các queue RabbitMQ cũ để service tự tạo lại topology chuẩn..." -ForegroundColor Cyan
$sagaQueues = @(
    "inventory.order-events", "inventory.order-events.dlq",
    "cart.order-events", "cart.order-events.dlq",
    "payment.order-events", "payment.order-events.dlq",
    "payment.session-events", "payment.session-events.dlq",
    "order.payment-events", "order.payment-events.dlq",
    "order.validation-events", "order.validation-events.dlq",
    "order.saga-events", "order.saga-events.dlq"
)

foreach ($q in $sagaQueues) {
    docker exec ecommerce-rabbitmq rabbitmqctl delete_queue $q 2>$null | Out-Null
}

Write-Host " Hoàn tất reset state! Hệ thống đã sẵn sàng 100% để khởi chạy luồng Hybrid." -ForegroundColor Green
