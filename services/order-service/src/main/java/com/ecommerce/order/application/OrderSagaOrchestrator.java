package com.ecommerce.order.application;

import com.ecommerce.contracts.catalog.v1.GetProductResponse;
import com.ecommerce.contracts.catalog.v1.ProductVariant;
import com.ecommerce.contracts.inventory.v1.ReservationLine;
import com.ecommerce.contracts.order.v1.CreateOrderRequest;
import com.ecommerce.contracts.order.v1.CreateOrderResponse;
import com.ecommerce.contracts.order.v1.OrderDetail;
import com.ecommerce.contracts.order.v1.OrderItem;
import com.ecommerce.contracts.order.v1.ShippingAddressSnapshot;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.config.OrderProperties;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.ShippingPolicy;
import com.ecommerce.order.domain.SnapshotMismatchDetail;
import com.ecommerce.order.integration.AuthorizationGrpcAdapter;
import com.ecommerce.order.integration.CatalogGrpcAdapter;
import com.ecommerce.order.integration.InventoryGrpcAdapter;
import com.ecommerce.order.integration.PaymentGrpcAdapter;
import com.ecommerce.order.persistence.OrderItemEntity;
import io.grpc.Status;
import java.security.SecureRandom;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.UUID;
import java.util.stream.Collectors;
import org.springframework.stereotype.Service;

@Service
public class OrderSagaOrchestrator {
    private static final String CODE_ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
    private static final DateTimeFormatter DATE_FMT = DateTimeFormatter.ofPattern("yyyyMMdd").withZone(ZoneOffset.UTC);
    private static final SecureRandom RANDOM = new SecureRandom();

    private final AuthorizationGrpcAdapter authorizationAdapter;
    private final CatalogGrpcAdapter catalogAdapter;
    private final InventoryGrpcAdapter inventoryAdapter;
    private final PaymentGrpcAdapter paymentAdapter;
    private final IdempotencyService idempotencyService;
    private final OrderTransactionStepService txStepService;
    private final OrderProperties properties;

    public OrderSagaOrchestrator(
            AuthorizationGrpcAdapter authorizationAdapter,
            CatalogGrpcAdapter catalogAdapter,
            InventoryGrpcAdapter inventoryAdapter,
            PaymentGrpcAdapter paymentAdapter,
            IdempotencyService idempotencyService,
            OrderTransactionStepService txStepService,
            OrderProperties properties) {
        this.authorizationAdapter = authorizationAdapter;
        this.catalogAdapter = catalogAdapter;
        this.inventoryAdapter = inventoryAdapter;
        this.paymentAdapter = paymentAdapter;
        this.idempotencyService = idempotencyService;
        this.txStepService = txStepService;
        this.properties = properties;
    }

