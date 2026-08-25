package com.ecommerce.identity.user.admin.application;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.rbac.domain.AuthItemType;
import com.ecommerce.identity.rbac.domain.RbacHierarchyConflictException;
import com.ecommerce.identity.rbac.domain.RbacItem;
import com.ecommerce.identity.rbac.domain.RbacItemNotFoundException;
import com.ecommerce.identity.rbac.domain.RbacPermissions;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.user.admin.domain.LastSuperAdminRequiredException;
import com.ecommerce.identity.user.admin.domain.SelfDisableNotAllowedException;
import com.ecommerce.identity.user.admin.domain.SuperAdminAssignmentForbiddenException;
import com.ecommerce.identity.user.domain.UserNotFoundException;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Clock;
import java.util.Set;
import java.util.UUID;
import lombok.RequiredArgsConstructor;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
@RequiredArgsConstructor
public class AdminUserService {
    private final IdentityUserRepository users;
    private final RbacRepository rbac;
    private final Clock clock;

    @Transactional(readOnly = true)
    public AdminUserPage list(String rawQuery, int page, int pageSize) {
        String query = rawQuery == null ? "" : rawQuery.trim().toLowerCase(java.util.Locale.ROOT);
        PageRequest pageable = PageRequest.of(page - 1, pageSize,
                Sort.by(Sort.Order.desc("createdAt"), Sort.Order.asc("id")));
        Page<IdentityUserEntity> result = users.searchAdminUsers(query, pageable);
        return new AdminUserPage(result.getContent().stream().map(this::view).toList(), page, pageSize,
                result.getTotalElements(), result.getTotalPages());
    }

    @Transactional(readOnly = true)
    public AdminUserView get(UUID userId) {
        return view(users.findWithProfileById(userId).orElseThrow(UserNotFoundException::new));
    }

    @Transactional
    public AdminUserView changeStatus(UUID actorId, UUID userId, AccountStatus status) {
        IdentityUserEntity user = users.findForUpdateById(userId).orElseThrow(UserNotFoundException::new);
        if (status == AccountStatus.DISABLED && actorId.equals(userId)) {
            throw new SelfDisableNotAllowedException();
        }
        if (status == AccountStatus.DISABLED
                && user.status() == AccountStatus.ACTIVE
                && rbac.isSuperAdmin(userId)
                && rbac.activeSuperAdminCount() <= 1) {
            throw new LastSuperAdminRequiredException();
        }
        user.changeStatus(status, clock.instant());
        return view(user);
    }

    @Transactional
    public AdminUserView assignRole(UUID actorId, UUID userId, String rawRoleName) {
        IdentityUserEntity user = users.findForUpdateById(userId).orElseThrow(UserNotFoundException::new);
        RbacItem role = role(rawRoleName);
        if (RbacPermissions.SUPER_ADMIN.equals(role.name()) && !rbac.isSuperAdmin(actorId)) {
            throw new SuperAdminAssignmentForbiddenException();
        }
        rbac.assignRole(user.id(), role.name(), clock.instant());
        return view(user);
    }

    @Transactional
    public AdminUserView revokeRole(UUID actorId, UUID userId, String rawRoleName) {
        IdentityUserEntity user = users.findForUpdateById(userId).orElseThrow(UserNotFoundException::new);
        RbacItem role = role(rawRoleName);
        if (RbacPermissions.SUPER_ADMIN.equals(role.name())) {
            if (!rbac.isSuperAdmin(actorId)) {
                throw new SuperAdminAssignmentForbiddenException();
            }
            if (rbac.isSuperAdmin(userId) && rbac.activeSuperAdminCount() <= 1) {
                throw new LastSuperAdminRequiredException();
            }
        }
        rbac.revokeRole(user.id(), role.name());
        return view(user);
    }

    private RbacItem role(String rawRoleName) {
        String roleName = rawRoleName == null ? null : rawRoleName.trim();
        RbacItem item = rbac.findItem(roleName).orElseThrow(RbacItemNotFoundException::new);
        if (item.type() != AuthItemType.ROLE) {
            throw new RbacHierarchyConflictException();
        }
        return item;
    }

    private AdminUserView view(IdentityUserEntity user) {
        Set<String> roles = rbac.assignedRoles(user.id());
        return new AdminUserView(user.id(), user.email(), user.fullName(), user.status(), roles,
                user.createdAt(), user.updatedAt());
    }
}
