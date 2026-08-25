package com.ecommerce.identity.rbac.application;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.when;

import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.security.context.CallerContextPrincipal;
import java.util.List;
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
        when(repository.findAllowedPermissions(userId, List.of("address.create")))
                .thenReturn(Set.of("address.create"));

        assertThat(service.can(new CallerContextPrincipal(userId, Set.of("CUSTOMER")),
                "address.create")).isTrue();
    }

    @Test
    void checksPermissionForExplicitUserId() {
        when(repository.findAllowedPermissions(userId, List.of("cart.item.add")))
                .thenReturn(Set.of("cart.item.add"));

        assertThat(service.can(userId, "cart.item.add")).isTrue();
    }

    @Test
    void returnsAllowedSubsetForBatchWithoutLosingInputContract() {
        List<String> requested = List.of("cart.item.add", "cart.item.remove");
        when(repository.findAllowedPermissions(userId, requested)).thenReturn(Set.of("cart.item.add"));

        assertThat(service.allowedPermissions(userId, requested)).containsExactly("cart.item.add");
    }

    @Test
    void deniesUnknownAuthenticationAndBlankPermission() {
        Authentication unrelated = new UsernamePasswordAuthenticationToken("user", "credentials");

        assertThat(service.can(unrelated, "address.create")).isFalse();
        assertThat(service.can((Authentication) null, "address.create")).isFalse();
        assertThat(service.can(new CallerContextPrincipal(userId, Set.of("CUSTOMER")), " ")).isFalse();
        assertThat(service.can(new CallerContextPrincipal(userId, Set.of("CUSTOMER")), null)).isFalse();
    }
}
