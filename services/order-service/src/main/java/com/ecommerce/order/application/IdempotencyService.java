package com.ecommerce.order.application;

import com.ecommerce.contracts.order.v1.CreateOrderRequest;
import com.ecommerce.contracts.order.v1.CreateOrderResponse;
import com.ecommerce.contracts.order.v1.OrderItem;
import com.ecommerce.contracts.order.v1.ShippingAddressSnapshot;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.persistence.IdempotencyRecordEntity;
import com.ecommerce.order.persistence.IdempotencyRecordId;
import com.ecommerce.order.persistence.IdempotencyRecordRepository;
import com.google.protobuf.InvalidProtocolBufferException;
import com.google.protobuf.util.JsonFormat;
import io.grpc.Status;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Instant;
import java.util.Comparator;
import java.util.HexFormat;
import java.util.List;
import java.util.Locale;
import java.util.Optional;
import java.util.UUID;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class IdempotencyService {
    public static final String STATUS_IN_PROGRESS = "IN_PROGRESS";
    public static final String STATUS_COMPLETED = "COMPLETED";
    public static final String STATUS_FAILED_TERMINAL = "FAILED_TERMINAL";

    private final IdempotencyRecordRepository repository;

    public IdempotencyService(IdempotencyRecordRepository repository) {
        this.repository = repository;
    }

    public String computeFingerprint(CreateOrderRequest req) {
        ShippingAddressSnapshot addr = req.getShippingAddress();
        StringBuilder sb = new StringBuilder(512);
        sb.append(req.getPaymentMethod().trim().toUpperCase(Locale.ROOT)).append('|')
                .append(addr.getRecipientName().trim()).append('|')
                .append(addr.getPhone().trim()).append('|')
                .append(addr.getProvinceCode().trim()).append('|')
                .append(addr.getWardCode().trim()).append('|')
                .append(addr.getAddressLine().trim()).append('|')
                .append(req.getNote().trim()).append('|');

        List<OrderItem> sortedItems = req.getItemsList().stream()
                .sorted(Comparator.comparing(OrderItem::getSkuId)
                        .thenComparing(OrderItem::getProductId)
                        .thenComparingInt(OrderItem::getQuantity))
                .toList();

        for (OrderItem item : sortedItems) {
            long unitPrice = item.hasUnitPrice() ? item.getUnitPrice().getAmountMinor() : 0L;
            sb.append(item.getProductId().trim()).append(':')
                    .append(item.getSkuId().trim()).append(':')
                    .append(item.getQuantity()).append(':')
                    .append(unitPrice).append(':')
                    .append(item.getExpectedProductVersion()).append(';');
        }

        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            return HexFormat.of().formatHex(digest.digest(sb.toString().getBytes(StandardCharsets.UTF_8)));
        } catch (NoSuchAlgorithmException ex) {
            throw new IllegalStateException("SHA-256 unavailable", ex);
        }
    }

    @Transactional(readOnly = true)
    public Optional<CreateOrderResponse> checkExistingOrThrow(UUID userId, String key, String fingerprint) {
        Optional<IdempotencyRecordEntity> found = repository.findById(new IdempotencyRecordId(userId, key));
        if (found.isEmpty()) {
            return Optional.empty();
        }
        IdempotencyRecordEntity record = found.get();
        boolean expired = record.getExpiresAt() != null && !record.getExpiresAt().isAfter(Instant.now());
        if (expired && record.getOrderId() == null) {
            return Optional.empty();
        }
        if (!record.getRequestFingerprint().equals(fingerprint)) {
            throw new DomainException(
                    Status.Code.ALREADY_EXISTS,
                    "IDEMPOTENCY_KEY_REUSED",
                    "Idempotency-Key was already used with a different order payload");
        }

        return switch (record.getStatus()) {
            case STATUS_IN_PROGRESS -> throw new DomainException(
                    Status.Code.ABORTED,
                    "REQUEST_IN_PROGRESS",
                    "Order creation with this Idempotency-Key is currently in progress");
            case STATUS_COMPLETED -> Optional.of(deserializeResponse(record.getResponseJson()));
            case STATUS_FAILED_TERMINAL -> {
                String reason = record.getFailureCode() == null || record.getFailureCode().isBlank()
                        ? "ORDER_CREATION_FAILED"
                        : record.getFailureCode();
                String msg = record.getFailureMessage() == null || record.getFailureMessage().isBlank()
                        ? "Previous order attempt failed"
                        : record.getFailureMessage();
                Status.Code code = mapReasonToStatusCode(reason);
                throw new DomainException(code, reason, msg);
            }
            default -> Optional.empty();
        };
    }

    public String serializeResponse(CreateOrderResponse response) {
        try {
            return JsonFormat.printer().omittingInsignificantWhitespace().print(response);
        } catch (InvalidProtocolBufferException ex) {
            throw new IllegalStateException("Failed to serialize CreateOrderResponse", ex);
        }
    }

    public CreateOrderResponse deserializeResponse(String json) {
        try {
            CreateOrderResponse.Builder builder = CreateOrderResponse.newBuilder();
            if (json != null && !json.isBlank()) {
                JsonFormat.parser().ignoringUnknownFields().merge(json, builder);
            }
            return builder.build();
        } catch (InvalidProtocolBufferException ex) {
            throw new IllegalStateException("Failed to deserialize CreateOrderResponse", ex);
        }
    }

    private static Status.Code mapReasonToStatusCode(String reason) {
        return switch (reason) {
            case "INVENTORY_INSUFFICIENT", "PRODUCT_SNAPSHOT_MISMATCH", "PAYMENT_GATEWAY_REJECTED",
                    "PAYMENT_ALREADY_CANCELLED", "ORDER_ALREADY_CANCELLED" -> Status.Code.FAILED_PRECONDITION;
            default -> Status.Code.UNAVAILABLE;
        };
    }
}
