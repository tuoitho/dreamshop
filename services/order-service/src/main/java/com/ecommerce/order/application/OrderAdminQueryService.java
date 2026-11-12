package com.ecommerce.order.application;

import com.ecommerce.contracts.order.v1.AdminGetOrderResponse;
import com.ecommerce.contracts.order.v1.AdminListOrdersResponse;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.OrderTransitions;
import com.ecommerce.order.integration.AuthorizationGrpcAdapter;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderRepository;
import com.ecommerce.order.persistence.OrderStatusHistoryRepository;
import io.grpc.Status;
import java.util.Locale;
import java.util.UUID;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

@Service
public class OrderAdminQueryService {
    static final String VIEW_PERMISSION = "order.admin.view";

    private final OrderRepository orderRepository;
    private final OrderStatusHistoryRepository statusHistoryRepository;
    private final AuthorizationGrpcAdapter authorizationAdapter;
    private final TransactionTemplate readOnlyTx;

    public OrderAdminQueryService(
            OrderRepository orderRepository,
            OrderStatusHistoryRepository statusHistoryRepository,
            AuthorizationGrpcAdapter authorizationAdapter,
            PlatformTransactionManager transactionManager) {
        this.orderRepository = orderRepository;
        this.statusHistoryRepository = statusHistoryRepository;
        this.authorizationAdapter = authorizationAdapter;
        TransactionTemplate template = new TransactionTemplate(transactionManager);
        template.setReadOnly(true);
        this.readOnlyTx = template;
    }

    public AdminListOrdersResponse listOrders(String rawUserId, String rawStatus, String query, int page, int pageSize) {
        requireView(AdminRequests.requiredUuid(rawUserId, "user_id"));
        OrderStatus statusFilter = AdminRequests.optionalStatus(rawStatus, "status");
        String prefix = query == null ? "" : query.trim();
        int safePage = page <= 0 ? 1 : page;
        int safePageSize = pageSize <= 0 ? 20 : Math.min(pageSize, 100);
        return readOnlyTx.execute(status -> listReadOnly(statusFilter, prefix, safePage, safePageSize));
    }

    public AdminGetOrderResponse getOrder(String rawUserId, String rawOrderId) {
        requireView(AdminRequests.requiredUuid(rawUserId, "user_id"));
        UUID orderId = AdminRequests.requiredUuid(rawOrderId, "order_id");
        return readOnlyTx.execute(status -> {
            OrderEntity order = orderRepository.findById(orderId)
                    .orElseThrow(() -> new DomainException(Status.Code.NOT_FOUND, "ORDER_NOT_FOUND", "Order not found"));
            AdminGetOrderResponse.Builder builder = AdminGetOrderResponse.newBuilder()
                    .setOrder(OrderProtoMapper.toOrderDetail(
                            order, statusHistoryRepository.findByOrderIdOrderByCreatedAtAsc(orderId)));
            OrderTransitions.adminTargets(order.getStatus())
                    .forEach(next -> builder.addAllowedNextStatuses(next.name()));
            return builder.build();
        });
    }

    private AdminListOrdersResponse listReadOnly(OrderStatus statusFilter, String prefix, int page, int pageSize) {
        PageRequest pageable = PageRequest.of(page - 1, pageSize);
        Page<OrderEntity> result;
        if (statusFilter == null && prefix.isEmpty()) {
            result = orderRepository.findAllByOrderByCreatedAtDesc(pageable);
        } else if (prefix.isEmpty()) {
            result = orderRepository.findByStatusOrderByCreatedAtDesc(statusFilter, pageable);
        } else if (statusFilter == null) {
            result = orderRepository.findByOrderCodeStartingWithIgnoreCaseOrderByCreatedAtDesc(prefix, pageable);
        } else {
            result = orderRepository.findByStatusAndOrderCodeStartingWithIgnoreCaseOrderByCreatedAtDesc(
                    statusFilter, prefix, pageable);
        }

        AdminListOrdersResponse.Builder builder = AdminListOrdersResponse.newBuilder()
                .setPage(page)
                .setPageSize(pageSize)
                .setTotalItems(result.getTotalElements())
                .setTotalPages(result.getTotalPages());
        for (OrderEntity entity : result.getContent()) {
            builder.addItems(OrderProtoMapper.toOrderDetail(entity));
        }
        for (OrderStatus status : OrderStatus.values()) {
            builder.putStatusCounts(status.name(), 0L);
        }
        String likePrefix = prefix.toUpperCase(Locale.ROOT)
                .replace("\\", "\\\\")
                .replace("%", "\\%")
                .replace("_", "\\_");
        for (OrderRepository.StatusCount count : orderRepository.countByStatus(likePrefix)) {
            builder.putStatusCounts(count.getStatus().name(), count.getTotal());
        }
        return builder.build();
    }

    private void requireView(UUID userId) {
        if (!authorizationAdapter.hasPermission(userId, VIEW_PERMISSION)) {
            throw new DomainException(
                    Status.Code.PERMISSION_DENIED, "ORDER_PERMISSION_DENIED", "Missing " + VIEW_PERMISSION + " permission");
        }
    }
}
