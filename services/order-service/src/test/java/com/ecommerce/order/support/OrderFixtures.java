package com.ecommerce.order.support;

import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderItemEntity;
import com.ecommerce.order.persistence.OrderRepository;
import java.time.Instant;
import java.util.UUID;

public final class OrderFixtures {

    private OrderFixtures() {
    }

    public static UUID seedOrder(
            OrderRepository orderRepository,
            UUID userId,
            String code,
            OrderStatus status,
            PaymentMethod method,
            PaymentStatus paymentStatus) {
        UUID orderId = UUID.randomUUID();
        OrderEntity entity = OrderEntity.builder()
                .id(orderId)
                .orderCode(code)
                .userId(userId)
                .idempotencyKey("idem-" + code)
                .status(status)
                .paymentMethod(method)
                .paymentStatus(paymentStatus)
                .recipientName("Le Van C")
                .recipientPhone("0909998888")
                .provinceCode("79")
                .provinceName("HCM")
                .wardCode("26734")
                .wardName("Ben Nghe")
                .addressLine("2 Hai Trieu")
                .subtotalMinor(225_000L)
                .shippingFeeMinor(25_000L)
                .totalAmountMinor(250_000L)
                .payUrl(method == PaymentMethod.MOMO && status == OrderStatus.PENDING_PAYMENT ? "https://momo.test/pay/" + code : "")
                .paymentExpiresAt(method == PaymentMethod.MOMO ? Instant.now().plusSeconds(900) : null)
                .createdAt(Instant.now())
                .updatedAt(Instant.now())
                .build();
        entity.addItem(OrderItemEntity.builder()
                .id(UUID.randomUUID())
                .cartItemId("ci-1")
                .productId("p-1")
                .skuId("p-1-v1")
                .productVersion(1L)
                .productName("Product")
                .brand("Brand")
                .categoryName("Cat")
                .variantLabel("Std")
                .variantColorHex("")
                .imageUrl("")
                .imageEmoji("")
                .imageHue("")
                .unitPriceMinor(225_000L)
                .quantity(1)
                .lineTotalMinor(225_000L)
                .build());
        orderRepository.saveAndFlush(entity);
        return orderId;
    }
}
