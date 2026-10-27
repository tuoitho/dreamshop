package com.ecommerce.identity.user.admin.application;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageImpl;
import org.springframework.data.domain.Pageable;

class AdminUserServiceTest {
    private static final Instant NOW = Instant.parse("2026-08-25T00:00:00Z");
    private static final UUID USER_1 = UUID.fromString("00000000-0000-0000-0000-000000000101");
    private static final UUID USER_2 = UUID.fromString("00000000-0000-0000-0000-000000000102");
    private static final UUID USER_3 = UUID.fromString("00000000-0000-0000-0000-000000000103");

    private final IdentityUserRepository users = mock(IdentityUserRepository.class);
    private final RbacRepository rbac = mock(RbacRepository.class);
    private final AdminUserService service = new AdminUserService(
            users, rbac, Clock.fixed(NOW, ZoneOffset.UTC));

    @Test
    void listLoadsAssignedRolesInSingleBatchQueryWithoutPerUserQueries() {
        IdentityUserEntity u1 = mockUser(USER_1, "u1@example.com", "User One");
        IdentityUserEntity u2 = mockUser(USER_2, "u2@example.com", "User Two");
        IdentityUserEntity u3 = mockUser(USER_3, "u3@example.com", "User Three");
        when(users.searchAdminUsers(any(String.class), any(), any(Pageable.class)))
                .thenReturn(new PageImpl<>(List.of(u1, u2, u3)));
        when(rbac.assignedRolesByUserIds(List.of(USER_1, USER_2, USER_3)))
                .thenReturn(Map.of(
                        USER_1, Set.of("ADMIN"),
                        USER_2, Set.of("CUSTOMER", "SUPPORT")));

        AdminUserPage page = service.list("", 1, 20);

        assertThat(page.items()).hasSize(3);
        assertThat(page.items().get(0).roles()).containsExactly("ADMIN");
        assertThat(page.items().get(1).roles()).containsExactlyInAnyOrder("CUSTOMER", "SUPPORT");
        assertThat(page.items().get(2).roles()).isEmpty();
        verify(rbac).assignedRolesByUserIds(List.of(USER_1, USER_2, USER_3));
        verify(rbac, never()).assignedRoles(any(UUID.class));
    }

    @Test
    void listSkipsBatchRoleQueryWhenPageIsEmpty() {
        when(users.searchAdminUsers(any(String.class), any(), any(Pageable.class)))
                .thenReturn(Page.empty());
        when(users.countByStatus(AccountStatus.ACTIVE)).thenReturn(4L);
        when(users.countByStatus(AccountStatus.DISABLED)).thenReturn(2L);

        AdminUserPage page = service.list("", AccountStatus.DISABLED, 1, 20);

        assertThat(page.items()).isEmpty();
        assertThat(page.activeCount()).isEqualTo(4L);
        assertThat(page.disabledCount()).isEqualTo(2L);
        verify(rbac, never()).assignedRolesByUserIds(any());
        verify(rbac, never()).assignedRoles(any(UUID.class));
    }

    private static IdentityUserEntity mockUser(UUID id, String email, String fullName) {
        IdentityUserEntity user = mock(IdentityUserEntity.class);
        when(user.id()).thenReturn(id);
        when(user.email()).thenReturn(email);
        when(user.fullName()).thenReturn(fullName);
        when(user.status()).thenReturn(AccountStatus.ACTIVE);
        when(user.createdAt()).thenReturn(NOW);
        when(user.updatedAt()).thenReturn(NOW);
        return user;
    }
}
