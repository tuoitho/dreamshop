package com.ecommerce.identity.rbac.application;

import com.ecommerce.identity.rbac.domain.AuthItemType;
import com.ecommerce.identity.rbac.domain.InvalidRbacItemException;
import com.ecommerce.identity.rbac.domain.RbacHierarchyConflictException;
import com.ecommerce.identity.rbac.domain.RbacItem;
import com.ecommerce.identity.rbac.domain.RbacItemExistsException;
import com.ecommerce.identity.rbac.domain.RbacItemNotFoundException;
import com.ecommerce.identity.rbac.domain.RbacPermissions;
import com.ecommerce.identity.rbac.domain.RbacSystemItemProtectedException;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import java.time.Clock;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Pattern;
import lombok.RequiredArgsConstructor;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
@RequiredArgsConstructor
public class RbacAdminService {
    private static final Pattern ROLE_NAME = Pattern.compile("^[A-Z][A-Z0-9_]{1,127}$");
    private static final Pattern PERMISSION_NAME = Pattern.compile("^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)+$");

    private final RbacRepository repository;
    private final Clock clock;

    @Transactional(readOnly = true)
    public List<RbacItemView> list(AuthItemType type) {
        List<RbacItem> items = repository.findItems(type);
        if (items.isEmpty()) {
            return List.of();
        }
        Map<String, Set<String>> childrenByParent = repository.findDirectChildrenByParentNames(
                items.stream().map(RbacItem::name).toList());
        return items.stream()
                .map(item -> toView(item, childrenByParent.getOrDefault(item.name(), Set.of())))
                .toList();
    }

    @Transactional(readOnly = true)
    public RbacItemView get(String rawName) {
        return view(requireItem(rawName));
    }

    @Transactional
    public RbacItemView create(String rawName, AuthItemType type, String rawDescription) {
        String name = normalizeName(rawName);
        validateName(name, type);
        repository.lockHierarchy();
        if (repository.findItem(name).isPresent()) {
            throw new RbacItemExistsException();
        }
        Instant now = clock.instant();
        if (!repository.createItem(name, type, normalizeDescription(rawDescription), now)) {
            throw new RbacItemExistsException();
        }
        return get(name);
    }

    @Transactional
    public RbacItemView update(String rawCurrentName, String rawName, String rawDescription) {
        RbacItem current = requireItem(rawCurrentName);
        String newName = normalizeName(rawName);
        validateName(newName, current.type());
        if (current.systemItem() && !current.name().equals(newName)) {
            throw new RbacSystemItemProtectedException();
        }
        if (!current.name().equals(newName)) {
            repository.lockHierarchy();
            if (repository.findItem(newName).isPresent()) {
                throw new RbacItemExistsException();
            }
        }
        boolean updated;
        try {
            updated = repository.updateItem(
                    current.name(), newName, normalizeDescription(rawDescription), clock.instant());
        } catch (DataIntegrityViolationException exception) {
            throw new RbacItemExistsException();
        }
        if (!updated) {
            throw new RbacItemNotFoundException();
        }
        return get(newName);
    }

    @Transactional
    public void delete(String rawName) {
        RbacItem item = requireItem(rawName);
        if (item.systemItem()) {
            throw new RbacSystemItemProtectedException();
        }
        if (!repository.deleteCustomItem(item.name())) {
            throw new RbacItemNotFoundException();
        }
    }

    @Transactional
    public void addChild(String rawParent, String rawChild) {
        RbacItem parent = requireItem(rawParent);
        RbacItem child = requireItem(rawChild);
        validateEdge(parent, child);
        repository.lockHierarchy();
        if (repository.wouldCreateCycle(parent.name(), child.name())) {
            throw new RbacHierarchyConflictException();
        }
        repository.addChild(parent.name(), child.name());
    }

    @Transactional
    public void removeChild(String rawParent, String rawChild) {
        RbacItem parent = requireItem(rawParent);
        RbacItem child = requireItem(rawChild);
        if (RbacPermissions.SUPER_ADMIN.equals(parent.name())
                || RbacPermissions.SUPER_ADMIN.equals(child.name())) {
            throw new RbacSystemItemProtectedException();
        }
        repository.removeChild(parent.name(), child.name());
    }

    private RbacItem requireItem(String rawName) {
        String name = normalizeName(rawName);
        return repository.findItem(name).orElseThrow(RbacItemNotFoundException::new);
    }

    private void validateEdge(RbacItem parent, RbacItem child) {
        if (parent.name().equals(child.name())
                || (parent.type() == AuthItemType.PERMISSION && child.type() == AuthItemType.ROLE)
                || RbacPermissions.SUPER_ADMIN.equals(parent.name())
                || RbacPermissions.SUPER_ADMIN.equals(child.name())) {
            if (RbacPermissions.SUPER_ADMIN.equals(parent.name())
                    || RbacPermissions.SUPER_ADMIN.equals(child.name())) {
                throw new RbacSystemItemProtectedException();
            }
            throw new RbacHierarchyConflictException();
        }
    }

    private void validateName(String name, AuthItemType type) {
        if (type == null) {
            throw new InvalidRbacItemException(java.util.Map.of("type", "must be supplied"));
        }
        Pattern pattern = type == AuthItemType.ROLE ? ROLE_NAME : PERMISSION_NAME;
        if (name == null || !pattern.matcher(name).matches()) {
            throw new InvalidRbacItemException(java.util.Map.of(
                    "name", "must match the naming convention for " + type.name().toLowerCase()));
        }
    }

    private static String normalizeName(String name) {
        return name == null ? null : name.trim();
    }

    private static String normalizeDescription(String description) {
        return description == null ? "" : description.trim();
    }

    private RbacItemView view(RbacItem item) {
        return toView(item, repository.findDirectChildren(item.name()));
    }

    private static RbacItemView toView(RbacItem item, Set<String> directChildren) {
        return new RbacItemView(item.name(), item.type(), item.description(), item.systemItem(),
                directChildren, item.createdAt(), item.updatedAt());
    }
}
