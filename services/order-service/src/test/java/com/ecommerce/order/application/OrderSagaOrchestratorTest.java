package com.ecommerce.order.application;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doAnswer;
import static org.mockito.Mockito.reset;
import static org.mockito.Mockito.when;

import com.ecommerce.contracts.catalog.v1.GetProductResponse;
import com.ecommerce.contracts.catalog.v1.ProductVariant;
import com.ecommerce.contracts.common.v1.Money;
import com.ecommerce.contracts.order.v1.CreateOrderRequest;
import com.ecommerce.contracts.order.v1.CreateOrderResponse;
import com.ecommerce.contracts.order.v1.OrderItem;
import com.ecommerce.contracts.order.v1.ShippingAddressSnapshot;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.integration.AuthorizationGrpcAdapter;
import com.ecommerce.order.integration.CatalogGrpcAdapter;
import com.ecommerce.order.integration.InventoryGrpcAdapter;
import com.ecommerce.order.integration.PaymentGrpcAdapter;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderRepository;
import com.ecommerce.order.support.PostgresRabbitIntegrationTestSupport;
import io.grpc.Status;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.transaction.support.TransactionSynchronizationManager;

class OrderSagaOrchestratorTest extends PostgresRabbitIntegrationTestSupport {

    @Autowired
    OrderSagaOrchestrator orchestrator;

    @Autowired
    OrderRepository orderRepository;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @MockitoBean
    AuthorizationGrpcAdapter authorizationAdapter;

    @MockitoBean
    CatalogGrpcAdapter catalogAdapter;

    @MockitoBean
    InventoryGrpcAdapter inventoryAdapter;

    @MockitoBean
    PaymentGrpcAdapter paymentAdapter;

    @BeforeEach
    void resetState() {
        jdbcTemplate.execute("TRUNCATE TABLE order_items, orders, idempotency_records, outbox_events, processed_events CASCADE");
        reset(authorizationAdapter, catalogAdapter, inventoryAdapter, paymentAdapter);
        when(authorizationAdapter.hasPermission(any(UUID.class), eq("order.create"))).thenReturn(true);
    }

    @Test
    void snapshotMismatchVariantsRejectBeforeTx1WithZeroDatabaseWrites() {
        UUID userId = UUID.randomUUID();
        when(catalogAdapter.getProduct("p-price")).thenReturn(Optional.of(sampleCatalogProduct("p-price", "p-price-v1", "Phone X", "Black", 220_000L, 2L, false)));

        CreateOrderRequest req = buildRequest(userId, "idem-snap-1", "COD", "p-price", "p-price-v1", "Phone X", "Black", 200_000L, 1L);

        assertThatThrownBy(() -> orchestrator.createOrder(req))
                .isInstanceOf(DomainException.class)
                .satisfies(ex -> {
                    DomainException de = (DomainException) ex;
                    assertThat(de.reason()).isEqualTo("PRODUCT_SNAPSHOT_MISMATCH");
                    assertThat(de.metadata()).containsKey("p-price-v1");
                    assertThat(de.metadata().get("p-price-v1")).contains("[PRICE_CHANGED]");
                });

        Integer orderCount = jdbcTemplate.queryForObject("SELECT COUNT(*) FROM orders", Integer.class);
        Integer idemCount = jdbcTemplate.queryForObject("SELECT COUNT(*) FROM idempotency_records", Integer.class);
        assertThat(orderCount).isZero();
        assertThat(idemCount).isZero();
    }

