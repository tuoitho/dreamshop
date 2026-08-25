package com.ecommerce.identity.user.persistence;

import java.util.Optional;
import java.util.UUID;
import jakarta.persistence.LockModeType;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.EntityGraph;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

public interface IdentityUserRepository extends JpaRepository<IdentityUserEntity, UUID> {
    @EntityGraph(attributePaths = {"profile"})
    Optional<IdentityUserEntity> findWithProfileByNormalizedEmail(String normalizedEmail);

    @EntityGraph(attributePaths = {"profile"})
    Optional<IdentityUserEntity> findWithProfileById(UUID id);

    @EntityGraph(attributePaths = {"profile"})
    @Query("""
            select u from IdentityUserEntity u
            where (:query = ''
                or lower(u.normalizedEmail) like lower(concat('%', :query, '%'))
                or lower(u.profile.fullName) like lower(concat('%', :query, '%')))
            """)
    Page<IdentityUserEntity> searchAdminUsers(@Param("query") String query, Pageable pageable);

    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("select u from IdentityUserEntity u where u.id = :id")
    Optional<IdentityUserEntity> findForUpdateById(@Param("id") UUID id);

    boolean existsByNormalizedEmail(String normalizedEmail);

    long countByNormalizedEmail(String normalizedEmail);
}
