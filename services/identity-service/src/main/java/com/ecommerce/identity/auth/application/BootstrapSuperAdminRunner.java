package com.ecommerce.identity.auth.application;

import com.ecommerce.identity.common.config.IdentityBootstrapSuperAdminProperties;
import lombok.RequiredArgsConstructor;
import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.core.annotation.Order;
import org.springframework.core.Ordered;
import org.springframework.stereotype.Component;

@Component
@Order(Ordered.LOWEST_PRECEDENCE)
@RequiredArgsConstructor
public class BootstrapSuperAdminRunner implements ApplicationRunner {
    private final IdentityBootstrapSuperAdminProperties properties;
    private final BootstrapSuperAdminService service;

    @Override
    public void run(ApplicationArguments args) {
        service.ensureSuperAdmin(properties);
    }
}
