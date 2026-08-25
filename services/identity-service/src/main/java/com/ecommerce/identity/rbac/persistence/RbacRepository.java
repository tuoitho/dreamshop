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
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;

@Repository
@RequiredArgsConstructor
public class RbacRepository {
    private final RbacJpaRepository repository;

    public Set<String> assignedRoles(UUID userId) {
        return Set.copyOf(repository.findAssignedRoles(userId));
    }

    public boolean hasPermission(UUID userId, String permission) {
        return findAllowedPermissions(userId, List.of(permission)).contains(permission);
    }

    public Set<String> findAllowedPermissions(UUID userId, Collection<String> permissions) {
        if (permissions.isEmpty()) {
            return Set.of();
        }
        return Collections.unmodifiableSet(new LinkedHashSet<>(
                repository.findAllowedPermissionNames(userId, permissions)));
    }

    public boolean roleExists(String roleName) {
        return repository.existsRole(roleName);
    }

    public boolean assignRole(UUID userId, String roleName, Instant createdAt) {
        return repository.insertAssignment(userId, roleName, createdAt) > 0;
    }

    public boolean revokeRole(UUID userId, String roleName) {
        return repository.deleteAssignment(userId, roleName) > 0;
    }

    public boolean isSuperAdmin(UUID userId) {
        return repository.isActiveSuperAdmin(userId);
    }

    public long activeSuperAdminCount() {
        return repository.countActiveSuperAdmins();
    }

    public List<RbacItem> findItems(AuthItemType type) {
        List<AuthItemEntity> items = type == null
                ? repository.findAllItems()
                : repository.findItemsByType(type.name());
        return items.stream().map(AuthItemEntity::toDomain).toList();
    }

    public Optional<RbacItem> findItem(String name) {
        return repository.findItem(name).map(AuthItemEntity::toDomain);
    }

    public Set<String> findDirectChildren(String parentName) {
        return Set.copyOf(repository.findDirectChildren(parentName));
    }

    public boolean createItem(String name, AuthItemType type, String description, Instant now) {
        return repository.insertItem(name, type.name(), description, now) > 0;
    }

    public boolean updateItem(String currentName, String newName, String description, Instant now) {
        return repository.updateItem(currentName, newName, description, now) > 0;
    }

    public void lockHierarchy() {
        repository.lockHierarchy();
    }

    public void lockSuperAdminMutation() {
        repository.lockSuperAdminMutation();
    }

    public boolean deleteCustomItem(String name) {
        return repository.deleteCustomItem(name) > 0;
    }

    public boolean wouldCreateCycle(String parentName, String childName) {
        return repository.wouldCreateCycle(parentName, childName);
    }

    public boolean addChild(String parentName, String childName) {
        return repository.insertChild(parentName, childName) > 0;
    }

    public boolean removeChild(String parentName, String childName) {
        return repository.deleteChild(parentName, childName) > 0;
    }
}