    public CreateOrderResponse createOrder(CreateOrderRequest request) {
        UUID userId = parseRequiredUuid(request.getUserId(), "user_id");
        String idempotencyKey = requireNonBlank(request.getIdempotencyKey(), "idempotency_key");
        if (idempotencyKey.length() > 128) {
            throw new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", "idempotency_key exceeds 128 characters");
        }
        PaymentMethod paymentMethod = parsePaymentMethod(request.getPaymentMethod());
        validateShippingAddress(request.getShippingAddress());
        validateOrderItems(request.getItemsList());

        if (!authorizationAdapter.hasPermission(userId, "order.create")) {
            throw new DomainException(Status.Code.PERMISSION_DENIED, "ORDER_PERMISSION_DENIED", "Missing order.create permission");
        }

        String fingerprint = idempotencyService.computeFingerprint(request);
        Optional<CreateOrderResponse> preExisting = idempotencyService.checkExistingOrThrow(userId, idempotencyKey, fingerprint);
        if (preExisting.isPresent()) {
            return preExisting.get();
        }

        // Verify live catalog snapshot outside any DB transaction
        Map<String, Optional<GetProductResponse>> catalogByProductId = new LinkedHashMap<>();
        for (OrderItem item : request.getItemsList()) {
            catalogByProductId.computeIfAbsent(item.getProductId().trim(), catalogAdapter::getProduct);
        }

        Map<String, String> mismatches = new LinkedHashMap<>();
        List<OrderItemEntity> verifiedItems = new ArrayList<>(request.getItemsCount());
        long subtotalMinor = 0L;

        for (OrderItem reqItem : request.getItemsList()) {
            String productId = reqItem.getProductId().trim();
            String skuId = reqItem.getSkuId().trim();
            Optional<GetProductResponse> maybeProduct = catalogByProductId.get(productId);
            if (maybeProduct == null || maybeProduct.isEmpty() || maybeProduct.get().getDeleted()) {
                mismatches.put(skuId, new SnapshotMismatchDetail(skuId, "UNAVAILABLE", "Sản phẩm đã ngừng kinh doanh").formattedMessage());
                continue;
            }

            GetProductResponse live = maybeProduct.get();
            ProductVariant variant = findVariant(live, skuId);
            if (variant == null) {
                mismatches.put(skuId, new SnapshotMismatchDetail(skuId, "VARIANT_UNAVAILABLE", "Phân loại bạn chọn không còn tồn tại").formattedMessage());
                continue;
            }

            long livePriceMinor = variant.hasPrice() ? variant.getPrice().getAmountMinor() : live.getPrice().getAmountMinor();
            long expectedPriceMinor = reqItem.getUnitPrice().getAmountMinor();
            long liveVersion = live.getVersion() > 0 ? live.getVersion() : 1L;

            if (livePriceMinor != expectedPriceMinor) {
                mismatches.put(skuId, new SnapshotMismatchDetail(
                        skuId,
                        "PRICE_CHANGED",
                        "Giá phân loại vừa thay đổi từ " + expectedPriceMinor + " sang " + livePriceMinor).formattedMessage());
                continue;
            }

            boolean nameChanged = !reqItem.getExpectedName().isBlank()
                    && !Objects.equals(live.getName(), reqItem.getExpectedName());
            boolean labelChanged = !reqItem.getVariantLabel().isBlank()
                    && !Objects.equals(variant.getLabel(), reqItem.getVariantLabel());
            if (nameChanged || labelChanged) {
                mismatches.put(skuId, new SnapshotMismatchDetail(
                        skuId,
                        "INFO_UPDATED",
                        "Tên sản phẩm hoặc nhãn phân loại vừa được cập nhật").formattedMessage());
                continue;
            }

            if (reqItem.getExpectedProductVersion() > 0 && liveVersion != reqItem.getExpectedProductVersion()) {
                mismatches.put(skuId, new SnapshotMismatchDetail(
                        skuId,
                        "VERSION_CHANGED",
                        "Thông tin sản phẩm vừa được cập nhật mới").formattedMessage());
                continue;
            }

            long lineTotalMinor = Math.multiplyExact(livePriceMinor, (long) reqItem.getQuantity());
            subtotalMinor = Math.addExact(subtotalMinor, lineTotalMinor);

            String imageUrl = live.getImagesCount() > 0 ? live.getImages(0) : reqItem.getImageUrl();
            String imageHue = live.getImageHueList().isEmpty()
                    ? ""
                    : live.getImageHueList().stream().map(String::valueOf).collect(Collectors.joining(","));

            verifiedItems.add(OrderItemEntity.builder()
                    .id(UUID.randomUUID())
                    .cartItemId(reqItem.getCartItemId().trim())
                    .productId(productId)
                    .skuId(skuId)
                    .productVersion(liveVersion)
                    .productName(live.getName())
                    .brand(live.getBrand())
                    .categoryName(live.getCategoryName())
                    .variantLabel(variant.getLabel())
                    .variantColorHex(variant.getColorHex())
                    .imageUrl(imageUrl)
                    .imageEmoji(live.getEmoji())
                    .imageHue(imageHue)
                    .unitPriceMinor(livePriceMinor)
                    .quantity(reqItem.getQuantity())
                    .lineTotalMinor(lineTotalMinor)
                    .build());
        }

        if (!mismatches.isEmpty()) {
            throw new DomainException(
                    Status.Code.FAILED_PRECONDITION,
                    "PRODUCT_SNAPSHOT_MISMATCH",
                    "Thông tin sản phẩm trong đơn hàng vừa thay đổi",
                    mismatches);
        }

        long shippingFeeMinor = ShippingPolicy.computeShippingFeeMinor(subtotalMinor);
        long totalAmountMinor = ShippingPolicy.computeTotalMinor(subtotalMinor, shippingFeeMinor);
        String orderCode = generateOrderCode();

        OrderTransactionStepService.Tx1Outcome tx1 = txStepService.beginOrderTx1(
                userId,
                idempotencyKey,
                fingerprint,
                orderCode,
                paymentMethod,
                request.getShippingAddress(),
                request.getNote(),
                verifiedItems,
                subtotalMinor,
                shippingFeeMinor,
                totalAmountMinor);

        if (!tx1.claimed()) {
            return idempotencyService.checkExistingOrThrow(userId, idempotencyKey, fingerprint)
                    .orElseThrow(() -> new DomainException(
                            Status.Code.ABORTED,
                            "REQUEST_IN_PROGRESS",
                            "Concurrent order creation in progress"));
        }

        UUID orderId = tx1.orderId();
        Duration paymentTtl = properties.paymentTtl() != null ? properties.paymentTtl() : Duration.ofMinutes(15);
        Instant reservationExpiresAt = tx1.paymentExpiresAt() != null
                ? tx1.paymentExpiresAt()
                : Instant.now().plus(paymentTtl);

        List<ReservationLine> reservationLines = verifiedItems.stream()
                .map(i -> ReservationLine.newBuilder()
                        .setSkuId(i.getSkuId())
                        .setProductId(i.getProductId())
                        .setQuantity(i.getQuantity())
                        .build())
                .toList();

        try {
            inventoryAdapter.reserveStock(orderId, reservationLines, reservationExpiresAt);
        } catch (DomainException ex) {
            boolean businessInsufficient = "INVENTORY_INSUFFICIENT".equals(ex.reason());
            txStepService.markCancelledOnSagaFailure(
                    orderId,
                    userId,
                    idempotencyKey,
                    businessInsufficient ? "INVENTORY_INSUFFICIENT" : "INVENTORY_RESERVE_FAILED",
                    ex.reason(),
                    ex.getMessage(),
                    !businessInsufficient,
                    false);
            throw ex;
        } catch (RuntimeException ex) {
            txStepService.markCancelledOnSagaFailure(
                    orderId,
                    userId,
                    idempotencyKey,
                    "INVENTORY_RESERVE_FAILED",
                    "INVENTORY_UNAVAILABLE",
                    "Inventory service unavailable",
                    true,
                    false);
            throw new DomainException(Status.Code.UNAVAILABLE, "INVENTORY_UNAVAILABLE", "Inventory service unavailable");
        }

        if (paymentMethod == PaymentMethod.COD) {
            return txStepService.markPendingConfirmation(orderId, userId, idempotencyKey);
        }

        String payUrl;
        try {
            payUrl = paymentAdapter.createPaymentSession(
                    orderId,
                    orderCode,
                    userId,
                    totalAmountMinor,
                    reservationExpiresAt);
        } catch (DomainException ex) {
            txStepService.markCancelledOnSagaFailure(
                    orderId,
                    userId,
                    idempotencyKey,
                    "PAYMENT_SESSION_FAILED",
                    ex.reason(),
                    ex.getMessage(),
                    true,
                    true);
            throw ex;
        } catch (RuntimeException ex) {
            txStepService.markCancelledOnSagaFailure(
                    orderId,
                    userId,
                    idempotencyKey,
                    "PAYMENT_SESSION_FAILED",
                    "PAYMENT_GATEWAY_UNAVAILABLE",
                    "Payment service unavailable",
                    true,
                    true);
            throw new DomainException(Status.Code.UNAVAILABLE, "PAYMENT_GATEWAY_UNAVAILABLE", "Payment service unavailable");
        }

        return txStepService.markPendingPayment(orderId, userId, idempotencyKey, payUrl);
    }

