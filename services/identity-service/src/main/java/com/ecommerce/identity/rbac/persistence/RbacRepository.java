package com.ecommerce.identity.rbac.persistence;

import com.ecommerce.identity.rbac.domain.AuthItemType;
import com.ecommerce.identity.rbac.domain.RbacItem;
import java.time.Instant;
import java.util.Collection;
import java.util.Collections;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import java.util.UUID;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.transaction.annotation.Transactional;

public interface RbacRepository extends JpaRepository<AuthItemEntity, String> {
    boolean existsByNameAndType(String name, AuthItemType type);

    List<AuthItemEntity> findAllByOrderByTypeAscNameAsc();

    List<AuthItemEntity> findAllByTypeOrderByName(AuthItemType type);

    @Transactional
    long deleteByNameAndSystemItemFalse(String name);

    @Query(value = """
            select i.name
            from auth_assignments a
            join auth_items i on i.name = a.item_name and i.type = 'ROLE'
            where a.user_id = :userId
            order by i.name
            """, nativeQuery = true)
    List<String> findAssignedRoleNames(@Param("userId") UUID userId);

    @Query(value = """
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
            select target.name
            from auth_items target
            where target.type = 'PERMISSION'
              and target.name in (:permissions)
              and (
                  exists (select 1 from reachable where name = 'SUPER_ADMIN')
                  or exists (select 1 from reachable where name = target.name)
              )
            order by target.name
            """, nativeQuery = true)
    List<String> findAllowedPermissionNames(
            @Param("userId") UUID userId,
            @Param("permissions") Collection<String> permissions);

    @Modifying
    @Transactional
    @Query(value = """
            insert into auth_assignments (user_id, item_name, created_at)
            select :userId, name, :createdAt
            from auth_items
            where name = :roleName and type = 'ROLE'
            on conflict (user_id, item_name) do nothing
            """, nativeQuery = true)
    int insertAssignment(
            @Param("userId") UUID userId,
            @Param("roleName") String roleName,
            @Param("createdAt") Instant createdAt);

    @Modifying
    @Transactional
    @Query(value = """
            delete from auth_assignments a
            using auth_items i
            where a.user_id = :userId
              and a.item_name = i.name
              and i.type = 'ROLE'
              and a.item_name = :roleName
            """, nativeQuery = true)
    int deleteAssignment(@Param("userId") UUID userId, @Param("roleName") String roleName);

    @Query(value = """
            select exists (
                select 1
                from auth_assignments a
                join identity_users u on u.id = a.user_id and u.status = 'ACTIVE'
                where a.user_id = :userId and a.item_name = 'SUPER_ADMIN'
            )
            """, nativeQuery = true)
    boolean isActiveSuperAdmin(@Param("userId") UUID userId);

    @Query(value = """
            select count(distinct a.user_id)
            from auth_assignments a
            join identity_users u on u.id = a.user_id and u.status = 'ACTIVE'
            where a.item_name = 'SUPER_ADMIN'
            """, nativeQuery = true)
    long countActiveSuperAdmins();

    @Query(value = """
            select child_name
            from auth_item_children
            where parent_name = :parentName
            order by child_name
            """, nativeQuery = true)
    List<String> findDirectChildNames(@Param("parentName") String parentName);

    @Modifying
    @Transactional
    @Query(value = """
            insert into auth_items (name, type, description, system_item, created_at, updated_at)
            values (:name, :type, :description, false, :createdAt, :updatedAt)
            on conflict (name) do nothing
            """, nativeQuery = true)
    int insertItem(
            @Param("name") String name,
            @Param("type") String type,
            @Param("description") String description,
            @Param("createdAt") Instant createdAt,
            @Param("updatedAt") Instant updatedAt);

    default int insertItem(String name, String type, String description, Instant now) {
        return insertItem(name, type, description, now, now);
    }

