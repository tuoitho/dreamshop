package com.ecommerce.identity.common.error;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.Status;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;
import org.slf4j.MDC;

class GrpcTraceIdInterceptorTest {
    private static final Metadata.Key<String> TRACE_ID_KEY =
            Metadata.Key.of("x-trace-id", Metadata.ASCII_STRING_MARSHALLER);

    @AfterEach
    void clearMdc() {
        MDC.clear();
    }

    @Test
    void scopesIncomingTraceIdDuringSuccessfulRpcAndClearsItAfterward() {
        assertTraceScopedAndCleared(Status.OK);
    }

    @Test
    void scopesIncomingTraceIdDuringFailedRpcAndClearsItAfterward() {
        assertTraceScopedAndCleared(Status.INTERNAL.withDescription("authorization check failed"));
    }

    private void assertTraceScopedAndCleared(Status returnedStatus) {
        GrpcTraceIdInterceptor interceptor = new GrpcTraceIdInterceptor();
        Metadata headers = new Metadata();
        headers.put(TRACE_ID_KEY, "trace-abc-123");
        AtomicReference<String> handlingTrace = new AtomicReference<>();
        ServerCall<String, String> call = serverCall();
        ServerCallHandler<String, String> next = (activeCall, ignoredHeaders) ->
                new ServerCall.Listener<>() {
                    @Override
                    public void onHalfClose() {
                        handlingTrace.set(MDC.get("trace_id"));
                        activeCall.close(returnedStatus, new Metadata());
                    }
                };

        ServerCall.Listener<String> listener = interceptor.interceptCall(call, headers, next);
        assertThat(MDC.get("trace_id")).isNull();

        listener.onHalfClose();

        assertThat(handlingTrace).hasValue("trace-abc-123");
        assertThat(MDC.get("trace_id")).isNull();
    }

    @SuppressWarnings("unchecked")
    private ServerCall<String, String> serverCall() {
        return mock(ServerCall.class);
    }
}
