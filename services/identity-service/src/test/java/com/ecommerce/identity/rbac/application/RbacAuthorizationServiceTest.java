package com.ecommerce.identity.rbac.application;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.when;

import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.security.CallerContextPrincipal;
import java.util.Set;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;
import org.springframework.security.core.Authentication;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;

class RbacAuthorizationServiceTest {
    private final RbacRepository repository = Mockito.mock(RbacRepository.class);
    private final RbacAuthorizationService service = new RbacAuthorizationService(repository);
    private final UUID userId = UUID.randomUUID();

    @Test
    void delegatesPermissionCheckToCurrentDatabaseState() {
        when(repository.hasPermission(userId, "address.create")).thenReturn(true);

        assertThat(service.can(new CallerContextPrincipal(userId, Set.of("CUSTOMER")),
                "address.create")).isTrue();
    }

    @Test
    void deniesUnknownAuthenticationAndBlankPermission() {
        Authentication unrelated = new UsernamePasswordAuthenticationToken("user", "credentials");

        assertThat(service.can(unrelated, "address.create")).isFalse();
        assertThat(service.can(null, "address.create")).isFalse();
        assertThat(service.can(new CallerContextPrincipal(userId, Set.of("CUSTOMER")), " ")).isFalse();
        assertThat(service.can(new CallerContextPrincipal(userId, Set.of("CUSTOMER")), null)).isFalse();
    }
}
