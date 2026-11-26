package com.ecommerce.order.integration;

import com.ecommerce.contracts.catalog.v1.CatalogServiceGrpc;
import com.ecommerce.contracts.catalog.v1.GetProductRequest;
import com.ecommerce.contracts.catalog.v1.GetProductResponse;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.common.GrpcTraceIdInterceptor;
import com.ecommerce.order.common.TraceIdProvider;
import com.ecommerce.order.config.OrderProperties;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.MetadataUtils;
import java.util.Optional;
import java.util.concurrent.TimeUnit;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.stereotype.Component;

@Component
public class CatalogGrpcAdapter {
    private static final Logger log = LoggerFactory.getLogger(CatalogGrpcAdapter.class);

    private final CatalogServiceGrpc.CatalogServiceBlockingStub stub;
    private final OrderProperties properties;

    public CatalogGrpcAdapter(
            @Qualifier("catalogManagedChannel") ManagedChannel channel,
            OrderProperties properties) {
        this.stub = CatalogServiceGrpc.newBlockingStub(channel);
        this.properties = properties;
    }

    public Optional<GetProductResponse> getProduct(String productId) {
        Metadata headers = new Metadata();
        headers.put(GrpcTraceIdInterceptor.TRACE_ID_KEY, TraceIdProvider.current());
        try {
            GetProductResponse response = stub
                    .withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers))
                    .withDeadlineAfter(properties.clients().catalogTimeout().toMillis(), TimeUnit.MILLISECONDS)
                    .getProduct(GetProductRequest.newBuilder().setId(productId).build());
            return Optional.of(response);
        } catch (StatusRuntimeException ex) {
            if (ex.getStatus().getCode() == Status.Code.NOT_FOUND) {
                return Optional.empty();
            }
            log.warn(
                    "Downstream catalog getProduct failed productId={} grpcStatus={} traceId={} description={}",
                    productId,
                    ex.getStatus().getCode(),
                    TraceIdProvider.current(),
                    ex.getStatus().getDescription());
            throw new DomainException(
                    Status.Code.UNAVAILABLE,
                    "CATALOG_UNAVAILABLE",
                    "Catalog service is unavailable");
        }
    }
}
