package com.ecommerce.identity.rbac.persistence;

import com.ecommerce.identity.rbac.domain.AuthItemType;
import com.ecommerce.identity.rbac.domain.RbacItem;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import java.time.Instant;
import lombok.AccessLevel;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Entity
@Table(name = "auth_items")
@Getter
@NoArgsConstructor(access = AccessLevel.PROTECTED)
public class AuthItemEntity {
    @Id
    @Column(name = "name", length = 128)
    private String name;

    @Enumerated(EnumType.STRING)
    @Column(name = "type", nullable = false, length = 16)
    private AuthItemType type;

    @Column(name = "description", length = 255)
    private String description;

    @Column(name = "system_item", nullable = false)
    private boolean systemItem;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt;

    @Column(name = "updated_at", nullable = false)
    private Instant updatedAt;

    RbacItem toDomain() {
        return new RbacItem(name, type, description, systemItem, createdAt, updatedAt);
    }
}
