package com.ecommerce.order.integration;

import com.ecommerce.contracts.common.v1.Money;
import com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest;
import com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse;
import com.ecommerce.contracts.payment.v1.PaymentServiceGrpc;
import com.ecommerce.contracts.payment.v1.RetryRefundRequest;
import com.ecommerce.contracts.payment.v1.RetryRefundResponse;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.common.GrpcTraceIdInterceptor;
import com.ecommerce.order.common.TraceIdProvider;
import com.ecommerce.order.config.OrderProperties;
import com.ecommerce.order.domain.ShippingPolicy;
import com.google.rpc.ErrorInfo;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.MetadataUtils;
import java.time.Instant;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.stereotype.Component;

@Component
public class PaymentGrpcAdapter {
    private final PaymentServiceGrpc.PaymentServiceBlockingStub stub;
    private final OrderProperties properties;

    public PaymentGrpcAdapter(
            @Qualifier("paymentManagedChannel") ManagedChannel channel,
            OrderProperties properties) {
        this.stub = PaymentServiceGrpc.newBlockingStub(channel);
        this.properties = properties;
    }

    public String createPaymentSession(
            UUID orderId,
            String orderCode,
            UUID userId,
            long amountMinor,
            Instant expiresAt) {
        Metadata headers = new Metadata();
        headers.put(GrpcTraceIdInterceptor.TRACE_ID_KEY, TraceIdProvider.current());
        try {
            CreatePaymentSessionResponse response = stub
                    .withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers))
                    .withDeadlineAfter(properties.clients().paymentTimeout().toMillis(), TimeUnit.MILLISECONDS)
                    .createPaymentSession(CreatePaymentSessionRequest.newBuilder()
                            .setOrderId(orderId.toString())
                            .setOrderCode(orderCode)
                            .setUserId(userId.toString())
                            .setAmount(Money.newBuilder()
                                    .setCurrency(ShippingPolicy.CURRENCY)
                                    .setAmountMinor(amountMinor)
                                    .build())
                            .setExpiresAt(expiresAt.toString())
                            .build());
            return response.getPayUrl();
        } catch (StatusRuntimeException ex) {
            ErrorInfo info = InventoryGrpcAdapter.extractErrorInfo(ex);
            if (ex.getStatus().getCode() == Status.Code.FAILED_PRECONDITION && info != null) {
                if ("PAYMENT_GATEWAY_REJECTED".equals(info.getReason())) {
                    throw new DomainException(
                            Status.Code.FAILED_PRECONDITION,
                            "PAYMENT_GATEWAY_REJECTED",
                            "Payment gateway rejected session");
                }
                if ("PAYMENT_ALREADY_CANCELLED".equals(info.getReason())) {
                    throw new DomainException(
                            Status.Code.FAILED_PRECONDITION,
                            "PAYMENT_ALREADY_CANCELLED",
                            "Payment session was already cancelled");
                }
            }
            throw new DomainException(
                    Status.Code.UNAVAILABLE,
                    "PAYMENT_GATEWAY_UNAVAILABLE",
                    "Payment service is unavailable");
        }
    }

    public record RetryRefundOutcome(String paymentStatus, String refundTransId, String message) {
    }

    public RetryRefundOutcome retryRefund(UUID orderId, String reason) {
        Metadata headers = new Metadata();
        headers.put(GrpcTraceIdInterceptor.TRACE_ID_KEY, TraceIdProvider.current());
        try {
            RetryRefundResponse response = stub
                    .withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers))
                    .withDeadlineAfter(properties.clients().paymentTimeout().toMillis(), TimeUnit.MILLISECONDS)
                    .retryRefund(RetryRefundRequest.newBuilder()
                            .setOrderId(orderId.toString())
                            .setReason(reason == null ? "" : reason)
                            .build());
            return new RetryRefundOutcome(
                    response.getPaymentStatus(),
                    response.getRefundTransId(),
                    response.getMessage());
        } catch (StatusRuntimeException ex) {
            ErrorInfo info = InventoryGrpcAdapter.extractErrorInfo(ex);
            String errorReason = info != null ? info.getReason() : "";
            String desc = ex.getStatus().getDescription() == null ? "" : ex.getStatus().getDescription().trim();
            switch (errorReason) {
                case "PAYMENT_REFUND_NOT_FOUND" -> throw new DomainException(
                        Status.Code.FAILED_PRECONDITION,
                        "ORDER_REFUND_NOT_ELIGIBLE",
                        "Order is not eligible for manual refund");
                case "REFUND_AUTO_RETRY_IN_PROGRESS" -> throw new DomainException(
                        Status.Code.FAILED_PRECONDITION,
                        "REFUND_AUTO_RETRY_IN_PROGRESS",
                        "Automatic refund retry is still in progress");
                case "REFUND_IN_PROGRESS" -> throw new DomainException(
                        Status.Code.ABORTED,
                        "REFUND_IN_PROGRESS",
                        "A refund attempt is already in progress");
                case "PAYMENT_GATEWAY_REJECTED" -> throw new DomainException(
                        Status.Code.FAILED_PRECONDITION,
                        "PAYMENT_GATEWAY_REJECTED",
                        desc.isEmpty() ? "Payment gateway rejected refund" : desc);
                case "PAYMENT_GATEWAY_UNAVAILABLE" -> throw new DomainException(
                        Status.Code.UNAVAILABLE,
                        "PAYMENT_GATEWAY_UNAVAILABLE",
                        "Payment gateway is temporarily unavailable");
                default -> {
                    if (ex.getStatus().getCode() == Status.Code.INTERNAL) {
                        throw new DomainException(
                                Status.Code.INTERNAL,
                                "PAYMENT_INTERNAL_ERROR",
                                "Payment service encountered an internal error");
                    }
                    throw new DomainException(
                            Status.Code.UNAVAILABLE,
                            "PAYMENT_GATEWAY_UNAVAILABLE",
                            "Payment service is unavailable");
                }
            }
        }
    }
}
