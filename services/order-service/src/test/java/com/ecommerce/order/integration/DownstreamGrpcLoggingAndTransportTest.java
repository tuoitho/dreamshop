package com.ecommerce.order.integration;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.read.ListAppender;
import com.ecommerce.contracts.catalog.v1.CatalogServiceGrpc;
import com.ecommerce.contracts.catalog.v1.GetProductRequest;
import com.ecommerce.contracts.catalog.v1.GetProductResponse;
import com.ecommerce.contracts.identity.v1.AuthorizationServiceGrpc;
import com.ecommerce.contracts.identity.v1.CheckPermissionRequest;
import com.ecommerce.contracts.identity.v1.CheckPermissionResponse;
import com.ecommerce.contracts.inventory.v1.InventoryServiceGrpc;
import com.ecommerce.contracts.inventory.v1.ReserveStockRequest;
import com.ecommerce.contracts.inventory.v1.ReserveStockResponse;
import com.ecommerce.contracts.payment.v1.PaymentServiceGrpc;
import com.ecommerce.contracts.payment.v1.RetryRefundRequest;
import com.ecommerce.contracts.payment.v1.RetryRefundResponse;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.common.TraceIdProvider;
import com.ecommerce.order.config.OrderProperties;
import com.google.protobuf.Any;
import com.google.rpc.ErrorInfo;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.Status;
import io.grpc.protobuf.StatusProto;
import io.grpc.stub.StreamObserver;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;

class DownstreamGrpcLoggingAndTransportTest {

    private Server server;
    private ManagedChannel channel;
    private OrderProperties props;
    private final AtomicReference<RuntimeException> nextInventoryError = new AtomicReference<>();
    private final AtomicReference<RuntimeException> nextCatalogError = new AtomicReference<>();
    private final AtomicReference<RuntimeException> nextAuthError = new AtomicReference<>();
    private final AtomicReference<RuntimeException> nextPaymentError = new AtomicReference<>();