    @Test
    void codHappyPathConfirmsOrderComputesShippingAndNeverHoldsDbTransactionDuringGrpc() {
        UUID userId = UUID.randomUUID();
        when(catalogAdapter.getProduct("p-cod")).thenReturn(Optional.of(sampleCatalogProduct("p-cod", "p-cod-v1", "Headset", "White", 200_000L, 1L, false)));
        doAnswer(inv -> {
            assertThat(TransactionSynchronizationManager.isActualTransactionActive()).isFalse();
            return null;
        }).when(inventoryAdapter).reserveStock(any(UUID.class), any(), any());

        CreateOrderRequest req = buildRequest(userId, "idem-cod-1", "COD", "p-cod", "p-cod-v1", "Headset", "White", 200_000L, 1L);
        CreateOrderResponse resp = orchestrator.createOrder(req);

        assertThat(resp.getStatus()).isEqualTo("CONFIRMED");
        assertThat(resp.getOrder().getSubtotal().getAmountMinor()).isEqualTo(200_000L);
        assertThat(resp.getOrder().getShippingFee().getAmountMinor()).isEqualTo(25_000L);
        assertThat(resp.getOrder().getTotalAmount().getAmountMinor()).isEqualTo(225_000L);

        Integer outboxCount = jdbcTemplate.queryForObject(
                "SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND routing_key = 'order.confirmed'",
                Integer.class,
                resp.getOrderId());
        assertThat(outboxCount).isEqualTo(1);
    }

    @Test
    void inventoryInsufficientCancelsWithoutOutboxWhereasTimeoutEmitsReleaseCompensation() {
        UUID userId = UUID.randomUUID();
        when(catalogAdapter.getProduct("p-inv")).thenReturn(Optional.of(sampleCatalogProduct("p-inv", "p-inv-v1", "Item", "Std", 300_000L, 1L, false)));

        doAnswer(inv -> {
            throw new DomainException(Status.Code.FAILED_PRECONDITION, "INVENTORY_INSUFFICIENT", "Out of stock");
        }).when(inventoryAdapter).reserveStock(any(UUID.class), any(), any());

        assertThatThrownBy(() -> orchestrator.createOrder(
                buildRequest(userId, "idem-inv-insuff", "COD", "p-inv", "p-inv-v1", "Item", "Std", 300_000L, 1L)))
                .isInstanceOf(DomainException.class)
                .extracting(e -> ((DomainException) e).reason())
                .isEqualTo("INVENTORY_INSUFFICIENT");

        Integer compOutbox1 = jdbcTemplate.queryForObject("SELECT COUNT(*) FROM outbox_events", Integer.class);
        assertThat(compOutbox1).isZero();

        reset(inventoryAdapter);
        doAnswer(inv -> {
            throw new DomainException(Status.Code.UNAVAILABLE, "INVENTORY_UNAVAILABLE", "ReserveStock deadline exceeded");
        }).when(inventoryAdapter).reserveStock(any(UUID.class), any(), any());

        assertThatThrownBy(() -> orchestrator.createOrder(
                buildRequest(userId, "idem-inv-timeout", "COD", "p-inv", "p-inv-v1", "Item", "Std", 300_000L, 1L)))
                .isInstanceOf(DomainException.class)
                .extracting(e -> ((DomainException) e).reason())
                .isEqualTo("INVENTORY_UNAVAILABLE");

        Integer releaseCount = jdbcTemplate.queryForObject(
                "SELECT COUNT(*) FROM outbox_events WHERE routing_key = 'order.reservation.release_requested'",
                Integer.class);
        assertThat(releaseCount).isEqualTo(1);
    }

