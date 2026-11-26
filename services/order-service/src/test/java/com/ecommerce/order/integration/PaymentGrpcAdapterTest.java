package com.ecommerce.order.integration;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.ecommerce.contracts.payment.v1.PaymentServiceGrpc;
import com.ecommerce.contracts.payment.v1.RetryRefundRequest;
import com.ecommerce.contracts.payment.v1.RetryRefundResponse;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.config.OrderProperties;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.time.Duration;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class PaymentGrpcAdapterTest {

    private Server server;
    private ManagedChannel channel;
    private PaymentGrpcAdapter adapter;
    private final AtomicReference<Status> nextError = new AtomicReference<>();

    @BeforeEach
    void setUp() throws Exception {
        server = ServerBuilder.forPort(0)
                .directExecutor()
                .addService(new PaymentServiceGrpc.PaymentServiceImplBase() {
                    @Override
                    public void retryRefund(
                            RetryRefundRequest request,
                            StreamObserver<RetryRefundResponse> responseObserver) {
                        responseObserver.onError(nextError.get().asRuntimeException());
                    }
                })
                .build()
                .start();
        channel = ManagedChannelBuilder.forAddress("127.0.0.1", server.getPort())
                .usePlaintext()
                .directExecutor()
                .build();
        OrderProperties props = new OrderProperties(
                null,
                new OrderProperties.Clients(
                        "localhost:50054",
                        Duration.ofMillis(500),
                        "localhost:50051",
                        Duration.ofSeconds(3),
                        "localhost:50052",
                        Duration.ofSeconds(3),
                        "localhost:50056",
                        Duration.ofSeconds(12)),
                null,
                null,
                null,
                null,
                null);
        adapter = new PaymentGrpcAdapter(channel, props);
    }

    @AfterEach
    void tearDown() throws Exception {
        if (channel != null) {
            channel.shutdownNow();
            channel.awaitTermination(5, TimeUnit.SECONDS);
        }
        if (server != null) {
            server.shutdownNow();
            server.awaitTermination(5, TimeUnit.SECONDS);
        }
    }

    @Test
    void retryRefundMapsInternalErrorSeparatelyWithoutLeakingRawDescription() {
        nextError.set(Status.INTERNAL.withDescription("payment operation failed: pq: connection refused"));

        assertThatThrownBy(() -> adapter.retryRefund(UUID.randomUUID(), "retry"))
                .isInstanceOf(DomainException.class)
                .satisfies(ex -> {
                    DomainException de = (DomainException) ex;
                    assertThat(de.grpcCode()).isEqualTo(Status.Code.INTERNAL);
                    assertThat(de.reason()).isEqualTo("PAYMENT_INTERNAL_ERROR");
                    assertThat(de.getMessage()).doesNotContain("pq:");
                });
    }

    @Test
    void retryRefundMapsUnreasonedUnavailableAndDeadlineToFixedSafeMessage() {
        nextError.set(Status.DEADLINE_EXCEEDED.withDescription("internal socket timeout at 10.0.0.5:50056"));

        assertThatThrownBy(() -> adapter.retryRefund(UUID.randomUUID(), "retry"))
                .isInstanceOf(DomainException.class)
                .satisfies(ex -> {
                    DomainException de = (DomainException) ex;
                    assertThat(de.grpcCode()).isEqualTo(Status.Code.UNAVAILABLE);
                    assertThat(de.reason()).isEqualTo("PAYMENT_GATEWAY_UNAVAILABLE");
                    assertThat(de.getMessage()).isEqualTo("Payment service is unavailable");
                });
    }
}
