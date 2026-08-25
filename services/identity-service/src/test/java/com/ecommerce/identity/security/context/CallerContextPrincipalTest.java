package com.ecommerce.identity.security.context;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.Set;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.security.core.GrantedAuthority;

class CallerContextPrincipalTest {
    @Test
    void exposesRolesAsSpringRoleAuthorities() {
        CallerContextPrincipal principal = new CallerContextPrincipal(
                UUID.randomUUID(), Set.of("CUSTOMER", "ADMIN"));

        assertThat(principal.getAuthorities())
                .extracting(GrantedAuthority::getAuthority)
                .containsExactly("ROLE_ADMIN", "ROLE_CUSTOMER");
    }

    @Test
    void copiesRolesSoCallerCannotMutatePrincipalState() {
        Set<String> roles = new java.util.HashSet<>(Set.of("CUSTOMER"));
        CallerContextPrincipal principal = new CallerContextPrincipal(UUID.randomUUID(), roles);
        roles.add("ADMIN");

        assertThat(principal.roles()).containsExactly("CUSTOMER");
    }
}