    @Test
    void momoSessionSuccessAndTimeoutAndMidFlightCancelGuard() {
        UUID userId = UUID.randomUUID();
        when(catalogAdapter.getProduct("p-momo")).thenReturn(Optional.of(sampleCatalogProduct("p-momo", "p-momo-v1", "Laptop", "Silver", 500_000L, 1L, false)));

        when(paymentAdapter.createPaymentSession(any(UUID.class), anyString(), eq(userId), eq(500_000L), any()))
                .thenReturn("https://momo.test/pay/ok-1");

        CreateOrderResponse momoOk = orchestrator.createOrder(
                buildRequest(userId, "idem-momo-ok", "MOMO", "p-momo", "p-momo-v1", "Laptop", "Silver", 500_000L, 1L));
        assertThat(momoOk.getStatus()).isEqualTo("PENDING_PAYMENT");
        assertThat(momoOk.getPayUrl()).isEqualTo("https://momo.test/pay/ok-1");

        // Timeout on CreatePaymentSession emits BOTH release stock and cancel payment
        reset(paymentAdapter);
        when(paymentAdapter.createPaymentSession(any(UUID.class), anyString(), eq(userId), anyLong(), any()))
                .thenThrow(new DomainException(Status.Code.UNAVAILABLE, "PAYMENT_GATEWAY_UNAVAILABLE", "MoMo timeout"));

        assertThatThrownBy(() -> orchestrator.createOrder(
                buildRequest(userId, "idem-momo-timeout", "MOMO", "p-momo", "p-momo-v1", "Laptop", "Silver", 500_000L, 1L)))
                .isInstanceOf(DomainException.class)
                .extracting(e -> ((DomainException) e).reason())
                .isEqualTo("PAYMENT_GATEWAY_UNAVAILABLE");

        List<String> routingKeys = jdbcTemplate.queryForList(
                "SELECT routing_key FROM outbox_events ORDER BY routing_key ASC",
                String.class);
        assertThat(routingKeys).contains("order.payment.cancel_requested", "order.reservation.release_requested");

        // Mid-flight cancel guard: if order becomes CANCELLED while CreatePaymentSession is in flight, payUrl is NOT saved or returned
        reset(paymentAdapter);
        when(paymentAdapter.createPaymentSession(any(UUID.class), anyString(), eq(userId), anyLong(), any()))
                .thenAnswer(inv -> {
                    UUID oid = inv.getArgument(0);
                    jdbcTemplate.update("UPDATE orders SET status = 'CANCELLED', cancel_reason = 'RECONCILE_STALE' WHERE id = ?", oid);
                    return "https://momo.test/pay/should-not-leak";
                });

        assertThatThrownBy(() -> orchestrator.createOrder(
                buildRequest(userId, "idem-momo-midcancel", "MOMO", "p-momo", "p-momo-v1", "Laptop", "Silver", 500_000L, 1L)))
                .isInstanceOf(DomainException.class);

        OrderEntity midOrder = orderRepository.findAll().stream()
                .filter(o -> "idem-momo-midcancel".equals(o.getIdempotencyKey()))
                .findFirst()
                .orElseThrow();
        assertThat(midOrder.getStatus()).isEqualTo(OrderStatus.CANCELLED);
        assertThat(midOrder.getPayUrl()).isEmpty();
        assertThat(midOrder.getPaymentStatus()).isNotEqualTo(PaymentStatus.PAID);
    }

    private static GetProductResponse sampleCatalogProduct(
            String productId,
            String skuId,
            String name,
            String variantLabel,
            long priceMinor,
            long version,
            boolean deleted) {
        return GetProductResponse.newBuilder()
                .setId(productId)
                .setName(name)
                .setBrand("Brand")
                .setCategoryName("Category")
                .setDeleted(deleted)
                .setVersion(version)
                .setPrice(Money.newBuilder().setCurrency("VND").setAmountMinor(priceMinor).build())
                .addVariants(ProductVariant.newBuilder()
                        .setSkuId(skuId)
                        .setLabel(variantLabel)
                        .setPrice(Money.newBuilder().setCurrency("VND").setAmountMinor(priceMinor).build())
                        .build())
                .build();
    }

    private static CreateOrderRequest buildRequest(
            UUID userId,
            String idemKey,
            String method,
            String productId,
            String skuId,
            String expectedName,
            String variantLabel,
            long unitPriceMinor,
            long expectedVersion) {
        return CreateOrderRequest.newBuilder()
                .setUserId(userId.toString())
                .setIdempotencyKey(idemKey)
                .setPaymentMethod(method)
                .setShippingAddress(ShippingAddressSnapshot.newBuilder()
                        .setRecipientName("Tran Thi B")
                        .setPhone("0912345678")
                        .setProvinceCode("01")
                        .setProvinceName("Ha Noi")
                        .setWardCode("00001")
                        .setWardName("Phuong Phuc Xa")
                        .setAddressLine("10 Nguyen Trai")
                        .build())
                .addItems(OrderItem.newBuilder()
                        .setCartItemId("ci-" + skuId)
                        .setProductId(productId)
                        .setSkuId(skuId)
                        .setQuantity(1)
                        .setExpectedName(expectedName)
                        .setVariantLabel(variantLabel)
                        .setExpectedProductVersion(expectedVersion)
                        .setUnitPrice(Money.newBuilder().setCurrency("VND").setAmountMinor(unitPriceMinor).build())
                        .build())
                .build();
    }
}
