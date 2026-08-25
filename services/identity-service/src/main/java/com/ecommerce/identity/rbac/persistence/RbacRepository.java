package com.ecommerce.identity.rbac.persistence;

import java.time.Instant;
import java.util.Set;
import java.util.UUID;
import lombok.RequiredArgsConstructor;
import org.springframework.jdbc.core.simple.JdbcClient;
import org.springframework.stereotype.Repository;

@Repository
@RequiredArgsConstructor
public class RbacRepository {
    private final JdbcClient jdbc;

    public Set<String> assignedRoles(UUID userId) {
        return Set.copyOf(jdbc.sql("""
                select i.name
                from auth_assignments a
                join auth_items i on i.name = a.item_name and i.type = 'ROLE'
                where a.user_id = :userId
                order by i.name
                """)
                .param("userId", userId)
                .query(String.class)
                .list());
    }

    public boolean hasPermission(UUID userId, String permission) {
        Boolean result = jdbc.sql("""
                with recursive reachable(name) as (
                    select a.item_name
                    from auth_assignments a
                    join identity_users u on u.id = a.user_id and u.status = 'ACTIVE'
                    where a.user_id = :userId
                    union
                    select c.child_name
                    from auth_item_children c
                    join reachable r on r.name = c.parent_name
                )
                select exists (
                    select 1 from reachable where name = 'SUPER_ADMIN'
                ) or exists (
                    select 1
                    from reachable r
                    join auth_items i on i.name = r.name and i.type = 'PERMISSION'
                    where r.name = :permission
                )
                """)
                .param("userId", userId)
                .param("permission", permission)
                .query(Boolean.class)
                .single();
        return Boolean.TRUE.equals(result);
    }

    public boolean roleExists(String roleName) {
        return jdbc.sql("""
                select exists (
                    select 1 from auth_items where name = :roleName and type = 'ROLE'
                )
                """)
                .param("roleName", roleName)
                .query(Boolean.class)
                .single();
    }

    public boolean assignRole(UUID userId, String roleName, Instant createdAt) {
        return jdbc.sql("""
                insert into auth_assignments (user_id, item_name, created_at)
                select :userId, name, :createdAt
                from auth_items
                where name = :roleName and type = 'ROLE'
                on conflict (user_id, item_name) do nothing
                """)
                .param("userId", userId)
                .param("roleName", roleName)
                .param("createdAt", java.sql.Timestamp.from(createdAt))
                .update() > 0;
    }
}