    @BeforeEach
    void setUp() throws Exception {
        MDC.put(TraceIdProvider.MDC_KEY, "trace-order-log-1");
        server = ServerBuilder.forPort(0)
                .addService(new InventoryServiceGrpc.InventoryServiceImplBase() {
                    @Override
                    public void reserveStock(
                            ReserveStockRequest request,
                            StreamObserver<ReserveStockResponse> responseObserver) {
                        RuntimeException err = nextInventoryError.get();
                        if (err != null) {
                            responseObserver.onError(err);
                            return;
                        }
                        responseObserver.onNext(ReserveStockResponse.getDefaultInstance());
                        responseObserver.onCompleted();
                    }
                })
                .addService(new CatalogServiceGrpc.CatalogServiceImplBase() {
                    @Override
                    public void getProduct(
                            GetProductRequest request,
                            StreamObserver<GetProductResponse> responseObserver) {
                        RuntimeException err = nextCatalogError.get();
                        if (err != null) {
                            responseObserver.onError(err);
                            return;
                        }
                        responseObserver.onNext(GetProductResponse.getDefaultInstance());
                        responseObserver.onCompleted();
                    }
                })
                .addService(new AuthorizationServiceGrpc.AuthorizationServiceImplBase() {
                    @Override
                    public void checkPermission(
                            CheckPermissionRequest request,
                            StreamObserver<CheckPermissionResponse> responseObserver) {
                        RuntimeException err = nextAuthError.get();
                        if (err != null) {
                            responseObserver.onError(err);
                            return;
                        }
                        responseObserver.onNext(CheckPermissionResponse.newBuilder().setAllowed(true).build());
                        responseObserver.onCompleted();
                    }
                })
                .addService(new PaymentServiceGrpc.PaymentServiceImplBase() {
                    @Override
                    public void retryRefund(
                            RetryRefundRequest request,
                            StreamObserver<RetryRefundResponse> responseObserver) {
                        RuntimeException err = nextPaymentError.get();
                        if (err != null) {
                            responseObserver.onError(err);
                            return;
                        }
                        responseObserver.onNext(RetryRefundResponse.getDefaultInstance());
                        responseObserver.onCompleted();
                    }
                })
                .build()
                .start();
        channel = ManagedChannelBuilder.forAddress("127.0.0.1", server.getPort())
                .usePlaintext()
                .build();
        props = new OrderProperties(
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
    }

    @AfterEach
    void tearDown() throws Exception {
        MDC.remove(TraceIdProvider.MDC_KEY);
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
    void inventoryAdapterLogsWarnOnUnavailableAndStaysQuietOnInsufficientStock() {
        ListAppender<ILoggingEvent> appender = attachAppender(InventoryGrpcAdapter.class);
        try {
            InventoryGrpcAdapter adapter = new InventoryGrpcAdapter(channel, props);

            nextInventoryError.set(Status.UNAVAILABLE.withDescription("connection refused").asRuntimeException());
            assertThatThrownBy(() -> adapter.reserveStock(UUID.randomUUID(), List.of(), Instant.now().plusSeconds(60)))
                    .isInstanceOf(DomainException.class);

            assertThat(appender.list)
                    .anySatisfy(event -> {
                        assertThat(event.getLevel()).isEqualTo(Level.WARN);
                        assertThat(event.getFormattedMessage())
                                .contains("trace-order-log-1")
                                .contains("UNAVAILABLE");
                    });

            appender.list.clear();
            com.google.rpc.Status insufficientStatus = com.google.rpc.Status.newBuilder()
                    .setCode(com.google.rpc.Code.FAILED_PRECONDITION_VALUE)
                    .setMessage("Insufficient stock")
                    .addDetails(Any.pack(ErrorInfo.newBuilder()
                            .setReason("INVENTORY_INSUFFICIENT")
                            .build()))
                    .build();
            nextInventoryError.set(StatusProto.toStatusRuntimeException(insufficientStatus));
            assertThatThrownBy(() -> adapter.reserveStock(UUID.randomUUID(), List.of(), Instant.now().plusSeconds(60)))
                    .isInstanceOf(DomainException.class);

            assertThat(appender.list).isEmpty();
        } finally {
            detachAppender(InventoryGrpcAdapter.class, appender);
        }
    }

    @Test
    void catalogAuthorizationAndPaymentAdaptersLogDownstreamFailuresWithTraceId() {
        ListAppender<ILoggingEvent> catalogAppender = attachAppender(CatalogGrpcAdapter.class);
        ListAppender<ILoggingEvent> authAppender = attachAppender(AuthorizationGrpcAdapter.class);
        ListAppender<ILoggingEvent> paymentAppender = attachAppender(PaymentGrpcAdapter.class);
        try {
            CatalogGrpcAdapter catalogAdapter = new CatalogGrpcAdapter(channel, props);
            nextCatalogError.set(Status.UNAVAILABLE.withDescription("catalog down").asRuntimeException());
            assertThatThrownBy(() -> catalogAdapter.getProduct("prod-1"))
                    .isInstanceOf(DomainException.class);
            assertThat(catalogAppender.list)
                    .anySatisfy(event -> {
                        assertThat(event.getLevel()).isEqualTo(Level.WARN);
                        assertThat(event.getFormattedMessage()).contains("trace-order-log-1").contains("UNAVAILABLE");
                    });

            AuthorizationGrpcAdapter authAdapter = new AuthorizationGrpcAdapter(channel, props);
            nextAuthError.set(Status.DEADLINE_EXCEEDED.withDescription("timeout").asRuntimeException());
            assertThatThrownBy(() -> authAdapter.hasPermission(UUID.randomUUID(), "order:read"))
                    .isInstanceOf(DomainException.class);
            assertThat(authAppender.list)
                    .anySatisfy(event -> {
                        assertThat(event.getLevel()).isEqualTo(Level.WARN);
                        assertThat(event.getFormattedMessage())
                                .contains("trace-order-log-1")
                                .contains("DEADLINE_EXCEEDED");
                    });

            PaymentGrpcAdapter paymentAdapter = new PaymentGrpcAdapter(channel, props);
            nextPaymentError.set(Status.INTERNAL.withDescription("db failure").asRuntimeException());
            assertThatThrownBy(() -> paymentAdapter.retryRefund(UUID.randomUUID(), "manual"))
                    .isInstanceOf(DomainException.class);
            assertThat(paymentAppender.list)
                    .anySatisfy(event -> {
                        assertThat(event.getLevel()).isEqualTo(Level.ERROR);
                        assertThat(event.getFormattedMessage()).contains("trace-order-log-1").contains("INTERNAL");
                    });
        } finally {
            detachAppender(CatalogGrpcAdapter.class, catalogAppender);
            detachAppender(AuthorizationGrpcAdapter.class, authAppender);
            detachAppender(PaymentGrpcAdapter.class, paymentAppender);
        }
    }

    @Test
    void nettyClientHandlesGracefulServerGoAwayWithoutNoSuchMethodError() throws Exception {
        Logger http2Logger = (Logger) LoggerFactory.getLogger(
                "io.grpc.netty.shaded.io.netty.handler.codec.http2.DefaultHttp2Connection");
        ListAppender<ILoggingEvent> http2Appender = new ListAppender<>();
        http2Appender.start();
        http2Logger.addAppender(http2Appender);
        try {
            // Establish an active HTTP/2 transport before graceful server shutdown sends GOAWAY.
            CatalogGrpcAdapter catalogAdapter = new CatalogGrpcAdapter(channel, props);
            assertThat(catalogAdapter.getProduct("prod-1")).isPresent();

            server.shutdown();
            assertThat(server.awaitTermination(5, TimeUnit.SECONDS)).isTrue();
            // Give the Netty event loop a brief moment to process the GOAWAY frame.
            Thread.sleep(150);

            assertThat(http2Appender.list)
                    .noneMatch(event -> event.getThrowableProxy() != null
                            && event.getThrowableProxy().getClassName().contains("NoSuchMethodError"));
        } finally {
            http2Logger.detachAppender(http2Appender);
            http2Appender.stop();
        }
    }

    private static ListAppender<ILoggingEvent> attachAppender(Class<?> target) {
        Logger logger = (Logger) LoggerFactory.getLogger(target);
        ListAppender<ILoggingEvent> appender = new ListAppender<>();
        appender.start();
        logger.addAppender(appender);
        return appender;
    }

    private static void detachAppender(Class<?> target, ListAppender<ILoggingEvent> appender) {
        Logger logger = (Logger) LoggerFactory.getLogger(target);
        logger.detachAppender(appender);
        appender.stop();
    }
}
