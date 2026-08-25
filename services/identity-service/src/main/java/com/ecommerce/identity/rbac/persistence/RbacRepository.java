package com.ecommerce.identity.rbac.persistence;

import com.ecommerce.identity.rbac.domain.AuthItemType;
import com.ecommerce.identity.rbac.domain.RbacItem;
import java.time.Instant;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.List;
import java.util.Optional;
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
                    select 1
                    from auth_items target
                    where target.name = :permission
                      and (
                          exists (select 1 from reachable where name = 'SUPER_ADMIN')
                          or exists (
                              select 1
                              from reachable r
                              join auth_items i on i.name = r.name and i.type = 'PERMISSION'
                              where r.name = :permission
                          )
                      )
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

    public List<RbacItem> findItems(AuthItemType type) {
        if (type == null) {
            return jdbc.sql("""
                    select name, type, description, system_item, created_at, updated_at
                    from auth_items order by type, name
                    """)
                    .query(this::toItem)
                    .list();
        }
        return jdbc.sql("""
                select name, type, description, system_item, created_at, updated_at
                from auth_items where type = :type order by name
                """)
                .param("type", type.name())
                .query(this::toItem)
                .list();
    }

    public Optional<RbacItem> findItem(String name) {
        return jdbc.sql("""
                select name, type, description, system_item, created_at, updated_at
                from auth_items where name = :name
                """)
                .param("name", name)
                .query(this::toItem)
                .optional();
    }

    public Set<String> findDirectChildren(String parentName) {
        return Set.copyOf(jdbc.sql("""
                select child_name from auth_item_children
                where parent_name = :parentName order by child_name
                """)
                .param("parentName", parentName)
                .query(String.class)
                .list());
    }

    public boolean createItem(String name, AuthItemType type, String description, Instant now) {
        return jdbc.sql("""
                insert into auth_items (name, type, description, system_item, created_at, updated_at)
                values (:name, :type, :description, false, :createdAt, :updatedAt)
                on conflict (name) do nothing
                """)
                .param("name", name)
                .param("type", type.name())
                .param("description", description)
                .param("createdAt", java.sql.Timestamp.from(now))
                .param("updatedAt", java.sql.Timestamp.from(now))
                .update() > 0;
    }

    public boolean updateItem(String currentName, String newName, String description, Instant now) {
        return jdbc.sql("""
                update auth_items
                set name = :newName, description = :description, updated_at = :updatedAt
                where name = :currentName
                  and (system_item = false or :currentName = :newName)
                """)
                .param("currentName", currentName)
                .param("newName", newName)
                .param("description", description)
                .param("updatedAt", java.sql.Timestamp.from(now))
                .update() > 0;
    }

    public void lockHierarchy() {
        jdbc.sql("select pg_advisory_xact_lock(7242221)")
                .query((resultSet, rowNum) -> resultSet.getObject(1))
                .list();
    }

    public boolean deleteCustomItem(String name) {
        return jdbc.sql("delete from auth_items where name = :name and system_item = false")
                .param("name", name)
                .update() > 0;
    }

    public boolean wouldCreateCycle(String parentName, String childName) {
        return jdbc.sql("""
                with recursive descendants(name) as (
                    select child_name from auth_item_children where parent_name = :childName
                    union
                    select c.child_name
                    from auth_item_children c
                    join descendants d on d.name = c.parent_name
                )
                select exists (select 1 from descendants where name = :parentName)
                """)
                .param("parentName", parentName)
                .param("childName", childName)
                .query(Boolean.class)
                .single();
    }

    public boolean addChild(String parentName, String childName) {
        return jdbc.sql("""
                insert into auth_item_children (parent_name, child_name)
                values (:parentName, :childName)
                on conflict (parent_name, child_name) do nothing
                """)
                .param("parentName", parentName)
                .param("childName", childName)
                .update() > 0;
    }

    public boolean removeChild(String parentName, String childName) {
        return jdbc.sql("""
                delete from auth_item_children
                where parent_name = :parentName and child_name = :childName
                """)
                .param("parentName", parentName)
                .param("childName", childName)
                .update() > 0;
    }

    private RbacItem toItem(ResultSet resultSet, int rowNum) throws SQLException {
        return new RbacItem(
                resultSet.getString("name"),
                AuthItemType.valueOf(resultSet.getString("type")),
                resultSet.getString("description"),
                resultSet.getBoolean("system_item"),
                resultSet.getTimestamp("created_at").toInstant(),
                resultSet.getTimestamp("updated_at").toInstant());
    }
}
