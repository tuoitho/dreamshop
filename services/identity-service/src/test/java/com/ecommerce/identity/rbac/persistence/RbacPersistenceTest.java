package com.ecommerce.identity.rbac.persistence;

import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.time.Instant;
import java.util.List;
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
        repository.assignRole(USER_ID, "CUSTOMER", Instant.now());

        assertThat(repository.assignedRoles(USER_ID)).containsExactly("CUSTOMER");
        assertThat(repository.findAllowedPermissions(USER_ID,
                List.of("address.create", "rbac.manage"))).containsExactly("address.create");
    }

    @Test
    void deniesPermissionsForDisabledUser() {
        repository.assignRole(USER_ID, "CUSTOMER", Instant.now());
        jdbcTemplate.update("update identity_users set status = 'DISABLED' where id = ?", USER_ID);

        assertThat(repository.findAllowedPermissions(USER_ID, List.of("address.create"))).isEmpty();
    }

    @Test
    void superAdminReceivesPermissionCreatedAfterAssignment() {
        repository.assignRole(USER_ID, "SUPER_ADMIN", Instant.now());
        jdbcTemplate.update("insert into auth_items "
                + "(name, type, description, system_item, created_at, updated_at) "
                + "values ('ticket.view', 'PERMISSION', 'View tickets', false, now(), now())");

        assertThat(repository.findAllowedPermissions(USER_ID, List.of("ticket.view")))
                .containsExactly("ticket.view");
    }

    @Test
    @org.springframework.transaction.annotation.Transactional
    void resolvesCurrentAllowedSubsetAndReflectsRevocationImmediately() {
        repository.assignRole(USER_ID, "CUSTOMER", Instant.now());

        assertThat(repository.findAllowedPermissions(USER_ID,
                List.of("cart.item.add", "rbac.manage", "missing.permission")))
                .containsExactly("cart.item.add");

        jdbcTemplate.update("delete from auth_item_children "
                + "where parent_name = 'CUSTOMER' and child_name = 'cart.item.add'");

        assertThat(repository.findAllowedPermissions(USER_ID, List.of("cart.item.add"))).isEmpty();
    }

    @Test
    void grantsProductDeleteToAdminAndSuperAdminButNotCustomer() {
        repository.assignRole(USER_ID, "CUSTOMER", Instant.now());
        assertThat(repository.findAllowedPermissions(USER_ID, List.of("product.delete"))).isEmpty();

        jdbcTemplate.update("delete from auth_assignments where user_id = ?", USER_ID);
        repository.assignRole(USER_ID, "ADMIN", Instant.now());
        assertThat(repository.findAllowedPermissions(USER_ID, List.of("product.delete")))
                .containsExactly("product.delete");

        jdbcTemplate.update("delete from auth_assignments where user_id = ?", USER_ID);
        repository.assignRole(USER_ID, "SUPER_ADMIN", Instant.now());
        assertThat(repository.findAllowedPermissions(USER_ID, List.of("product.delete")))
                .containsExactly("product.delete");
    }

    @Test
    void grantsFavoriteManageToCustomerAndSuperAdminButNotAdmin() {
        repository.assignRole(USER_ID, "CUSTOMER", Instant.now());
        assertThat(repository.findAllowedPermissions(USER_ID, List.of("favorite.manage")))
                .containsExactly("favorite.manage");

        jdbcTemplate.update("delete from auth_assignments where user_id = ?", USER_ID);
        repository.assignRole(USER_ID, "ADMIN", Instant.now());
        assertThat(repository.findAllowedPermissions(USER_ID, List.of("favorite.manage")))
                .isEmpty();

        jdbcTemplate.update("delete from auth_assignments where user_id = ?", USER_ID);
        repository.assignRole(USER_ID, "SUPER_ADMIN", Instant.now());
        assertThat(repository.findAllowedPermissions(USER_ID, List.of("favorite.manage")))
                .containsExactly("favorite.manage");
    }

    @Test
    void grantsCartPermissionsToCustomerAndSuperAdminButNotAdmin() {
        List<String> cartPermissions = List.of(
                "cart.view", "cart.item.add", "cart.item.update", "cart.item.remove", "cart.clear");

        repository.assignRole(USER_ID, "CUSTOMER", Instant.now());
        assertThat(repository.findAllowedPermissions(USER_ID, cartPermissions))
                .containsExactlyInAnyOrderElementsOf(cartPermissions);

        jdbcTemplate.update("delete from auth_assignments where user_id = ?", USER_ID);
        repository.assignRole(USER_ID, "ADMIN", Instant.now());
        assertThat(repository.findAllowedPermissions(USER_ID, cartPermissions))
                .isEmpty();

        jdbcTemplate.update("delete from auth_assignments where user_id = ?", USER_ID);
        repository.assignRole(USER_ID, "SUPER_ADMIN", Instant.now());
        assertThat(repository.findAllowedPermissions(USER_ID, cartPermissions))
                .containsExactlyInAnyOrderElementsOf(cartPermissions);
    }

    @Test
    void grantsOrderPermissionsToCustomerAndSuperAdminButNotAdmin() {
        List<String> orderPermissions = List.of("order.create", "order.view", "order.cancel");

        repository.assignRole(USER_ID, "CUSTOMER", Instant.now());
        assertThat(repository.findAllowedPermissions(USER_ID, orderPermissions))
                .containsExactlyInAnyOrderElementsOf(orderPermissions);

        jdbcTemplate.update("delete from auth_assignments where user_id = ?", USER_ID);
        repository.assignRole(USER_ID, "ADMIN", Instant.now());
        assertThat(repository.findAllowedPermissions(USER_ID, orderPermissions))
                .isEmpty();

        jdbcTemplate.update("delete from auth_assignments where user_id = ?", USER_ID);
        repository.assignRole(USER_ID, "SUPER_ADMIN", Instant.now());
        assertThat(repository.findAllowedPermissions(USER_ID, orderPermissions))
                .containsExactlyInAnyOrderElementsOf(orderPermissions);
    }

    @Test
    void findsAllEffectivePermissionsForActiveRolesAndReturnsEmptyWhenDisabled() {
        repository.assignRole(USER_ID, "ADMIN", Instant.now());
        assertThat(repository.findAllEffectivePermissions(USER_ID))
                .contains("product.view", "product.create", "product.update", "product.delete",
                        "inventory.view", "inventory.update", "user.view", "rbac.view")
                .doesNotContain("favorite.manage");

        jdbcTemplate.update("delete from auth_assignments where user_id = ?", USER_ID);
        repository.assignRole(USER_ID, "SUPER_ADMIN", Instant.now());
        assertThat(repository.findAllEffectivePermissions(USER_ID))
                .contains("product.view", "inventory.update", "favorite.manage", "address.manage");

        jdbcTemplate.update("update identity_users set status = 'DISABLED' where id = ?", USER_ID);
        assertThat(repository.findAllEffectivePermissions(USER_ID)).isEmpty();
    }
}
