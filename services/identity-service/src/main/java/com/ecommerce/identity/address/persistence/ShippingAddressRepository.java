package com.ecommerce.identity.address.persistence;

import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.springframework.data.jpa.repository.JpaRepository;

public interface ShippingAddressRepository extends JpaRepository<ShippingAddressEntity, UUID> {
    List<ShippingAddressEntity> findAllByUserIdOrderByIsDefaultDescCreatedAtDescIdAsc(UUID userId);

    Optional<ShippingAddressEntity> findByIdAndUserId(UUID id, UUID userId);

    Optional<ShippingAddressEntity> findFirstByUserIdAndIsDefaultTrue(UUID userId);

    Optional<ShippingAddressEntity> findFirstByUserIdOrderByCreatedAtDescIdAsc(UUID userId);

    boolean existsByUserId(UUID userId);
}
