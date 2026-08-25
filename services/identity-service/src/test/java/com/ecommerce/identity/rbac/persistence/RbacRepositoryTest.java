package com.ecommerce.identity.rbac.persistence;

import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.identity.rbac.domain.AuthItemType;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.annotation.Transactional;

class RbacRepositoryTest extends PostgresIntegrationTestSupport {
    private static final UUID USER_ID = UUID.fromString("00000000-0000-0000-0000-000000000088");
    private static final Instant NOW = Instant.parse("2026-08-25T00:00:00Z");

    @Autowired
    JdbcTemplate jdbc;

    @Autowired
    RbacRepository repository;

    @BeforeEach
    void reset() {
        jdbc.update("delete from auth_assignments");
        jdbc.update("delete from identity_users");
        jdbc.update("delete from auth_items where system_item = false");
        jdbc.update("""
                insert into identity_users
                    (id, email, normalized_email, password_hash, status, created_at, updated_at)
                values (?, 'jpa-rbac@example.test', 'jpa-rbac@example.test', 'hash', 'ACTIVE', now(), now())
                """, USER_ID);
    }

    @Test
    void usesDerivedQueriesForSimpleAuthorizationItemReads() {
        assertThat(repository.existsByNameAndType("CUSTOMER", AuthItemType.ROLE)).isTrue();
        assertThat(repository.findAllByTypeOrderByName(AuthItemType.PERMISSION))
                .extracting(AuthItemEntity::getName)
                .contains("address.create", "cart.item.add");
        assertThat(repository.findById("CUSTOMER")).isPresent();
    }

    @Test
    void resolvesAssignmentsAndCurrentPermissionsWithNativeQueries() {
        assertThat(repository.insertAssignment(USER_ID, "CUSTOMER", NOW)).isEqualTo(1);

        assertThat(repository.findAssignedRoleNames(USER_ID)).containsExactly("CUSTOMER");
        assertThat(repository.findAllowedPermissionNames(
                USER_ID, List.of("address.create", "rbac.manage", "missing.permission")))
                .containsExactly("address.create");
        assertThat(repository.deleteAssignment(USER_ID, "CUSTOMER")).isEqualTo(1);
        assertThat(repository.findAllowedPermissionNames(USER_ID, List.of("address.create")))
                .isEmpty();
    }

    @Test
    @Transactional
    void managesAuthorizationItemsAndHierarchyWithNativeQueries() {
        assertThat(repository.insertItem("SUPPORT", "ROLE", "Support role", NOW)).isEqualTo(1);
        assertThat(repository.insertItem("ticket.view", "PERMISSION", "View tickets", NOW)).isEqualTo(1);
        assertThat(repository.insertChild("SUPPORT", "ticket.view")).isEqualTo(1);

        AuthItemEntity item = repository.findById("ticket.view").orElseThrow();
        assertThat(item.getType()).isEqualTo(AuthItemType.PERMISSION);
        assertThat(repository.findDirectChildNames("SUPPORT")).containsExactly("ticket.view");
        assertThat(repository.wouldCreateCycle("ticket.view", "SUPPORT")).isTrue();

        assertThat(repository.updateItem(
                "ticket.view", "ticket.audit", "Audit tickets", NOW.plusSeconds(1))).isTrue();
        assertThat(repository.findById("ticket.audit").orElseThrow().getDescription())
                .isEqualTo("Audit tickets");
        assertThat(repository.deleteChild("SUPPORT", "ticket.audit")).isEqualTo(1);
        assertThat(repository.deleteByNameAndSystemItemFalse("ticket.audit")).isEqualTo(1);
        assertThat(repository.deleteByNameAndSystemItemFalse("SUPPORT")).isEqualTo(1);
    }

    @Test
    @Transactional
    void acquiresRbacAdvisoryLocksThroughRepositoryQueries() {
        repository.lockHierarchy();
        repository.lockSuperAdminMutation();

        assertThat(repository.countActiveSuperAdmins()).isZero();
    }
}
