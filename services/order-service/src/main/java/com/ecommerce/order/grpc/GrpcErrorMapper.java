package com.ecommerce.order.grpc;

import com.ecommerce.order.common.DomainException;
import com.google.protobuf.Any;
import com.google.rpc.ErrorInfo;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.protobuf.StatusProto;

public final class GrpcErrorMapper {
    public static final String ERROR_DOMAIN = "ecommerce.order.v1";

    private GrpcErrorMapper() {
    }

    public static StatusRuntimeException toStatusException(DomainException ex) {
        ErrorInfo.Builder infoBuilder = ErrorInfo.newBuilder()
                .setReason(ex.reason())
                .setDomain(ERROR_DOMAIN);
        if (ex.metadata() != null && !ex.metadata().isEmpty()) {
            infoBuilder.putAllMetadata(ex.metadata());
        }

        com.google.rpc.Status rpcStatus = com.google.rpc.Status.newBuilder()
                .setCode(ex.grpcCode().value())
                .setMessage(ex.getMessage() == null ? ex.reason() : ex.getMessage())
                .addDetails(Any.pack(infoBuilder.build()))
                .build();
        return StatusProto.toStatusRuntimeException(rpcStatus);
    }

    public static StatusRuntimeException internalError() {
        return Status.INTERNAL.withDescription("order operation failed").asRuntimeException();
    }
}
