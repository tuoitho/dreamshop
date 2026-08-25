package com.ecommerce.identity.user.admin.application;

import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.rbac.domain.AuthItemType;
import com.ecommerce.identity.rbac.domain.RbacItem;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Optional;
import java.util.Set;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.mockito.InOrder;

class AdminUserServiceConcurrencyGuardTest {
    private static final Instant NOW = Instant.parse("2026-01-01T00:00:00Z");
    private static final UUID ACTOR_ID = UUID.randomUUID();
    private static final UUID TARGET_ID = UUID.randomUUID();

    private final IdentityUserRepository users = mock(IdentityUserRepository.class);
    private final RbacRepository rbac = mock(RbacRepository.class);
    private final IdentityUserEntity target = mock(IdentityUserEntity.class);
    private final AdminUserService service = new AdminUserService(
            users, rbac, Clock.fixed(NOW, ZoneOffset.UTC));

    @Test
    void locksLastSuperAdminGuardBeforeDisablingUser() {
        givenActiveSuperAdmin();
        when(rbac.activeSuperAdminCount()).thenReturn(2L);

        service.changeStatus(ACTOR_ID, TARGET_ID, AccountStatus.DISABLED);

        InOrder order = inOrder(rbac, users);
        order.verify(rbac).lockSuperAdminMutation();
        order.verify(users).findForUpdateById(TARGET_ID);
        order.verify(rbac).isSuperAdmin(TARGET_ID);
        order.verify(rbac).activeSuperAdminCount();
    }

    @Test
    void locksLastSuperAdminGuardBeforeRevokingRole() {
        givenActiveSuperAdmin();
        when(rbac.findItem("SUPER_ADMIN")).thenReturn(Optional.of(new RbacItem(
                "SUPER_ADMIN", AuthItemType.ROLE, "", true, NOW, NOW)));
        when(rbac.isSuperAdmin(ACTOR_ID)).thenReturn(true);
        when(rbac.activeSuperAdminCount()).thenReturn(2L);

        service.revokeRole(ACTOR_ID, TARGET_ID, "SUPER_ADMIN");

        InOrder order = inOrder(rbac, users);
        order.verify(rbac).lockSuperAdminMutation();
        order.verify(users).findForUpdateById(TARGET_ID);
        order.verify(rbac).findItem("SUPER_ADMIN");
        order.verify(rbac).isSuperAdmin(ACTOR_ID);
        order.verify(rbac).isSuperAdmin(TARGET_ID);
        order.verify(rbac).activeSuperAdminCount();
    }

    private void givenActiveSuperAdmin() {
        when(users.findForUpdateById(TARGET_ID)).thenReturn(Optional.of(target));
        when(target.id()).thenReturn(TARGET_ID);
        when(target.email()).thenReturn("target@example.com");
        when(target.fullName()).thenReturn("Target");
        when(target.status()).thenReturn(AccountStatus.ACTIVE);
        when(target.createdAt()).thenReturn(NOW);
        when(target.updatedAt()).thenReturn(NOW);
        when(rbac.isSuperAdmin(TARGET_ID)).thenReturn(true);
        when(rbac.assignedRoles(TARGET_ID)).thenReturn(Set.of("SUPER_ADMIN"));
    }
}
