package com.ecommerce.order.integration;

import com.ecommerce.contracts.inventory.v1.InventoryServiceGrpc;
import com.ecommerce.contracts.inventory.v1.ReservationLine;
import com.ecommerce.contracts.inventory.v1.ReserveStockRequest;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.common.GrpcTraceIdInterceptor;
import com.ecommerce.order.common.TraceIdProvider;
import com.ecommerce.order.config.OrderProperties;
import com.google.protobuf.Any;
import com.google.protobuf.InvalidProtocolBufferException;
import com.google.rpc.ErrorInfo;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.protobuf.StatusProto;
import io.grpc.stub.MetadataUtils;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.stereotype.Component;

@Component
public class InventoryGrpcAdapter {
    private static final Logger log = LoggerFactory.getLogger(InventoryGrpcAdapter.class);

    private final InventoryServiceGrpc.InventoryServiceBlockingStub stub;
    private final OrderProperties properties;

    public InventoryGrpcAdapter(
            @Qualifier("inventoryManagedChannel") ManagedChannel channel,
            OrderProperties properties) {
        this.stub = InventoryServiceGrpc.newBlockingStub(channel);
        this.properties = properties;
    }

    public void reserveStock(UUID orderId, List<ReservationLine> lines, Instant expiresAt) {
        Metadata headers = new Metadata();
        headers.put(GrpcTraceIdInterceptor.TRACE_ID_KEY, TraceIdProvider.current());
        try {
            stub.withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers))
                    .withDeadlineAfter(properties.clients().inventoryTimeout().toMillis(), TimeUnit.MILLISECONDS)
                    .reserveStock(ReserveStockRequest.newBuilder()
                            .setOrderId(orderId.toString())
                            .addAllItems(lines)
                            .setExpiresAt(expiresAt.toString())
                            .build());
        } catch (StatusRuntimeException ex) {
            ErrorInfo info = extractErrorInfo(ex);
            if (ex.getStatus().getCode() == Status.Code.FAILED_PRECONDITION
                    && info != null
                    && "INVENTORY_INSUFFICIENT".equals(info.getReason())) {
                String msg = ex.getStatus().getDescription() != null
                        ? ex.getStatus().getDescription()
                        : "Insufficient inventory for selected SKU";
                throw new DomainException(
                        Status.Code.FAILED_PRECONDITION,
                        "INVENTORY_INSUFFICIENT",
                        msg,
                        info.getMetadataMap());
            }
            log.warn(
                    "Downstream inventory reserveStock failed orderId={} grpcStatus={} traceId={} description={}",
                    orderId,
                    ex.getStatus().getCode(),
                    TraceIdProvider.current(),
                    ex.getStatus().getDescription());
            throw new DomainException(
                    Status.Code.UNAVAILABLE,
                    "INVENTORY_UNAVAILABLE",
                    "Inventory reservation could not be confirmed",
                    Map.of());
        }
    }

    static ErrorInfo extractErrorInfo(StatusRuntimeException ex) {
        com.google.rpc.Status rpcStatus = StatusProto.fromThrowable(ex);
        if (rpcStatus == null) {
            return null;
        }
        for (Any detail : rpcStatus.getDetailsList()) {
            if (detail.is(ErrorInfo.class)) {
                try {
                    return detail.unpack(ErrorInfo.class);
                } catch (InvalidProtocolBufferException ignored) {
                    return null;
                }
            }
        }
        return null;
    }
}
