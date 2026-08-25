package com.ecommerce.identity.rbac.persistence;

import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;

class RbacPersistenceTest extends PostgresIntegrationTestSupport {
    private static final UUID USER_ID = UUID.fromString("00000000-0000-0000-0000-000000000011");

    @Autowired
    JdbcTemplate jdbcTemplate;

    @Autowired
    RbacRepository repository;

    @BeforeEach
    void reset() {
        jdbcTemplate.update("delete from auth_assignments");
        jdbcTemplate.update("delete from identity_users");
        jdbcTemplate.update("delete from auth_item_children where parent_name = 'address.manage'");
        jdbcTemplate.update("delete from auth_items where name = 'address.manage'");
        jdbcTemplate.update("insert into identity_users "
                + "(id, email, normalized_email, password_hash, status, created_at, updated_at) "
                + "values (?, ?, ?, ?, 'ACTIVE', now(), now())", USER_ID, "rbac@example.com",
                "rbac@example.com", "hash");
        jdbcTemplate.update("insert into auth_items "
                + "(name, type, description, system_item, created_at, updated_at) "
                + "values ('address.manage', 'PERMISSION', 'Address parent', false, now(), now())");
        jdbcTemplate.update("insert into auth_item_children (parent_name, child_name) "
                + "values ('address.manage', 'address.create')");
    }

    @Test
    void resolvesAssignedRolesAndNestedPermissionsForActiveUser() {
        repository.assignRole(USER_ID, "CUSTOMER", java.time.Instant.now());

        assertThat(repository.assignedRoles(USER_ID)).containsExactly("CUSTOMER");
        assertThat(repository.hasPermission(USER_ID, "address.create")).isTrue();
        assertThat(repository.hasPermission(USER_ID, "rbac.manage")).isFalse();
    }

    @Test
    void deniesPermissionsForDisabledUser() {
        repository.assignRole(USER_ID, "CUSTOMER", java.time.Instant.now());
        jdbcTemplate.update("update identity_users set status = 'DISABLED' where id = ?", USER_ID);

        assertThat(repository.hasPermission(USER_ID, "address.create")).isFalse();
    }

    @Test
    void superAdminReceivesPermissionCreatedAfterAssignment() {
        repository.assignRole(USER_ID, "SUPER_ADMIN", java.time.Instant.now());
        jdbcTemplate.update("insert into auth_items "
                + "(name, type, description, system_item, created_at, updated_at) "
                + "values ('ticket.view', 'PERMISSION', 'View tickets', false, now(), now())");

        assertThat(repository.hasPermission(USER_ID, "ticket.view")).isTrue();
    }
}
