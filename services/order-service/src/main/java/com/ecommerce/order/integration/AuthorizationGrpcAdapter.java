package com.ecommerce.order.integration;

import com.ecommerce.contracts.identity.v1.AuthorizationServiceGrpc;
import com.ecommerce.contracts.identity.v1.CheckPermissionRequest;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.common.GrpcTraceIdInterceptor;
import com.ecommerce.order.common.TraceIdProvider;
import com.ecommerce.order.config.OrderProperties;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.MetadataUtils;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.stereotype.Component;

@Component
public class AuthorizationGrpcAdapter {
    private static final Logger log = LoggerFactory.getLogger(AuthorizationGrpcAdapter.class);

    private final AuthorizationServiceGrpc.AuthorizationServiceBlockingStub stub;
    private final OrderProperties properties;

    public AuthorizationGrpcAdapter(
            @Qualifier("identityManagedChannel") ManagedChannel channel,
            OrderProperties properties) {
        this.stub = AuthorizationServiceGrpc.newBlockingStub(channel);
        this.properties = properties;
    }

    public boolean hasPermission(UUID userId, String permission) {
        Metadata headers = new Metadata();
        headers.put(GrpcTraceIdInterceptor.TRACE_ID_KEY, TraceIdProvider.current());
        try {
            return stub.withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers))
                    .withDeadlineAfter(properties.clients().identityTimeout().toMillis(), TimeUnit.MILLISECONDS)
                    .checkPermission(CheckPermissionRequest.newBuilder()
                            .setUserId(userId.toString())
                            .setPermission(permission)
                            .build())
                    .getAllowed();
        } catch (StatusRuntimeException ex) {
            log.warn(
                    "Downstream identity checkPermission failed userId={} permission={} grpcStatus={} traceId={} description={}",
                    userId,
                    permission,
                    ex.getStatus().getCode(),
                    TraceIdProvider.current(),
                    ex.getStatus().getDescription());
            throw new DomainException(
                    Status.Code.UNAVAILABLE,
                    "AUTHORIZATION_UNAVAILABLE",
                    "Authorization service is unavailable");
        }
    }
}
