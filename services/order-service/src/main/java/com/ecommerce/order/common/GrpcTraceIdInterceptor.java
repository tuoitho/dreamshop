package com.ecommerce.order.common;

import io.grpc.ForwardingServerCallListener;
import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import java.util.function.Supplier;
import org.slf4j.MDC;
import org.springframework.stereotype.Component;

@Component
public class GrpcTraceIdInterceptor implements ServerInterceptor {
    public static final Metadata.Key<String> TRACE_ID_KEY =
            Metadata.Key.of("x-trace-id", Metadata.ASCII_STRING_MARSHALLER);

    @Override
    public <ReqT, RespT> ServerCall.Listener<ReqT> interceptCall(
            ServerCall<ReqT, RespT> call,
            Metadata headers,
            ServerCallHandler<ReqT, RespT> next) {
        String traceId = TraceIdProvider.resolve(headers.get(TRACE_ID_KEY));
        ServerCall.Listener<ReqT> listener = withTraceId(traceId, () -> next.startCall(call, headers));
        return new ForwardingServerCallListener.SimpleForwardingServerCallListener<>(listener) {
            @Override
            public void onMessage(ReqT message) {
                withTraceId(traceId, () -> super.onMessage(message));
            }

            @Override
            public void onHalfClose() {
                withTraceId(traceId, super::onHalfClose);
            }

            @Override
            public void onCancel() {
                withTraceId(traceId, super::onCancel);
            }

            @Override
            public void onComplete() {
                withTraceId(traceId, super::onComplete);
            }

            @Override
            public void onReady() {
                withTraceId(traceId, super::onReady);
            }
        };
    }

    private <T> T withTraceId(String traceId, Supplier<T> action) {
        String previous = MDC.get(TraceIdProvider.MDC_KEY);
        MDC.put(TraceIdProvider.MDC_KEY, traceId);
        try {
            return action.get();
        } finally {
            if (previous == null) {
                MDC.remove(TraceIdProvider.MDC_KEY);
            } else {
                MDC.put(TraceIdProvider.MDC_KEY, previous);
            }
        }
    }

    private void withTraceId(String traceId, Runnable action) {
        withTraceId(traceId, () -> {
            action.run();
            return null;
        });
    }
}
