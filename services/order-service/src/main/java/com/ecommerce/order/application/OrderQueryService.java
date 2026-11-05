package com.ecommerce.order.application;

import com.ecommerce.contracts.order.v1.ListOrdersResponse;
import com.ecommerce.contracts.order.v1.OrderDetail;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.integration.AuthorizationGrpcAdapter;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderRepository;
import io.grpc.Status;
import java.util.Locale;
import java.util.UUID;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class OrderQueryService {
    private final OrderRepository orderRepository;
    private final AuthorizationGrpcAdapter authorizationAdapter;

    public OrderQueryService(OrderRepository orderRepository, AuthorizationGrpcAdapter authorizationAdapter) {
        this.orderRepository = orderRepository;
        this.authorizationAdapter = authorizationAdapter;
    }

    public OrderDetail getOrder(String rawUserId, String rawOrderId) {
        UUID userId = parseUuid(rawUserId, "user_id");
        UUID orderId = parseUuid(rawOrderId, "order_id");
        if (!authorizationAdapter.hasPermission(userId, "order.view")) {
            throw new DomainException(Status.Code.PERMISSION_DENIED, "ORDER_PERMISSION_DENIED", "Missing order.view permission");
        }
        return getOrderReadOnly(userId, orderId);
    }

    public ListOrdersResponse listOrders(String rawUserId, String rawStatus, int page, int pageSize) {
        UUID userId = parseUuid(rawUserId, "user_id");
        if (!authorizationAdapter.hasPermission(userId, "order.view")) {
            throw new DomainException(Status.Code.PERMISSION_DENIED, "ORDER_PERMISSION_DENIED", "Missing order.view permission");
        }
        int safePage = page <= 0 ? 1 : page;
        int safePageSize = pageSize <= 0 ? 20 : Math.min(pageSize, 100);

        OrderStatus statusFilter = null;
        if (rawStatus != null && !rawStatus.isBlank()) {
            try {
                statusFilter = OrderStatus.valueOf(rawStatus.trim().toUpperCase(Locale.ROOT));
            } catch (IllegalArgumentException ex) {
                throw new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", "Invalid order status filter");
            }
        }
        return listOrdersReadOnly(userId, statusFilter, safePage, safePageSize);
    }

    @Transactional(readOnly = true)
    protected OrderDetail getOrderReadOnly(UUID userId, UUID orderId) {
        OrderEntity order = orderRepository.findByIdAndUserId(orderId, userId)
                .orElseThrow(() -> new DomainException(Status.Code.NOT_FOUND, "ORDER_NOT_FOUND", "Order not found"));
        return OrderProtoMapper.toOrderDetail(order);
    }

    @Transactional(readOnly = true)
    protected ListOrdersResponse listOrdersReadOnly(UUID userId, OrderStatus statusFilter, int page, int pageSize) {
        PageRequest pageable = PageRequest.of(page - 1, pageSize);
        Page<OrderEntity> result = statusFilter == null
                ? orderRepository.findByUserIdOrderByCreatedAtDesc(userId, pageable)
                : orderRepository.findByUserIdAndStatusOrderByCreatedAtDesc(userId, statusFilter, pageable);

        ListOrdersResponse.Builder builder = ListOrdersResponse.newBuilder()
                .setPage(page)
                .setPageSize(pageSize)
                .setTotalItems(result.getTotalElements())
                .setTotalPages(result.getTotalPages());

        for (OrderEntity entity : result.getContent()) {
            builder.addItems(OrderProtoMapper.toOrderDetail(entity));
        }
        return builder.build();
    }

    private static UUID parseUuid(String raw, String field) {
        if (raw == null || raw.isBlank()) {
            throw new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", field + " is required");
        }
        try {
            return UUID.fromString(raw.trim());
        } catch (IllegalArgumentException ex) {
            throw new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", field + " must be a valid UUID");
        }
    }
}
