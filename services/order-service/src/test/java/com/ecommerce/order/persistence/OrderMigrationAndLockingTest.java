package com.ecommerce.order.persistence;

import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.support.PostgresRabbitIntegrationTestSupport;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.support.TransactionTemplate;

class OrderMigrationAndLockingTest extends PostgresRabbitIntegrationTestSupport {

    @Autowired
    JdbcTemplate jdbcTemplate;

    @Autowired
    OrderRepository orderRepository;

    @Autowired
    TransactionTemplate transactionTemplate;

    @BeforeEach
    void cleanTables() {
        jdbcTemplate.execute("TRUNCATE TABLE order_items, orders, idempotency_records, outbox_events, processed_events CASCADE");
    }

    @Test
    void createsFlywaySchemaAndSupportsPessimisticForUpdateLocking() {
        List<String> tables = jdbcTemplate.queryForList(
                "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'",
                String.class);
        assertThat(tables).contains("orders", "order_items", "idempotency_records", "outbox_events", "processed_events");

        UUID orderId = UUID.randomUUID();
        UUID userId = UUID.randomUUID();

        transactionTemplate.executeWithoutResult(status -> {
            OrderEntity order = OrderEntity.builder()
                    .id(orderId)
                    .orderCode("ORD-20261105-LOCK01")
                    .userId(userId)
                    .idempotencyKey("idem-lock-1")
                    .status(OrderStatus.PENDING_STOCK)
                    .paymentMethod(PaymentMethod.COD)
                    .paymentStatus(PaymentStatus.UNPAID)
                    .recipientName("Nguyen Van A")
                    .recipientPhone("0901234567")
                    .provinceCode("79")
                    .provinceName("TP. Ho Chi Minh")
                    .wardCode("26734")
                    .wardName("Phuong Ben Nghe")
                    .addressLine("1 Le Duan")
                    .addressLabel("Nha rieng")
                    .currency("VND")
                    .subtotalMinor(350_000L)
                    .shippingFeeMinor(0L)
                    .totalAmountMinor(350_000L)
                    .payUrl("")
                    .paymentTransId("")
                    .note("")
                    .cancelReason("")
                    .createdAt(Instant.now())
                    .updatedAt(Instant.now())
                    .build();
            order.addItem(OrderItemEntity.builder()
                    .id(UUID.randomUUID())
                    .cartItemId("ci-1")
                    .productId("prod-1")
                    .skuId("prod-1-v1")
                    .productVersion(3L)
                    .productName("Ao thun")
                    .brand("BrandX")
                    .categoryName("Thoi trang")
                    .variantLabel("Den / L")
                    .variantColorHex("#000000")
                    .imageUrl("https://cdn.example/1.jpg")
                    .imageEmoji("")
                    .imageHue("")
                    .unitPriceMinor(350_000L)
                    .quantity(1)
                    .lineTotalMinor(350_000L)
                    .build());
            orderRepository.saveAndFlush(order);
        });

        OrderEntity locked = transactionTemplate.execute(status -> {
            OrderEntity found = orderRepository.findByIdForUpdate(orderId).orElseThrow();
            found.setStatus(OrderStatus.CONFIRMED);
            found.setUpdatedAt(Instant.now());
            return orderRepository.saveAndFlush(found);
        });

        assertThat(locked).isNotNull();
        assertThat(locked.getStatus()).isEqualTo(OrderStatus.CONFIRMED);
        assertThat(locked.getVersion()).isGreaterThanOrEqualTo(1L);
    }
}
