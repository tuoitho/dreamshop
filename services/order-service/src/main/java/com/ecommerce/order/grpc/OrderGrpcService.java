package com.ecommerce.order.grpc;

import com.ecommerce.contracts.order.v1.AdminGetOrderRequest;
import com.ecommerce.contracts.order.v1.AdminGetOrderResponse;
import com.ecommerce.contracts.order.v1.AdminListOrdersRequest;
import com.ecommerce.contracts.order.v1.AdminListOrdersResponse;
import com.ecommerce.contracts.order.v1.AdminUpdateOrderStatusRequest;
import com.ecommerce.contracts.order.v1.AdminUpdateOrderStatusResponse;
import com.ecommerce.contracts.order.v1.CancelOrderRequest;
import com.ecommerce.contracts.order.v1.CancelOrderResponse;
import com.ecommerce.contracts.order.v1.CreateOrderRequest;
import com.ecommerce.contracts.order.v1.CreateOrderResponse;
import com.ecommerce.contracts.order.v1.GetOrderRequest;
import com.ecommerce.contracts.order.v1.GetOrderResponse;
import com.ecommerce.contracts.order.v1.ListOrdersRequest;
import com.ecommerce.contracts.order.v1.ListOrdersResponse;
import com.ecommerce.contracts.order.v1.OrderDetail;
import com.ecommerce.contracts.order.v1.OrderServiceGrpc.OrderServiceImplBase;
import com.ecommerce.order.application.OrderAdminQueryService;
import com.ecommerce.order.application.OrderLifecycleService;
import com.ecommerce.order.application.OrderQueryService;
import com.ecommerce.order.application.OrderSagaOrchestrator;
import com.ecommerce.order.common.DomainException;
import io.grpc.stub.StreamObserver;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;
import org.springframework.stereotype.Service;

@Service
public class OrderGrpcService extends OrderServiceImplBase {
    private static final Logger log = LoggerFactory.getLogger(OrderGrpcService.class);

    private final OrderSagaOrchestrator orchestrator;
    private final OrderQueryService queryService;
    private final OrderAdminQueryService adminQueryService;
    private final OrderLifecycleService lifecycleService;

    public OrderGrpcService(
            OrderSagaOrchestrator orchestrator,
            OrderQueryService queryService,
            OrderAdminQueryService adminQueryService,
            OrderLifecycleService lifecycleService) {
        this.orchestrator = orchestrator;
        this.queryService = queryService;
        this.adminQueryService = adminQueryService;
        this.lifecycleService = lifecycleService;
    }

    @Override
    public void createOrder(CreateOrderRequest request, StreamObserver<CreateOrderResponse> responseObserver) {
        try {
            CreateOrderResponse response = orchestrator.createOrder(request);
            responseObserver.onNext(response);
            responseObserver.onCompleted();
        } catch (DomainException ex) {
            responseObserver.onError(GrpcErrorMapper.toStatusException(ex));
        } catch (RuntimeException ex) {
            log.error("Unexpected failure in CreateOrder traceId={}", MDC.get("trace_id"), ex);
            responseObserver.onError(GrpcErrorMapper.internalError());
        }
    }

    @Override
    public void getOrder(GetOrderRequest request, StreamObserver<GetOrderResponse> responseObserver) {
        try {
            OrderDetail detail = queryService.getOrder(request.getUserId(), request.getOrderId());
            responseObserver.onNext(GetOrderResponse.newBuilder().setOrder(detail).build());
            responseObserver.onCompleted();
        } catch (DomainException ex) {
            responseObserver.onError(GrpcErrorMapper.toStatusException(ex));
        } catch (RuntimeException ex) {
            log.error("Unexpected failure in GetOrder traceId={}", MDC.get("trace_id"), ex);
            responseObserver.onError(GrpcErrorMapper.internalError());
        }
    }

    @Override
    public void listOrders(ListOrdersRequest request, StreamObserver<ListOrdersResponse> responseObserver) {
        try {
            ListOrdersResponse response = queryService.listOrders(
                    request.getUserId(),
                    request.getStatus(),
                    request.getPage(),
                    request.getPageSize());
            responseObserver.onNext(response);
            responseObserver.onCompleted();
        } catch (DomainException ex) {
            responseObserver.onError(GrpcErrorMapper.toStatusException(ex));
        } catch (RuntimeException ex) {
            log.error("Unexpected failure in ListOrders traceId={}", MDC.get("trace_id"), ex);
            responseObserver.onError(GrpcErrorMapper.internalError());
        }
    }

    @Override
    public void cancelOrder(CancelOrderRequest request, StreamObserver<CancelOrderResponse> responseObserver) {
        try {
            OrderDetail detail = orchestrator.cancelOrder(
                    request.getUserId(),
                    request.getOrderId(),
                    request.getReason());
            responseObserver.onNext(CancelOrderResponse.newBuilder().setOrder(detail).build());
            responseObserver.onCompleted();
        } catch (DomainException ex) {
            responseObserver.onError(GrpcErrorMapper.toStatusException(ex));
        } catch (RuntimeException ex) {
            log.error("Unexpected failure in CancelOrder traceId={}", MDC.get("trace_id"), ex);
            responseObserver.onError(GrpcErrorMapper.internalError());
        }
    }

    @Override
    public void adminListOrders(AdminListOrdersRequest request, StreamObserver<AdminListOrdersResponse> responseObserver) {
        try {
            AdminListOrdersResponse response = adminQueryService.listOrders(
                    request.getUserId(),
                    request.getStatus(),
                    request.getQuery(),
                    request.getPage(),
                    request.getPageSize());
            responseObserver.onNext(response);
            responseObserver.onCompleted();
        } catch (DomainException ex) {
            responseObserver.onError(GrpcErrorMapper.toStatusException(ex));
        } catch (RuntimeException ex) {
            log.error("Unexpected failure in AdminListOrders traceId={}", MDC.get("trace_id"), ex);
            responseObserver.onError(GrpcErrorMapper.internalError());
        }
    }

    @Override
    public void adminGetOrder(AdminGetOrderRequest request, StreamObserver<AdminGetOrderResponse> responseObserver) {
        try {
            responseObserver.onNext(adminQueryService.getOrder(request.getUserId(), request.getOrderId()));
            responseObserver.onCompleted();
        } catch (DomainException ex) {
            responseObserver.onError(GrpcErrorMapper.toStatusException(ex));
        } catch (RuntimeException ex) {
            log.error("Unexpected failure in AdminGetOrder traceId={}", MDC.get("trace_id"), ex);
            responseObserver.onError(GrpcErrorMapper.internalError());
        }
    }

    @Override
    public void adminUpdateOrderStatus(
            AdminUpdateOrderStatusRequest request,
            StreamObserver<AdminUpdateOrderStatusResponse> responseObserver) {
        try {
            AdminUpdateOrderStatusResponse response = lifecycleService.updateStatusByAdmin(
                    request.getUserId(),
                    request.getOrderId(),
                    request.getExpectedStatus(),
                    request.getTargetStatus(),
                    request.getReason(),
                    request.getShippingCarrier(),
                    request.getTrackingNumber());
            responseObserver.onNext(response);
            responseObserver.onCompleted();
        } catch (DomainException ex) {
            responseObserver.onError(GrpcErrorMapper.toStatusException(ex));
        } catch (RuntimeException ex) {
            log.error("Unexpected failure in AdminUpdateOrderStatus traceId={}", MDC.get("trace_id"), ex);
            responseObserver.onError(GrpcErrorMapper.internalError());
        }
    }
}
