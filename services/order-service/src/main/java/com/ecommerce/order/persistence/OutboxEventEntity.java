package com.ecommerce.order.persistence;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
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
@Table(name = "outbox_events")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class OutboxEventEntity {

    @Id
    @Column(name = "id", nullable = false)
    private UUID id;

    @Column(name = "event_type", nullable = false, length = 64)
    private String eventType;

    @Column(name = "routing_key", nullable = false, length = 64)
    private String routingKey;

    @Column(name = "event_version", nullable = false)
    @Builder.Default
    private int eventVersion = 1;

    @Column(name = "aggregate_id", nullable = false, length = 64)
    private String aggregateId;

    @Column(name = "correlation_id", nullable = false, length = 64)
    @Builder.Default
    private String correlationId = "";

    @JdbcTypeCode(SqlTypes.JSON)
    @Column(name = "payload", nullable = false, columnDefinition = "jsonb")
    private String payload;

    @Column(name = "attempts", nullable = false)
    @Builder.Default
    private int attempts = 0;

    // Assigned by the column default now(): the relay claims rows with
    // `next_attempt_at <= now()`, so a new row must be due by the database clock,
    // not by an application clock that may run ahead of it.
    @Column(name = "next_attempt_at", nullable = false, insertable = false, updatable = false)
    private Instant nextAttemptAt;

    @Column(name = "published_at")
    private Instant publishedAt;

    @Column(name = "last_error", nullable = false, columnDefinition = "text")
    @Builder.Default
    private String lastError = "";

    @Column(name = "created_at", nullable = false)
    private Instant createdAt;
}
