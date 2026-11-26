package com.ecommerce.order.persistence;

import com.ecommerce.order.domain.ActorType;
import com.ecommerce.order.domain.OrderStatus;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import java.time.Instant;
import java.util.UUID;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Entity
@Table(name = "order_status_history")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class OrderStatusHistoryEntity {

    @Id
    @Column(name = "id", nullable = false)
    private UUID id;

    @Column(name = "order_id", nullable = false)
    private UUID orderId;

    @Enumerated(EnumType.STRING)
    @Column(name = "from_status", length = 32)
    private OrderStatus fromStatus;

    @Enumerated(EnumType.STRING)
    @Column(name = "to_status", nullable = false, length = 32)
    private OrderStatus toStatus;

    @Enumerated(EnumType.STRING)
    @Column(name = "actor_type", nullable = false, length = 16)
    private ActorType actorType;

    @Column(name = "actor_id")
    private UUID actorId;

    @Column(name = "reason", nullable = false, length = 128)
    @Builder.Default
    private String reason = "";

    @Column(name = "shipping_carrier", nullable = false, length = 100)
    @Builder.Default
    private String shippingCarrier = "";

    @Column(name = "tracking_number", nullable = false, length = 100)
    @Builder.Default
    private String trackingNumber = "";

    @Column(name = "created_at", nullable = false)
    private Instant createdAt;
}
