package com.ecommerce.identity.auth.application;

import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.ecommerce.identity.common.config.IdentityBootstrapSuperAdminProperties;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Set;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@ExtendWith(MockitoExtension.class)
class BootstrapSuperAdminServiceTest {
    private static final UUID USER_ID = UUID.fromString("00000000-0000-0000-0000-000000000099");
    private static final Instant NOW = Instant.parse("2026-08-25T00:00:00Z");

    @Mock
    private IdentityUserRepository users;

    @Mock
    private RegisterUserService registration;

    @Mock
    private RbacRepository rbac;

    private BootstrapSuperAdminService service;

    @BeforeEach
    void setUp() {
        service = new BootstrapSuperAdminService(
                users, registration, rbac, Clock.fixed(NOW, ZoneOffset.UTC));
    }

    @Test
    void createsAndPromotesMissingBootstrapUser() {
        IdentityBootstrapSuperAdminProperties properties = properties();
        when(users.existsByNormalizedEmail("root@example.test")).thenReturn(false);
        when(registration.register(any(RegisterCommand.class)))
                .thenReturn(new RegisteredUser(USER_ID, "root@example.test", "Root Admin", Set.of("CUSTOMER")));
        when(rbac.assignRole(USER_ID, "SUPER_ADMIN", NOW)).thenReturn(true);

        service.ensureSuperAdmin(properties);

        verify(registration).register(new RegisterCommand(
                "root@example.test", "SecurePassword123!", "Root Admin"));
        verify(rbac).assignRole(USER_ID, "SUPER_ADMIN", NOW);
    }

    @Test
    void existingEmailIsAnIdempotentNoOpWithoutPasswordOrRoleChanges() {
        when(users.existsByNormalizedEmail("root@example.test")).thenReturn(true);

        service.ensureSuperAdmin(properties());

        verify(registration, never()).register(any(RegisterCommand.class));
        verify(rbac, never()).assignRole(any(UUID.class), any(String.class), any(Instant.class));
    }

    @Test
    void failsWhenSuperAdminRoleCannotBeAssigned() {
        when(users.existsByNormalizedEmail("root@example.test")).thenReturn(false);
        when(registration.register(any(RegisterCommand.class)))
                .thenReturn(new RegisteredUser(USER_ID, "root@example.test", "Root Admin", Set.of("CUSTOMER")));
        when(rbac.assignRole(USER_ID, "SUPER_ADMIN", NOW)).thenReturn(false);

        assertThatThrownBy(() -> service.ensureSuperAdmin(properties()))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("SUPER_ADMIN");
    }

    private IdentityBootstrapSuperAdminProperties properties() {
        return new IdentityBootstrapSuperAdminProperties(
                "root@example.test", "SecurePassword123!", "Root Admin");
    }
}
