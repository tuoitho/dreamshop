package com.ecommerce.order.persistence;

import org.springframework.data.jpa.repository.JpaRepository;

public interface IdempotencyRecordRepository extends JpaRepository<IdempotencyRecordEntity, IdempotencyRecordId> {
}
