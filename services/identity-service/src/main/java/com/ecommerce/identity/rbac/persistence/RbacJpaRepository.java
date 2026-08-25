package com.ecommerce.identity.rbac.persistence;

import java.time.Instant;
import java.util.Collection;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.Repository;
import org.springframework.data.repository.query.Param;
import org.springframework.transaction.annotation.Transactional;

public interface RbacJpaRepository extends Repository<AuthItemEntity, String> {
    @Query(value = """
            select i.name
            from auth_assignments a
            join auth_items i on i.name = a.item_name and i.type = 'ROLE'
            where a.user_id = :userId
            order by i.name
            """, nativeQuery = true)
    List<String> findAssignedRoles(@Param("userId") UUID userId);

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

    @Query(value = """
            select exists (
                select 1 from auth_items where name = :roleName and type = 'ROLE'
            )
            """, nativeQuery = true)
    boolean existsRole(@Param("roleName") String roleName);

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
            select name, type, description, system_item, created_at, updated_at
            from auth_items
            order by type, name
            """, nativeQuery = true)
    List<AuthItemEntity> findAllItems();

    @Query(value = """
            select name, type, description, system_item, created_at, updated_at
            from auth_items
            where type = :type
            order by name
            """, nativeQuery = true)
    List<AuthItemEntity> findItemsByType(@Param("type") String type);

    @Query(value = """
            select name, type, description, system_item, created_at, updated_at
            from auth_items
            where name = :name
            """, nativeQuery = true)
    Optional<AuthItemEntity> findItem(@Param("name") String name);

    @Query(value = """
            select child_name
            from auth_item_children
            where parent_name = :parentName
            order by child_name
            """, nativeQuery = true)
    List<String> findDirectChildren(@Param("parentName") String parentName);

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
    int updateItem(
            @Param("currentName") String currentName,
            @Param("newName") String newName,
            @Param("description") String description,
            @Param("updatedAt") Instant updatedAt);

    @Query(value = "select pg_advisory_xact_lock(7242221)", nativeQuery = true)
    void lockHierarchy();

    @Query(value = "select pg_advisory_xact_lock(7242222)", nativeQuery = true)
    void lockSuperAdminMutation();

    @Modifying(clearAutomatically = true, flushAutomatically = true)
    @Transactional
    @Query(value = """
            delete from auth_items
            where name = :name and system_item = false
            """, nativeQuery = true)
    int deleteCustomItem(@Param("name") String name);

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
}
