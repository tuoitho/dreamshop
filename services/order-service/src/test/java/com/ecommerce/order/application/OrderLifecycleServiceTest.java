package com.ecommerce.order.application;

import static com.ecommerce.order.support.OrderFixtures.seedOrder;
import static com.ecommerce.order.support.StatusHistoryQueries.historyOf;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.ecommerce.contracts.order.v1.OrderDetail;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderRepository;
import com.ecommerce.order.support.PostgresRabbitIntegrationTestSupport;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;

class OrderLifecycleServiceTest extends PostgresRabbitIntegrationTestSupport {

    @Autowired
    OrderLifecycleService lifecycleService;

    @Autowired
    OrderRepository orderRepository;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void cleanTables() {
        jdbcTemplate.execute("TRUNCATE TABLE order_items, orders, idempotency_records, outbox_events, processed_events CASCADE");
    }

    @Test
    void cancellingPendingConfirmationReleasesReservation() {
        UUID userId = UUID.randomUUID();
        UUID orderId = seedOrder(orderRepository, userId, "ORD-20261112-CAN01",
                OrderStatus.PENDING_CONFIRMATION, PaymentMethod.COD, PaymentStatus.UNPAID);

        OrderDetail detail = lifecycleService.cancelByCustomer(userId, orderId, " Doi y ");

        assertThat(detail.getStatus()).isEqualTo("CANCELLED");
        assertThat(detail.getCancelReason()).isEqualTo("Doi y");
        assertThat(detail.getStatusHistoryList()).hasSize(1);
        assertThat(routingKeys(orderId)).containsExactly("order.reservation.release_requested");
        assertThat(orderRepository.findById(orderId).orElseThrow().getPaymentStatus()).isEqualTo(PaymentStatus.UNPAID);
        assertThat(historyOf(jdbcTemplate, orderId)).containsExactly("PENDING_CONFIRMATION>CANCELLED:CUSTOMER");
    }

    @Test
    void cancellingConfirmedCodOrderRestocks() {
        UUID userId = UUID.randomUUID();
        UUID orderId = seedOrder(orderRepository, userId, "ORD-20261112-CAN02",
                OrderStatus.CONFIRMED, PaymentMethod.COD, PaymentStatus.UNPAID);

        lifecycleService.cancelByCustomer(userId, orderId, "");

        assertThat(routingKeys(orderId)).containsExactly("order.stock.restock_requested");
        assertThat(orderRepository.findById(orderId).orElseThrow().getCancelReason()).isEqualTo("USER_CANCELLED");
    }

    @Test
    void cancellingPaidMomoOrderRestocksAndRequestsRefund() {
        UUID userId = UUID.randomUUID();
        UUID orderId = seedOrder(orderRepository, userId, "ORD-20261112-CAN03",
                OrderStatus.CONFIRMED, PaymentMethod.MOMO, PaymentStatus.PAID);

        lifecycleService.cancelByCustomer(userId, orderId, "Doi y");

        assertThat(routingKeys(orderId)).containsExactly("order.payment.refund_requested", "order.stock.restock_requested");
        assertThat(orderRepository.findById(orderId).orElseThrow().getPaymentStatus()).isEqualTo(PaymentStatus.REFUND_REQUIRED);
    }

    @Test
    void cancellingPendingPaymentReleasesStockAndCancelsPayment() {
        UUID userId = UUID.randomUUID();
        UUID orderId = seedOrder(orderRepository, userId, "ORD-20261112-CAN04",
                OrderStatus.PENDING_PAYMENT, PaymentMethod.MOMO, PaymentStatus.PENDING);

        lifecycleService.cancelByCustomer(userId, orderId, "Doi y");

        OrderEntity order = orderRepository.findById(orderId).orElseThrow();
        assertThat(order.getPayUrl()).isEmpty();
        assertThat(routingKeys(orderId)).containsExactly("order.payment.cancel_requested", "order.reservation.release_requested");
    }

    @Test
    void cancellingAnotherUsersOrderIsNotFound() {
        UUID orderId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261112-CAN05",
                OrderStatus.CONFIRMED, PaymentMethod.COD, PaymentStatus.UNPAID);

        assertThatThrownBy(() -> lifecycleService.cancelByCustomer(UUID.randomUUID(), orderId, "x"))
                .isInstanceOf(DomainException.class)
                .extracting(e -> ((DomainException) e).reason())
                .isEqualTo("ORDER_NOT_FOUND");
    }

    @Test
    void cancellingAlreadyCancelledOrderIsIdempotent() {
        UUID userId = UUID.randomUUID();
        UUID orderId = seedOrder(orderRepository, userId, "ORD-20261112-CAN06",
                OrderStatus.CANCELLED, PaymentMethod.COD, PaymentStatus.UNPAID);

        OrderDetail detail = lifecycleService.cancelByCustomer(userId, orderId, "x");

        assertThat(detail.getStatus()).isEqualTo("CANCELLED");
        assertThat(routingKeys(orderId)).isEmpty();
    }

    @Test
    void cancellingShippingOrderIsRejected() {
        UUID userId = UUID.randomUUID();
        UUID orderId = seedOrder(orderRepository, userId, "ORD-20261112-CAN07",
                OrderStatus.SHIPPING, PaymentMethod.COD, PaymentStatus.UNPAID);

        assertThatThrownBy(() -> lifecycleService.cancelByCustomer(userId, orderId, "x"))
                .isInstanceOf(DomainException.class)
                .extracting(e -> ((DomainException) e).reason())
                .isEqualTo("ORDER_NOT_CANCELLABLE");
        assertThat(routingKeys(orderId)).isEmpty();
    }

    @Test
    void reasonLongerThan128CharactersIsRejected() {
        UUID userId = UUID.randomUUID();
        UUID orderId = seedOrder(orderRepository, userId, "ORD-20261112-CAN08",
                OrderStatus.CONFIRMED, PaymentMethod.COD, PaymentStatus.UNPAID);

        assertThatThrownBy(() -> lifecycleService.cancelByCustomer(userId, orderId, "x".repeat(129)))
                .isInstanceOf(DomainException.class)
                .extracting(e -> ((DomainException) e).reason())
                .isEqualTo("INVALID_PARAMETER");
    }

    private List<String> routingKeys(UUID orderId) {
        return jdbcTemplate.queryForList(
                "SELECT routing_key FROM outbox_events WHERE aggregate_id = ? ORDER BY routing_key ASC",
                String.class,
                orderId.toString());
    }
}
