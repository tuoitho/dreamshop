package com.ecommerce.order.persistence;

import jakarta.persistence.Column;
import jakarta.persistence.EmbeddedId;
import jakarta.persistence.Entity;
import jakarta.persistence.Table;
import java.time.Instant;
import java.util.UUID;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.type.SqlTypes;

@Entity
@Table(name = "idempotency_records")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class IdempotencyRecordEntity {

    @EmbeddedId
    private IdempotencyRecordId id;

    @Column(name = "request_fingerprint", nullable = false, length = 64)
    private String requestFingerprint;

    @Column(name = "status", nullable = false, length = 24)
    private String status;

    @Column(name = "order_id")
    private UUID orderId;

    @JdbcTypeCode(SqlTypes.JSON)
    @Column(name = "response_json", columnDefinition = "jsonb")
    private String responseJson;

    @Column(name = "failure_code", nullable = false, length = 64)
    @Builder.Default
    private String failureCode = "";

    @Column(name = "failure_message", nullable = false, columnDefinition = "text")
    @Builder.Default
    private String failureMessage = "";

    @Column(name = "expires_at", nullable = false)
    private Instant expiresAt;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt;

    @Column(name = "updated_at", nullable = false)
    private Instant updatedAt;
}
