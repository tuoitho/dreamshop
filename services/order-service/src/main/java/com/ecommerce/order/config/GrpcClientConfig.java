package com.ecommerce.order.config;

import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import jakarta.annotation.PreDestroy;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class GrpcClientConfig {
    private final List<ManagedChannel> channels = new ArrayList<>();

    @Bean(name = "identityManagedChannel")
    public ManagedChannel identityManagedChannel(OrderProperties props) {
        return register(props.clients().identityAddr());
    }

    @Bean(name = "catalogManagedChannel")
    public ManagedChannel catalogManagedChannel(OrderProperties props) {
        return register(props.clients().catalogAddr());
    }

    @Bean(name = "inventoryManagedChannel")
    public ManagedChannel inventoryManagedChannel(OrderProperties props) {
        return register(props.clients().inventoryAddr());
    }

    @Bean(name = "paymentManagedChannel")
    public ManagedChannel paymentManagedChannel(OrderProperties props) {
        return register(props.clients().paymentAddr());
    }

    private ManagedChannel register(String target) {
        ManagedChannel channel = ManagedChannelBuilder.forTarget(target)
                .usePlaintext()
                .build();
        channels.add(channel);
        return channel;
    }

    @PreDestroy
    public void shutdownChannels() {
        for (ManagedChannel ch : channels) {
            ch.shutdown();
        }
        for (ManagedChannel ch : channels) {
            try {
                if (!ch.awaitTermination(2, TimeUnit.SECONDS)) {
                    ch.shutdownNow();
                }
            } catch (InterruptedException e) {
                ch.shutdownNow();
                Thread.currentThread().interrupt();
            }
        }
    }
}