    public OrderDetail cancelOrder(String rawUserId, String rawOrderId, String reason) {
        UUID userId = parseRequiredUuid(rawUserId, "user_id");
        UUID orderId = parseRequiredUuid(rawOrderId, "order_id");
        if (!authorizationAdapter.hasPermission(userId, "order.cancel")) {
            throw new DomainException(Status.Code.PERMISSION_DENIED, "ORDER_PERMISSION_DENIED", "Missing order.cancel permission");
        }
        return txStepService.cancelPendingPaymentOrder(userId, orderId, reason);
    }

    private static ProductVariant findVariant(GetProductResponse product, String skuId) {
        for (ProductVariant v : product.getVariantsList()) {
            if (skuId.equals(v.getSkuId())) {
                return v;
            }
        }
        return null;
    }

    private static UUID parseRequiredUuid(String raw, String field) {
        if (raw == null || raw.isBlank()) {
            throw new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", field + " is required");
        }
        try {
            return UUID.fromString(raw.trim());
        } catch (IllegalArgumentException ex) {
            throw new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", field + " must be a valid UUID");
        }
    }

    private static String requireNonBlank(String value, String field) {
        if (value == null || value.isBlank()) {
            throw new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", field + " is required");
        }
        return value.trim();
    }

    private static PaymentMethod parsePaymentMethod(String raw) {
        String normalized = requireNonBlank(raw, "payment_method").toUpperCase(Locale.ROOT);
        try {
            return PaymentMethod.valueOf(normalized);
        } catch (IllegalArgumentException ex) {
            throw new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", "payment_method must be COD or MOMO");
        }
    }

