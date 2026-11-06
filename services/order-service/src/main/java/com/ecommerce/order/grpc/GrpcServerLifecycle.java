package com.ecommerce.order.grpc;

import com.ecommerce.order.common.GrpcTraceIdInterceptor;
import com.ecommerce.order.config.OrderProperties;
import io.grpc.Server;
import io.grpc.ServerInterceptors;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.time.Duration;
import java.util.concurrent.TimeUnit;
import org.springframework.context.SmartLifecycle;
import org.springframework.stereotype.Component;

@Component
public class GrpcServerLifecycle implements SmartLifecycle {
    private final OrderProperties properties;
    private final OrderGrpcService orderGrpcService;
    private final GrpcTraceIdInterceptor traceIdInterceptor;
    private volatile Server server;
    private volatile boolean running;

    public GrpcServerLifecycle(
            OrderProperties properties,
            OrderGrpcService orderGrpcService,
            GrpcTraceIdInterceptor traceIdInterceptor) {
        this.properties = properties;
        this.orderGrpcService = orderGrpcService;
        this.traceIdInterceptor = traceIdInterceptor;
    }

    @Override
    public synchronized void start() {
        if (running) {
            return;
        }
        String host = properties.grpc() != null && properties.grpc().host() != null
                ? properties.grpc().host()
                : "127.0.0.1";
        int port = properties.grpc() != null ? properties.grpc().port() : 50055;
        try {
            server = NettyServerBuilder
                    .forAddress(new InetSocketAddress(host, port))
                    .addService(ServerInterceptors.intercept(orderGrpcService, traceIdInterceptor))
                    .build()
                    .start();
            running = true;
        } catch (IOException ex) {
            throw new IllegalStateException("Unable to start Order gRPC listener at %s:%d".formatted(host, port), ex);
        }
    }

    @Override
    public synchronized void stop(Runnable callback) {
        Duration grace = properties.grpc() != null && properties.grpc().shutdownGrace() != null
                ? properties.grpc().shutdownGrace()
                : Duration.ofSeconds(5);
        try {
            if (server != null) {
                server.shutdown();
                if (!server.awaitTermination(grace.toNanos(), TimeUnit.NANOSECONDS)) {
                    server.shutdownNow();
                    server.awaitTermination(grace.toNanos(), TimeUnit.NANOSECONDS);
                }
            }
        } catch (InterruptedException ex) {
            if (server != null) {
                server.shutdownNow();
            }
            Thread.currentThread().interrupt();
        } finally {
            server = null;
            running = false;
            callback.run();
        }
    }

    @Override
    public void stop() {
        stop(() -> { });
    }

    @Override
    public boolean isRunning() {
        return running;
    }

    @Override
    public boolean isAutoStartup() {
        return true;
    }

    public int localPort() {
        Server active = server;
        if (!running || active == null) {
            throw new IllegalStateException("Order gRPC listener is not running");
        }
        return active.getPort();
    }
}
