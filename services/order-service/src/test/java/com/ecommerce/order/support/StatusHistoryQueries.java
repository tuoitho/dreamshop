package com.ecommerce.order.support;

import java.util.List;
import java.util.UUID;
import org.springframework.jdbc.core.JdbcTemplate;

public final class StatusHistoryQueries {

    private StatusHistoryQueries() {
    }

    /** Returns the order's history as {@code FROM>TO:ACTOR} entries, oldest first; a creation entry starts with {@code ->}. */
    public static List<String> historyOf(JdbcTemplate jdbcTemplate, UUID orderId) {
        return jdbcTemplate.queryForList(
                """
                SELECT COALESCE(from_status, '-') || '>' || to_status || ':' || actor_type
                FROM order_status_history
                WHERE order_id = ?
                ORDER BY created_at ASC, to_status ASC
                """,
                String.class,
                orderId);
    }
}
