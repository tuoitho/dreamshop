package com.ecommerce.order.grpc;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.reset;
import static org.mockito.Mockito.when;

import com.ecommerce.contracts.catalog.v1.GetProductResponse;
import com.ecommerce.contracts.catalog.v1.ProductVariant;
import com.ecommerce.contracts.common.v1.Money;
import com.ecommerce.contracts.order.v1.CreateOrderRequest;
import com.ecommerce.contracts.order.v1.CreateOrderResponse;
import com.ecommerce.contracts.order.v1.GetOrderRequest;
import com.ecommerce.contracts.order.v1.ListOrdersRequest;
import com.ecommerce.contracts.order.v1.ListOrdersResponse;
import com.ecommerce.contracts.order.v1.OrderItem;
import com.ecommerce.contracts.order.v1.OrderServiceGrpc;
import com.ecommerce.contracts.order.v1.ShippingAddressSnapshot;
import com.ecommerce.order.config.RabbitMqConfig;
import com.ecommerce.order.integration.AuthorizationGrpcAdapter;
import com.ecommerce.order.integration.CatalogGrpcAdapter;
import com.ecommerce.order.integration.InventoryGrpcAdapter;
import com.ecommerce.order.integration.PaymentGrpcAdapter;
import com.ecommerce.order.support.PostgresRabbitIntegrationTestSupport;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.amqp.rabbit.core.RabbitAdmin;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.bean.override.mockito.MockitoBean;

class OrderGrpcIntegrationTest extends PostgresRabbitIntegrationTestSupport {

    @Autowired
    GrpcServerLifecycle grpcLifecycle;

    @Autowired
    RabbitTemplate rabbitTemplate;

    @Autowired
    RabbitAdmin rabbitAdmin;

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

    private ManagedChannel channel;

    @BeforeEach
    void setUp() {
        jdbcTemplate.execute("TRUNCATE TABLE order_items, orders, idempotency_records, outbox_events, processed_events CASCADE");
        reset(authorizationAdapter, catalogAdapter, inventoryAdapter, paymentAdapter);
        when(authorizationAdapter.hasPermission(any(UUID.class), anyString())).thenReturn(true);
        channel = ManagedChannelBuilder.forAddress("127.0.0.1", grpcLifecycle.localPort())
                .usePlaintext()
                .build();
    }

    @AfterEach
    void tearDown() throws InterruptedException {
        if (channel != null) {
            channel.shutdownNow();
            channel.awaitTermination(5, TimeUnit.SECONDS);
        }
    }

    @Test
    void endToEndMomoOrderReceivesPaymentSucceededEventAndPublishesOrderConfirmedViaOutbox() throws Exception {
        UUID userId = UUID.randomUUID();
        when(catalogAdapter.getProduct("p-e2e")).thenReturn(Optional.of(GetProductResponse.newBuilder()
                .setId("p-e2e")
                .setName("Smartwatch")
                .setBrand("Garmin")
                .setCategoryName("Wearables")
                .setVersion(5L)
                .setPrice(Money.newBuilder().setCurrency("VND").setAmountMinor(400_000L).build())
                .addVariants(ProductVariant.newBuilder()
                        .setSkuId("p-e2e-v1")
                        .setLabel("42mm")
                        .setPrice(Money.newBuilder().setCurrency("VND").setAmountMinor(400_000L).build())
                        .build())
                .build()));
        when(paymentAdapter.createPaymentSession(any(UUID.class), anyString(), eq(userId), anyLong(), any()))
                .thenReturn("https://momo.test/pay/e2e");

        Queue probeQueue = new Queue("probe.order.confirmed." + UUID.randomUUID(), false, false, true);
        rabbitAdmin.declareQueue(probeQueue);
        rabbitAdmin.declareBinding(BindingBuilder.bind(probeQueue)
                .to(new TopicExchange(RabbitMqConfig.EXCHANGE_NAME, true, false))
                .with("order.confirmed"));

        OrderServiceGrpc.OrderServiceBlockingStub stub = OrderServiceGrpc.newBlockingStub(channel);
        CreateOrderResponse created = stub.createOrder(CreateOrderRequest.newBuilder()
                .setUserId(userId.toString())
                .setIdempotencyKey("idem-e2e-1")
                .setPaymentMethod("MOMO")
                .setShippingAddress(ShippingAddressSnapshot.newBuilder()
                        .setRecipientName("Pham D")
                        .setPhone("0933445566")
                        .setProvinceCode("79")
                        .setProvinceName("HCM")
                        .setWardCode("26734")
                        .setWardName("Ben Nghe")
                        .setAddressLine("99 Nguyen Hue")
                        .build())
                .addItems(OrderItem.newBuilder()
                        .setCartItemId("ci-e2e-1")
                        .setProductId("p-e2e")
                        .setSkuId("p-e2e-v1")
                        .setQuantity(1)
                        .setExpectedName("Smartwatch")
                        .setVariantLabel("42mm")
                        .setExpectedProductVersion(5L)
                        .setUnitPrice(Money.newBuilder().setCurrency("VND").setAmountMinor(400_000L).build())
                        .build())
                .build());

        assertThat(created.getStatus()).isEqualTo("PENDING_PAYMENT");
        assertThat(created.getPayUrl()).isEqualTo("https://momo.test/pay/e2e");

        // Publish payment.succeeded to RabbitMQ -> PaymentEventConsumer -> OutboxRelayScheduler -> order.confirmed
        String payload = """
                {
                  "eventId": "evt-e2e-pay-1",
                  "eventType": "PaymentSucceeded",
                  "eventVersion": 1,
                  "aggregateId": "%s",
                  "correlationId": "trace-e2e",
                  "data": {
                    "orderId": "%s",
                    "momoTransId": "momo-e2e-99",
                    "amountMinor": 400000
                  }
                }
                """.formatted(created.getOrderId(), created.getOrderId());
        MessageProperties props = new MessageProperties();
        props.setContentType(MessageProperties.CONTENT_TYPE_JSON);
        rabbitTemplate.send(
                RabbitMqConfig.EXCHANGE_NAME,
                "payment.succeeded",
                new Message(payload.getBytes(StandardCharsets.UTF_8), props));

        Message confirmedMsg = null;
        Instant deadline = Instant.now().plus(Duration.ofSeconds(8));
        while (Instant.now().isBefore(deadline) && confirmedMsg == null) {
            confirmedMsg = rabbitTemplate.receive(probeQueue.getName(), 250);
        }
        assertThat(confirmedMsg).isNotNull();
        String body = new String(confirmedMsg.getBody(), StandardCharsets.UTF_8);
        assertThat(body).contains("\"OrderConfirmed\"").contains(created.getOrderId());

        var fetched = stub.getOrder(GetOrderRequest.newBuilder()
                .setUserId(userId.toString())
                .setOrderId(created.getOrderId())
                .build());
        assertThat(fetched.getOrder().getStatus()).isEqualTo("CONFIRMED");
        assertThat(fetched.getOrder().getPaymentStatus()).isEqualTo("PAID");

        ListOrdersResponse listed = stub.listOrders(ListOrdersRequest.newBuilder()
                .setUserId(userId.toString())
                .setPage(1)
                .setPageSize(10)
                .build());
        assertThat(listed.getTotalItems()).isEqualTo(1L);
        assertThat(listed.getItems(0).getOrderId()).isEqualTo(created.getOrderId());
    }
}
