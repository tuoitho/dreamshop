package com.ecommerce.identity.rbac.persistence;

import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.identity.rbac.domain.AuthItemType;
import com.ecommerce.identity.rbac.domain.RbacItem;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Set;
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
        assertThat(repository.findItems(null))
                .extracting(RbacItem::name)
                .contains("CUSTOMER", "address.create", "cart.item.add");
        assertThat(repository.findItems(AuthItemType.ROLE))
                .extracting(RbacItem::name)
                .contains("CUSTOMER")
                .doesNotContain("address.create");
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
    void managesAuthorizationItemsAndHierarchyThroughRepositoryApi() {
        assertThat(repository.createItem("SUPPORT", AuthItemType.ROLE, "Support role", NOW)).isTrue();
        assertThat(repository.createItem(
                "ticket.view", AuthItemType.PERMISSION, "View tickets", NOW)).isTrue();
        assertThat(repository.addChild("SUPPORT", "ticket.view")).isTrue();

        AuthItemEntity item = repository.findById("ticket.view").orElseThrow();
        assertThat(item.getType()).isEqualTo(AuthItemType.PERMISSION);
        assertThat(repository.findDirectChildNames("SUPPORT")).containsExactly("ticket.view");
        assertThat(repository.wouldCreateCycle("ticket.view", "SUPPORT")).isTrue();

        assertThat(repository.updateItem(
                "ticket.view", "ticket.audit", "Audit tickets", NOW.plusSeconds(1))).isTrue();
        assertThat(repository.findById("ticket.audit").orElseThrow().getDescription())
                .isEqualTo("Audit tickets");
        assertThat(repository.removeChild("SUPPORT", "ticket.audit")).isTrue();
        assertThat(repository.deleteCustomItem("ticket.audit")).isTrue();
        assertThat(repository.deleteCustomItem("SUPPORT")).isTrue();
    }

    @Test
    @Transactional
    void acquiresRbacAdvisoryLocksThroughRepositoryQueries() {
        repository.lockHierarchy();
        repository.lockSuperAdminMutation();

        assertThat(repository.countActiveSuperAdmins()).isZero();
    }

    @Test
    void loadsAssignedRolesAndDirectChildrenInBatchQueries() {
        UUID otherUserId = UUID.fromString("00000000-0000-0000-0000-000000000089");
        UUID userWithoutRolesId = UUID.fromString("00000000-0000-0000-0000-000000000090");
        jdbc.update("""
                insert into identity_users
                    (id, email, normalized_email, password_hash, status, created_at, updated_at)
                values
                    (?, 'other-rbac@example.test', 'other-rbac@example.test', 'hash', 'ACTIVE', now(), now()),
                    (?, 'none-rbac@example.test', 'none-rbac@example.test', 'hash', 'ACTIVE', now(), now())
                """, otherUserId, userWithoutRolesId);
        repository.assignRole(USER_ID, "CUSTOMER", NOW);
        repository.assignRole(USER_ID, "ADMIN", NOW);
        repository.assignRole(otherUserId, "CUSTOMER", NOW);

        Map<UUID, Set<String>> rolesByUser = repository.assignedRolesByUserIds(
                List.of(USER_ID, otherUserId, userWithoutRolesId));
        assertThat(rolesByUser.get(USER_ID)).containsExactly("ADMIN", "CUSTOMER");
        assertThat(rolesByUser.get(otherUserId)).containsExactly("CUSTOMER");
        assertThat(rolesByUser).doesNotContainKey(userWithoutRolesId);
        assertThat(repository.assignedRolesByUserIds(List.of())).isEmpty();

        Map<String, Set<String>> childrenByParent = repository.findDirectChildrenByParentNames(
                List.of("CUSTOMER", "address.create"));
        assertThat(childrenByParent.get("CUSTOMER"))
                .contains("address.view", "address.create", "cart.item.add");
        assertThat(childrenByParent).doesNotContainKey("address.create");
        assertThat(repository.findDirectChildrenByParentNames(List.of())).isEmpty();
    }

    @Test
    void createsForeignKeyIndexesForAuthAssignmentsAndItemChildren() {
        List<String> indexNames = jdbc.queryForList("""
                select indexname
                from pg_indexes
                where schemaname = 'public'
                  and tablename in ('auth_assignments', 'auth_item_children')
                """, String.class);

        assertThat(indexNames).contains(
                "ix_auth_assignments_item_name",
                "ix_auth_item_children_child_name");
    }
}