    private static void validateShippingAddress(ShippingAddressSnapshot addr) {
        if (addr == null) {
            throw new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", "shipping_address is required");
        }
        requireNonBlank(addr.getRecipientName(), "shipping_address.recipient_name");
        requireNonBlank(addr.getPhone(), "shipping_address.phone");
        requireNonBlank(addr.getProvinceCode(), "shipping_address.province_code");
        requireNonBlank(addr.getProvinceName(), "shipping_address.province_name");
        requireNonBlank(addr.getWardCode(), "shipping_address.ward_code");
        requireNonBlank(addr.getWardName(), "shipping_address.ward_name");
        requireNonBlank(addr.getAddressLine(), "shipping_address.address_line");
    }

    private static void validateOrderItems(List<OrderItem> items) {
        if (items == null || items.isEmpty()) {
            throw new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", "items must not be empty");
        }
        for (OrderItem item : items) {
            requireNonBlank(item.getProductId(), "items.product_id");
            requireNonBlank(item.getSkuId(), "items.sku_id");
            if (item.getQuantity() <= 0) {
                throw new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", "items.quantity must be positive");
            }
            if (!item.hasUnitPrice() || item.getUnitPrice().getAmountMinor() < 0) {
                throw new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", "items.unit_price is required");
            }
        }
    }

    private static String generateOrderCode() {
        StringBuilder suffix = new StringBuilder(6);
        for (int i = 0; i < 6; i++) {
            suffix.append(CODE_ALPHABET.charAt(RANDOM.nextInt(CODE_ALPHABET.length())));
        }
        return "ORD-" + DATE_FMT.format(Instant.now()) + "-" + suffix;
    }
}
