package com.ecommerce.identity.rbac.grpc;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.read.ListAppender;
import com.ecommerce.contracts.identity.v1.AuthorizationServiceGrpc;
import com.ecommerce.contracts.identity.v1.CheckPermissionRequest;
import com.ecommerce.identity.common.config.IdentityGrpcProperties;
import com.ecommerce.identity.common.error.GrpcTraceIdInterceptor;
import com.ecommerce.identity.rbac.application.RbacAuthorizationService;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.Metadata;
import io.grpc.stub.MetadataUtils;
import java.time.Duration;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;
import org.slf4j.LoggerFactory;

class GrpcServerLifecycleTraceTest {
    private static final UUID USER_ID = UUID.fromString("00000000-0000-0000-0000-000000000043");

    @Test
    void registersTraceInterceptorOnThePrivateGrpcServer() throws InterruptedException {
        RbacAuthorizationService rbac = mock(RbacAuthorizationService.class);
        when(rbac.can(USER_ID, "cart.item.add")).thenReturn(true);
        AuthorizationGrpcService service = new AuthorizationGrpcService(rbac);
        GrpcServerLifecycle lifecycle = new GrpcServerLifecycle(
                new IdentityGrpcProperties("127.0.0.1", 0, Duration.ofSeconds(2)),
                service,
                new GrpcTraceIdInterceptor());
        Logger logger = (Logger) LoggerFactory.getLogger(AuthorizationGrpcService.class);
        Level previousLevel = logger.getLevel();
        ListAppender<ILoggingEvent> appender = new ListAppender<>();
        appender.start();
        logger.setLevel(Level.DEBUG);
        logger.addAppender(appender);
        lifecycle.start();
        ManagedChannel channel = ManagedChannelBuilder
                .forAddress("127.0.0.1", lifecycle.localPort())
                .usePlaintext()
                .build();
        try {
            Metadata headers = new Metadata();
            headers.put(Metadata.Key.of("x-trace-id", Metadata.ASCII_STRING_MARSHALLER), "trace-abc-123");
            AuthorizationServiceGrpc.AuthorizationServiceBlockingStub stub =
                    AuthorizationServiceGrpc.newBlockingStub(channel)
                            .withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers));

            stub.checkPermission(CheckPermissionRequest.newBuilder()
                    .setUserId(USER_ID.toString())
                    .setPermission("cart.item.add")
                    .build());

            assertThat(appender.list)
                    .anySatisfy(event -> assertThat(event.getMDCPropertyMap())
                            .containsEntry("trace_id", "trace-abc-123"));
        } finally {
            channel.shutdownNow();
            channel.awaitTermination(5, TimeUnit.SECONDS);
            lifecycle.stop();
            logger.detachAppender(appender);
            logger.setLevel(previousLevel);
            appender.stop();
        }
    }
}
