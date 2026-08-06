package com.ecommerce.identity.user.persistence;

import java.util.Optional;
import java.util.UUID;
import org.springframework.data.jpa.repository.EntityGraph;
import org.springframework.data.jpa.repository.JpaRepository;

public interface IdentityUserRepository extends JpaRepository<IdentityUserEntity, UUID> {
    @EntityGraph(attributePaths = {"profile", "roles"})
    Optional<IdentityUserEntity> findWithRolesByNormalizedEmail(String normalizedEmail);

    @EntityGraph(attributePaths = {"profile", "roles"})
    Optional<IdentityUserEntity> findWithProfileAndRolesById(UUID id);

    boolean existsByNormalizedEmail(String normalizedEmail);

    long countByNormalizedEmail(String normalizedEmail);
}
