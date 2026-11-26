package com.ecommerce.order.persistence;

import com.ecommerce.order.domain.OrderStatus;
import jakarta.persistence.LockModeType;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

public interface OrderRepository extends JpaRepository<OrderEntity, UUID> {

    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("SELECT o FROM OrderEntity o WHERE o.id = :id")
    Optional<OrderEntity> findByIdForUpdate(@Param("id") UUID id);

    Optional<OrderEntity> findByIdAndUserId(UUID id, UUID userId);

    Page<OrderEntity> findByUserIdOrderByCreatedAtDesc(UUID userId, Pageable pageable);

    Page<OrderEntity> findByUserIdAndStatusOrderByCreatedAtDesc(UUID userId, OrderStatus status, Pageable pageable);

    Page<OrderEntity> findAllByOrderByCreatedAtDesc(Pageable pageable);

    Page<OrderEntity> findByStatusOrderByCreatedAtDesc(OrderStatus status, Pageable pageable);

    Page<OrderEntity> findByOrderCodeStartingWithIgnoreCaseOrderByCreatedAtDesc(String codePrefix, Pageable pageable);

    Page<OrderEntity> findByStatusAndOrderCodeStartingWithIgnoreCaseOrderByCreatedAtDesc(
            OrderStatus status, String codePrefix, Pageable pageable);

    /**
     * Order counts per status. The prefix must be upper-case with {@code \}, {@code %} and
     * {@code _} escaped by a backslash; an empty prefix counts every order.
     */
    @Query("""
            SELECT o.status AS status, COUNT(o) AS total
            FROM OrderEntity o
            WHERE (:prefix = '' OR UPPER(o.orderCode) LIKE CONCAT(:prefix, '%') ESCAPE '\\')
            GROUP BY o.status
            """)
    List<StatusCount> countByStatus(@Param("prefix") String upperCodePrefix);

    interface StatusCount {
        OrderStatus getStatus();

        long getTotal();
    }
}
