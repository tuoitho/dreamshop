package com.ecommerce.order.application;

import static com.ecommerce.order.support.StatusHistoryQueries.historyOf;
import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderItemEntity;
import com.ecommerce.order.persistence.OrderRepository;
import com.ecommerce.order.support.PostgresRabbitIntegrationTestSupport;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;

class CodAutoConfirmSchedulerTest extends PostgresRabbitIntegrationTestSupport {

    @Autowired
    CodAutoConfirmScheduler scheduler;

    @Autowired
    OrderRepository orderRepository;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void cleanTables() {
        jdbcTemplate.execute("TRUNCATE TABLE order_items, orders, idempotency_records, outbox_events, processed_events CASCADE");
    }

    @Test
    void confirmsDueCodOrderAndPublishesOrderConfirmed() {
        UUID orderId = seedCodOrder("ORD-20261112-AUTO01", OrderStatus.PENDING_CONFIRMATION, Instant.now().minusSeconds(1));

        scheduler.confirmDueOrders();

        OrderEntity order = orderRepository.findById(orderId).orElseThrow();
        assertThat(order.getStatus()).isEqualTo(OrderStatus.CONFIRMED);
        assertThat(order.getPaymentStatus()).isEqualTo(PaymentStatus.UNPAID);
        assertThat(outboxRoutingKeys(orderId)).containsExactly("order.confirmed");
        assertThat(historyOf(jdbcTemplate, orderId)).containsExactly("PENDING_CONFIRMATION>CONFIRMED:SYSTEM");
    }

    @Test
    void leavesOrdersThatAreNotDueUntouched() {
        UUID orderId = seedCodOrder("ORD-20261112-AUTO02", OrderStatus.PENDING_CONFIRMATION, Instant.now().plusSeconds(60));

        scheduler.confirmDueOrders();

        assertThat(orderRepository.findById(orderId).orElseThrow().getStatus()).isEqualTo(OrderStatus.PENDING_CONFIRMATION);
        assertThat(outboxRoutingKeys(orderId)).isEmpty();
        assertThat(historyOf(jdbcTemplate, orderId)).isEmpty();
    }

    @Test
    void leavesCancelledOrdersUntouchedEvenWhenDue() {
        UUID orderId = seedCodOrder("ORD-20261112-AUTO03", OrderStatus.CANCELLED, Instant.now().minusSeconds(1));

        scheduler.confirmDueOrders();

        assertThat(orderRepository.findById(orderId).orElseThrow().getStatus()).isEqualTo(OrderStatus.CANCELLED);
        assertThat(outboxRoutingKeys(orderId)).isEmpty();
    }

    private List<String> outboxRoutingKeys(UUID orderId) {
        return jdbcTemplate.queryForList(
                "SELECT routing_key FROM outbox_events WHERE aggregate_id = ?",
                String.class,
                orderId.toString());
    }

    private UUID seedCodOrder(String code, OrderStatus status, Instant autoConfirmAt) {
        UUID orderId = UUID.randomUUID();
        OrderEntity entity = OrderEntity.builder()
                .id(orderId)
                .orderCode(code)
                .userId(UUID.randomUUID())
                .idempotencyKey("idem-" + code)
                .status(status)
                .paymentMethod(PaymentMethod.COD)
                .paymentStatus(PaymentStatus.UNPAID)
                .recipientName("Tran Thi B")
                .recipientPhone("0908887777")
                .provinceCode("79")
                .provinceName("HCM")
                .wardCode("26734")
                .wardName("Ben Nghe")
                .addressLine("3 Nguyen Hue")
                .subtotalMinor(100_000L)
                .shippingFeeMinor(25_000L)
                .totalAmountMinor(125_000L)
                .autoConfirmAt(autoConfirmAt)
                .createdAt(Instant.now().minusSeconds(120))
                .updatedAt(Instant.now().minusSeconds(120))
                .build();
        entity.addItem(OrderItemEntity.builder()
                .id(UUID.randomUUID())
                .cartItemId("ci-auto")
                .productId("p-auto")
                .skuId("p-auto-v1")
                .productVersion(1L)
                .productName("Product")
                .brand("Brand")
                .categoryName("Cat")
                .variantLabel("Std")
                .variantColorHex("")
                .imageUrl("")
                .imageEmoji("")
                .imageHue("")
                .unitPriceMinor(100_000L)
                .quantity(1)
                .lineTotalMinor(100_000L)
                .build());
        orderRepository.saveAndFlush(entity);
        return orderId;
    }
}