    @Modifying(clearAutomatically = true, flushAutomatically = true)
    @Transactional
    @Query(value = """
            update auth_items
            set name = :newName, description = :description, updated_at = :updatedAt
            where name = :currentName
              and (system_item = false or :currentName = :newName)
            """, nativeQuery = true)
    int updateItemRow(
            @Param("currentName") String currentName,
            @Param("newName") String newName,
            @Param("description") String description,
            @Param("updatedAt") Instant updatedAt);

    @Query(value = "select pg_advisory_xact_lock(7242221)", nativeQuery = true)
    void lockHierarchy();

    @Query(value = "select pg_advisory_xact_lock(7242222)", nativeQuery = true)
    void lockSuperAdminMutation();

    @Query(value = """
            with recursive descendants(name) as (
                select child_name from auth_item_children where parent_name = :childName
                union
                select c.child_name
                from auth_item_children c
                join descendants d on d.name = c.parent_name
            )
            select exists (select 1 from descendants where name = :parentName)
            """, nativeQuery = true)
    boolean wouldCreateCycle(
            @Param("parentName") String parentName,
            @Param("childName") String childName);

    @Modifying
    @Transactional
    @Query(value = """
            insert into auth_item_children (parent_name, child_name)
            values (:parentName, :childName)
            on conflict (parent_name, child_name) do nothing
            """, nativeQuery = true)
    int insertChild(
            @Param("parentName") String parentName,
            @Param("childName") String childName);

    @Modifying
    @Transactional
    @Query(value = """
            delete from auth_item_children
            where parent_name = :parentName and child_name = :childName
            """, nativeQuery = true)
    int deleteChild(
            @Param("parentName") String parentName,
            @Param("childName") String childName);

    default Set<String> assignedRoles(UUID userId) {
        return Set.copyOf(findAssignedRoleNames(userId));
    }

    default boolean hasPermission(UUID userId, String permission) {
        return findAllowedPermissions(userId, List.of(permission)).contains(permission);
    }

    default Set<String> findAllowedPermissions(UUID userId, Collection<String> permissions) {
        if (permissions.isEmpty()) {
            return Set.of();
        }
        return Collections.unmodifiableSet(new LinkedHashSet<>(
                findAllowedPermissionNames(userId, permissions)));
    }

    default boolean roleExists(String roleName) {
        return existsByNameAndType(roleName, AuthItemType.ROLE);
    }

    default boolean assignRole(UUID userId, String roleName, Instant createdAt) {
        return insertAssignment(userId, roleName, createdAt) > 0;
    }

    default boolean revokeRole(UUID userId, String roleName) {
        return deleteAssignment(userId, roleName) > 0;
    }

    default boolean isSuperAdmin(UUID userId) {
        return isActiveSuperAdmin(userId);
    }

    default long activeSuperAdminCount() {
        return countActiveSuperAdmins();
    }

    default List<RbacItem> findItems(AuthItemType type) {
        List<AuthItemEntity> items = type == null
                ? findAllByOrderByTypeAscNameAsc()
                : findAllByTypeOrderByName(type);
        return items.stream().map(AuthItemEntity::toDomain).toList();
    }

    default Optional<RbacItem> findItem(String name) {
        return findById(name).map(AuthItemEntity::toDomain);
    }

    default Set<String> findDirectChildren(String parentName) {
        return Set.copyOf(findDirectChildNames(parentName));
    }

    default boolean createItem(String name, AuthItemType type, String description, Instant now) {
        return insertItem(name, type.name(), description, now) > 0;
    }

    default boolean updateItem(String currentName, String newName, String description, Instant now) {
        return updateItemRow(currentName, newName, description, now) > 0;
    }

    default boolean deleteCustomItem(String name) {
        return deleteByNameAndSystemItemFalse(name) > 0;
    }

    default boolean addChild(String parentName, String childName) {
        return insertChild(parentName, childName) > 0;
    }

    default boolean removeChild(String parentName, String childName) {
        return deleteChild(parentName, childName) > 0;
    }
}
