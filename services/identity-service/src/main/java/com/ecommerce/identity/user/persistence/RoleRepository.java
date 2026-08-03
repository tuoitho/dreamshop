package com.ecommerce.identity.user.persistence;

import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;

public interface RoleRepository extends JpaRepository<RoleEntity, Short> {
    Optional<RoleEntity> findByCode(String code);
}
