package com.ecommerce.order.domain;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.EnumSet;
import java.util.Set;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;

class OrderTransitionsTest {

    @Test
    void adminTargetsFollowFulfillmentFlow() {
        assertThat(OrderTransitions.adminTargets(OrderStatus.PENDING_CONFIRMATION))
                .containsExactly(OrderStatus.CANCELLED);
        assertThat(OrderTransitions.adminTargets(OrderStatus.CONFIRMED))
                .containsExactly(OrderStatus.PREPARING, OrderStatus.CANCELLED);
        assertThat(OrderTransitions.adminTargets(OrderStatus.PREPARING))
                .containsExactly(OrderStatus.SHIPPING, OrderStatus.CANCELLED);
        assertThat(OrderTransitions.adminTargets(OrderStatus.SHIPPING))
                .containsExactly(OrderStatus.DELIVERED, OrderStatus.DELIVERY_FAILED);
        assertThat(OrderTransitions.adminTargets(OrderStatus.DELIVERY_FAILED))
                .containsExactly(OrderStatus.SHIPPING, OrderStatus.CANCELLED);
    }

    @ParameterizedTest
    @EnumSource(value = OrderStatus.class, names = {"PENDING_STOCK", "PENDING_PAYMENT", "DELIVERED", "CANCELLED"})
    void adminHasNoTargetsFromSystemOwnedOrTerminalStatuses(OrderStatus status) {
        assertThat(OrderTransitions.adminTargets(status)).isEmpty();
    }

    @ParameterizedTest
    @EnumSource(OrderStatus.class)
    void customerCanCancelOnlyBeforeFulfillmentStarts(OrderStatus status) {
        Set<OrderStatus> cancellable = EnumSet.of(
                OrderStatus.PENDING_PAYMENT, OrderStatus.PENDING_CONFIRMATION, OrderStatus.CONFIRMED);
        assertThat(OrderTransitions.customerCanCancel(status)).isEqualTo(cancellable.contains(status));
    }

    @ParameterizedTest
    @EnumSource(OrderStatus.class)
    void reasonIsRequiredForCancellationAndFailedDelivery(OrderStatus status) {
        boolean expected = status == OrderStatus.CANCELLED || status == OrderStatus.DELIVERY_FAILED;
        assertThat(OrderTransitions.requiresReason(status)).isEqualTo(expected);
    }
}
