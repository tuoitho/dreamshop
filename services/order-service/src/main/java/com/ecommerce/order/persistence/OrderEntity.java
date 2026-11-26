package com.ecommerce.order.persistence;

import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.PaymentStatus;
import jakarta.persistence.CascadeType;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.FetchType;
import jakarta.persistence.Id;
import jakarta.persistence.OneToMany;
import jakarta.persistence.OrderBy;
import jakarta.persistence.Table;
import jakarta.persistence.Version;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Entity
@Table(name = "orders")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class OrderEntity {

    @Id
    @Column(name = "id", nullable = false)
    private UUID id;

    @Column(name = "order_code", nullable = false, length = 32, unique = true)
    private String orderCode;

    @Column(name = "user_id", nullable = false)
    private UUID userId;

    @Column(name = "idempotency_key", nullable = false, length = 128)
    private String idempotencyKey;

    @Enumerated(EnumType.STRING)
    @Column(name = "status", nullable = false, length = 32)
    private OrderStatus status;

    @Enumerated(EnumType.STRING)
    @Column(name = "payment_method", nullable = false, length = 16)
    private PaymentMethod paymentMethod;

    @Enumerated(EnumType.STRING)
    @Column(name = "payment_status", nullable = false, length = 32)
    private PaymentStatus paymentStatus;

    @Column(name = "recipient_name", nullable = false, length = 255)
    private String recipientName;

    @Column(name = "recipient_phone", nullable = false, length = 32)
    private String recipientPhone;

    @Column(name = "province_code", nullable = false, length = 32)
    private String provinceCode;

    @Column(name = "province_name", nullable = false, length = 128)
    private String provinceName;

    @Column(name = "ward_code", nullable = false, length = 32)
    private String wardCode;

    @Column(name = "ward_name", nullable = false, length = 128)
    private String wardName;

    @Column(name = "address_line", nullable = false, length = 512)
    private String addressLine;

    @Column(name = "address_label", nullable = false, length = 64)
    @Builder.Default
    private String addressLabel = "";

    @Column(name = "currency", nullable = false, length = 8)
    @Builder.Default
    private String currency = "VND";

    @Column(name = "subtotal_minor", nullable = false)
    private long subtotalMinor;

    @Column(name = "shipping_fee_minor", nullable = false)
    private long shippingFeeMinor;

    @Column(name = "total_amount_minor", nullable = false)
    private long totalAmountMinor;

    @Column(name = "pay_url", nullable = false, columnDefinition = "text")
    @Builder.Default
    private String payUrl = "";

    @Column(name = "payment_trans_id", nullable = false, length = 64)
    @Builder.Default
    private String paymentTransId = "";

    @Column(name = "payment_expires_at")
    private Instant paymentExpiresAt;

    @Column(name = "note", nullable = false, columnDefinition = "text")
    @Builder.Default
    private String note = "";

    @Column(name = "cancel_reason", nullable = false, length = 128)
    @Builder.Default
    private String cancelReason = "";

    @Column(name = "auto_confirm_at")
    private Instant autoConfirmAt;

    @Column(name = "shipping_carrier", nullable = false, length = 100)
    @Builder.Default
    private String shippingCarrier = "";

    @Column(name = "tracking_number", nullable = false, length = 100)
    @Builder.Default
    private String trackingNumber = "";

    @Column(name = "refund_trans_id", nullable = false, length = 64)
    @Builder.Default
    private String refundTransId = "";

    @Column(name = "refund_last_error", nullable = false, length = 255)
    @Builder.Default
    private String refundLastError = "";

    @Column(name = "refunded_at")
    private Instant refundedAt;

    @Column(name = "refund_actor_type", nullable = false, length = 16)
    @Builder.Default
    private String refundActorType = "";

    @Column(name = "refund_actor_id")
    private UUID refundActorId;

    @Column(name = "refund_reason", nullable = false, length = 128)
    @Builder.Default
    private String refundReason = "";

    @Version
    @Column(name = "version", nullable = false)
    private long version;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt;

    @Column(name = "updated_at", nullable = false)
    private Instant updatedAt;

    @OneToMany(mappedBy = "order", cascade = CascadeType.ALL, orphanRemoval = true, fetch = FetchType.EAGER)
    @OrderBy("skuId ASC")
    @Builder.Default
    private List<OrderItemEntity> items = new ArrayList<>();

    public void addItem(OrderItemEntity item) {
        if (items == null) {
            items = new ArrayList<>();
        }
        item.setOrder(this);
        items.add(item);
    }
}
