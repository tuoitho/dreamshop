package com.ecommerce.identity.rbac.grpc;

import com.ecommerce.identity.common.config.IdentityGrpcProperties;
import io.grpc.Server;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.util.concurrent.TimeUnit;
import org.springframework.context.SmartLifecycle;
import org.springframework.stereotype.Component;

@Component
public class GrpcServerLifecycle implements SmartLifecycle {
    private final IdentityGrpcProperties properties;
    private final AuthorizationGrpcService authorizationService;
    private volatile Server server;
    private volatile boolean running;

    public GrpcServerLifecycle(IdentityGrpcProperties properties,
            AuthorizationGrpcService authorizationService) {
        this.properties = properties;
        this.authorizationService = authorizationService;
    }

    @Override
    public synchronized void start() {
        if (running) {
            return;
        }
        try {
            server = NettyServerBuilder
                    .forAddress(new InetSocketAddress(properties.host(), properties.port()))
                    .addService(authorizationService)
                    .build()
                    .start();
            running = true;
        } catch (IOException exception) {
            throw new IllegalStateException("Unable to start Identity gRPC listener at %s:%d"
                    .formatted(properties.host(), properties.port()), exception);
        }
    }

    @Override
    public synchronized void stop(Runnable callback) {
        try {
            if (server != null) {
                server.shutdown();
                if (!server.awaitTermination(properties.shutdownGrace().toNanos(), TimeUnit.NANOSECONDS)) {
                    server.shutdownNow();
                    server.awaitTermination(properties.shutdownGrace().toNanos(), TimeUnit.NANOSECONDS);
                }
            }
        } catch (InterruptedException exception) {
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
        Server activeServer = server;
        if (!running || activeServer == null) {
            throw new IllegalStateException("Identity gRPC listener is not running");
        }
        return activeServer.getPort();
    }
}
