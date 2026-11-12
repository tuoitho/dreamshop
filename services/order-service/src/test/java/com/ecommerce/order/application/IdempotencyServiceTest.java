package com.ecommerce.order.application;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.ecommerce.contracts.common.v1.Money;
import com.ecommerce.contracts.order.v1.CreateOrderRequest;
import com.ecommerce.contracts.order.v1.CreateOrderResponse;
import com.ecommerce.contracts.order.v1.OrderDetail;
import com.ecommerce.contracts.order.v1.OrderItem;
import com.ecommerce.contracts.order.v1.ShippingAddressSnapshot;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.persistence.OrderItemEntity;
import com.ecommerce.order.support.PostgresRabbitIntegrationTestSupport;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;

class IdempotencyServiceTest extends PostgresRabbitIntegrationTestSupport {

    @Autowired
    IdempotencyService idempotencyService;

    @Autowired
    OrderTransactionStepService txStepService;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void cleanTables() {
        jdbcTemplate.execute("TRUNCATE TABLE order_items, orders, idempotency_records, outbox_events, processed_events CASCADE");
    }

    @Test
    void concurrentTx1ClaimsProduceExactlyOneWinnerWithoutUniqueViolation() throws Exception {
        UUID userId = UUID.randomUUID();
        String key = "idem-concurrent-1";
        CreateOrderRequest req = sampleRequest(userId.toString(), key, 2);
        String fingerprint = idempotencyService.computeFingerprint(req);

        int threads = 12;
        ExecutorService pool = Executors.newFixedThreadPool(threads);
        CountDownLatch ready = new CountDownLatch(threads);
        CountDownLatch start = new CountDownLatch(1);
        List<Future<OrderTransactionStepService.Tx1Outcome>> futures = new ArrayList<>();

        for (int i = 0; i < threads; i++) {
            final int idx = i;
            futures.add(pool.submit(() -> {
                ready.countDown();
                start.await();
                OrderItemEntity item = OrderItemEntity.builder()
                        .id(UUID.randomUUID())
                        .cartItemId("ci-1")
                        .productId("p-1")
                        .skuId("p-1-v1")
                        .productVersion(1L)
                        .productName("Product 1")
                        .brand("Brand")
                        .categoryName("Cat")
                        .variantLabel("Default")
                        .variantColorHex("")
                        .imageUrl("")
                        .imageEmoji("")
                        .imageHue("")
                        .unitPriceMinor(150_000L)
                        .quantity(2)
                        .lineTotalMinor(300_000L)
                        .build();
                return txStepService.beginOrderTx1(
                        userId,
                        key,
                        fingerprint,
                        "ORD-20261105-IDEM" + String.format("%02d", idx),
                        PaymentMethod.COD,
                        req.getShippingAddress(),
                        req.getNote(),
                        List.of(item),
                        300_000L,
                        0L,
                        300_000L);
            }));
        }

        ready.await();
        start.countDown();

        int winners = 0;
        int losers = 0;
        UUID winningOrderId = null;
        for (Future<OrderTransactionStepService.Tx1Outcome> f : futures) {
            OrderTransactionStepService.Tx1Outcome out = f.get();
            if (out.claimed()) {
                winners++;
                winningOrderId = out.orderId();
            } else {
                losers++;
            }
        }
        pool.shutdownNow();

        assertThat(winners).isEqualTo(1);
        assertThat(losers).isEqualTo(threads - 1);
        assertThat(winningOrderId).isNotNull();

        // While IN_PROGRESS, same fingerprint yields REQUEST_IN_PROGRESS; mutated payload yields IDEMPOTENCY_KEY_REUSED
        assertThatThrownBy(() -> idempotencyService.checkExistingOrThrow(userId, key, fingerprint))
                .isInstanceOf(DomainException.class)
                .extracting(ex -> ((DomainException) ex).reason())
                .isEqualTo("REQUEST_IN_PROGRESS");

        String mutatedFp = idempotencyService.computeFingerprint(sampleRequest(userId.toString(), key, 5));
        assertThatThrownBy(() -> idempotencyService.checkExistingOrThrow(userId, key, mutatedFp))
                .isInstanceOf(DomainException.class)
                .extracting(ex -> ((DomainException) ex).reason())
                .isEqualTo("IDEMPOTENCY_KEY_REUSED");

        // After completing COD order, replay returns exact CreateOrderResponse
        CreateOrderResponse confirmedResp = txStepService.markPendingConfirmation(winningOrderId, userId, key);
        CreateOrderResponse replayed = idempotencyService.checkExistingOrThrow(userId, key, fingerprint).orElseThrow();
        assertThat(replayed.getOrderId()).isEqualTo(confirmedResp.getOrderId());
        assertThat(replayed.getStatus()).isEqualTo("PENDING_CONFIRMATION");
        assertThat(OrderDetail.getDefaultInstance()).isNotEqualTo(replayed.getOrder());

        // Even after the 24h TTL window elapses, a key tied to a persisted order replays deterministically
        // instead of deleting idempotency_records and hitting 23505 on orders.uq_orders_user_idempotency.
        jdbcTemplate.update(
                "UPDATE idempotency_records SET expires_at = now() - interval '1 hour' WHERE user_id = ? AND idempotency_key = ?",
                userId,
                key);
        CreateOrderResponse postTtlReplay = idempotencyService.checkExistingOrThrow(userId, key, fingerprint).orElseThrow();
        assertThat(postTtlReplay.getOrderId()).isEqualTo(confirmedResp.getOrderId());

        OrderTransactionStepService.Tx1Outcome postTtlClaim = txStepService.beginOrderTx1(
                userId,
                key,
                fingerprint,
                "ORD-20261105-LATE01",
                PaymentMethod.COD,
                req.getShippingAddress(),
                req.getNote(),
                List.of(),
                300_000L,
                0L,
                300_000L);
        assertThat(postTtlClaim.claimed()).isFalse();

        assertThatThrownBy(() -> idempotencyService.checkExistingOrThrow(userId, key, mutatedFp))
                .isInstanceOf(DomainException.class)
                .extracting(ex -> ((DomainException) ex).reason())
                .isEqualTo("IDEMPOTENCY_KEY_REUSED");
    }

    private static CreateOrderRequest sampleRequest(String userId, String key, int qty) {
        return CreateOrderRequest.newBuilder()
                .setUserId(userId)
                .setIdempotencyKey(key)
                .setPaymentMethod("COD")
                .setShippingAddress(ShippingAddressSnapshot.newBuilder()
                        .setRecipientName("Nguyen Van A")
                        .setPhone("0901234567")
                        .setProvinceCode("79")
                        .setProvinceName("TP. Ho Chi Minh")
                        .setWardCode("26734")
                        .setWardName("Phuong Ben Nghe")
                        .setAddressLine("1 Le Duan")
                        .build())
                .addItems(OrderItem.newBuilder()
                        .setProductId("p-1")
                        .setSkuId("p-1-v1")
                        .setQuantity(qty)
                        .setExpectedProductVersion(1L)
                        .setExpectedName("Product 1")
                        .setVariantLabel("Default")
                        .setUnitPrice(Money.newBuilder().setCurrency("VND").setAmountMinor(150_000L).build())
                        .build())
                .build();
    }
}
