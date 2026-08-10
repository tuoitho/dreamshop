package com.ecommerce.identity.user.persistence;

import com.ecommerce.identity.auth.domain.AccountStatus;
import jakarta.persistence.CascadeType;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.FetchType;
import jakarta.persistence.Id;
import jakarta.persistence.JoinColumn;
import jakarta.persistence.JoinTable;
import jakarta.persistence.ManyToMany;
import jakarta.persistence.OneToOne;
import jakarta.persistence.Table;
import java.time.Instant;
import java.util.Collections;
import java.util.LinkedHashSet;
import java.util.Locale;
import java.util.Objects;
import java.util.Set;
import java.util.TreeSet;
import java.util.UUID;
import lombok.AccessLevel;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.experimental.Accessors;

@Entity
@Table(name = "identity_users")
@Getter
@Accessors(fluent = true)
@NoArgsConstructor(access = AccessLevel.PROTECTED)
public class IdentityUserEntity {
    @Id
    @Column(nullable = false, updatable = false)
    private UUID id;

    @Column(nullable = false, length = 254)
    private String email;

    @Column(name = "normalized_email", nullable = false, length = 254)
    private String normalizedEmail;

    @Column(name = "password_hash", nullable = false, length = 255)
    private String passwordHash;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 32)
    private AccountStatus status;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt;

    @Column(name = "updated_at", nullable = false)
    private Instant updatedAt;

    @OneToOne(mappedBy = "user", cascade = CascadeType.ALL, orphanRemoval = true, fetch = FetchType.LAZY)
    private IdentityProfileEntity profile;

    @ManyToMany(fetch = FetchType.LAZY)
    @JoinTable(name = "user_roles", joinColumns = @JoinColumn(name = "user_id"),
            inverseJoinColumns = @JoinColumn(name = "role_id"))
    private Set<RoleEntity> roles = new LinkedHashSet<>();

    public static IdentityUserEntity create(UUID id, String email, String normalizedEmail,
            String passwordHash, AccountStatus status, Instant now, String fullName, Set<RoleEntity> roles) {
        IdentityUserEntity user = new IdentityUserEntity();
        user.id = Objects.requireNonNull(id, "id");
        user.email = Objects.requireNonNull(email, "email");
        user.normalizedEmail = Objects.requireNonNull(normalizedEmail, "normalizedEmail")
                .trim()
                .toLowerCase(Locale.ROOT);
        user.passwordHash = Objects.requireNonNull(passwordHash, "passwordHash");
        user.status = Objects.requireNonNull(status, "status");
        user.createdAt = Objects.requireNonNull(now, "now");
        user.updatedAt = now;
        user.profile = new IdentityProfileEntity(user, Objects.requireNonNull(fullName, "fullName"));
        user.roles = new LinkedHashSet<>(Objects.requireNonNull(roles, "roles"));
        if (user.roles.contains(null)) {
            throw new NullPointerException("roles must not contain null");
        }
        return user;
    }

    public String fullName() {
        return profile.fullName();
    }

    public Set<String> roleCodes() {
        Set<String> codes = new TreeSet<>();
        for (RoleEntity role : roles) {
            codes.add(role.code());
        }
        return Collections.unmodifiableSet(codes);
    }

    public void disable(Instant now) {
        status = AccountStatus.DISABLED;
        updatedAt = Objects.requireNonNull(now, "now");
    }

    public void updateFullName(String fullName, Instant now) {
        profile.rename(fullName);
        updatedAt = Objects.requireNonNull(now, "now");
    }
}
