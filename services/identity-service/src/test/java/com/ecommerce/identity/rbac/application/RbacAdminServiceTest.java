package com.ecommerce.identity.rbac.application;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.ecommerce.identity.rbac.domain.AuthItemType;
import com.ecommerce.identity.rbac.domain.RbacItem;
import com.ecommerce.identity.rbac.domain.RbacItemExistsException;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import org.junit.jupiter.api.Test;
import org.mockito.InOrder;
import org.springframework.dao.DataIntegrityViolationException;

class RbacAdminServiceTest {
    private static final Instant NOW = Instant.parse("2026-08-25T00:00:00Z");

    private final RbacRepository repository = mock(RbacRepository.class);
    private final RbacAdminService service = new RbacAdminService(
            repository, Clock.fixed(NOW, ZoneOffset.UTC));

    @Test
    void listLoadsDirectChildrenInSingleBatchQueryWithoutPerItemQueries() {
        RbacItem admin = new RbacItem("ADMIN", AuthItemType.ROLE, "Admin", true, NOW, NOW);
        RbacItem customer = new RbacItem("CUSTOMER", AuthItemType.ROLE, "Customer", true, NOW, NOW);
        RbacItem orphan = new RbacItem("SUPPORT", AuthItemType.ROLE, "Support", false, NOW, NOW);
        when(repository.findItems(AuthItemType.ROLE)).thenReturn(List.of(admin, customer, orphan));
        when(repository.findDirectChildrenByParentNames(List.of("ADMIN", "CUSTOMER", "SUPPORT")))
                .thenReturn(Map.of(
                        "ADMIN", Set.of("rbac.manage", "user.view"),
                        "CUSTOMER", Set.of("address.view")));

        List<RbacItemView> views = service.list(AuthItemType.ROLE);

        assertThat(views).hasSize(3);
        assertThat(views.get(0).directChildren()).containsExactlyInAnyOrder("rbac.manage", "user.view");
        assertThat(views.get(1).directChildren()).containsExactly("address.view");
        assertThat(views.get(2).directChildren()).isEmpty();
        verify(repository).findDirectChildrenByParentNames(List.of("ADMIN", "CUSTOMER", "SUPPORT"));
        verify(repository, never()).findDirectChildren(any());
    }

    @Test
    void updateLocksHierarchyAndTranslatesDataIntegrityViolationToRbacItemExists() {
        RbacItem current = new RbacItem("SUPPORT", AuthItemType.ROLE, "Support", false, NOW, NOW);
        when(repository.findItem("SUPPORT")).thenReturn(Optional.of(current));
        when(repository.findItem("AUDITOR")).thenReturn(Optional.empty());
        when(repository.updateItem("SUPPORT", "AUDITOR", "Auditor", NOW))
                .thenThrow(new DataIntegrityViolationException("duplicate key auth_items_pkey"));

        assertThatThrownBy(() -> service.update("SUPPORT", "AUDITOR", "Auditor"))
                .isInstanceOf(RbacItemExistsException.class);

        InOrder order = inOrder(repository);
        order.verify(repository).findItem("SUPPORT");
        order.verify(repository).lockHierarchy();
        order.verify(repository).findItem("AUDITOR");
        order.verify(repository).updateItem("SUPPORT", "AUDITOR", "Auditor", NOW);
    }
}
