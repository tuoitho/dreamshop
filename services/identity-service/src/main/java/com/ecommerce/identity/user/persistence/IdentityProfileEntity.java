package com.ecommerce.identity.user.persistence;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.FetchType;
import jakarta.persistence.Id;
import jakarta.persistence.JoinColumn;
import jakarta.persistence.MapsId;
import jakarta.persistence.OneToOne;
import jakarta.persistence.Table;
import java.util.Objects;
import java.util.UUID;

@Entity
@Table(name = "identity_profiles")
public class IdentityProfileEntity {
    @Id
    @Column(name = "user_id")
    private UUID userId;

    @MapsId
    @OneToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "user_id")
    private IdentityUserEntity user;

    @Column(name = "full_name", nullable = false, length = 100)
    private String fullName;

    protected IdentityProfileEntity() {
    }

    IdentityProfileEntity(IdentityUserEntity user, String fullName) {
        this.user = Objects.requireNonNull(user, "user");
        this.fullName = Objects.requireNonNull(fullName, "fullName");
    }

    String fullName() {
        return fullName;
    }
}
