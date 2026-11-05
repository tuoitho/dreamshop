package com.ecommerce.contracts.order.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.65.1)",
    comments = "Source: order/v1/order.proto")
@io.grpc.stub.annotations.GrpcGenerated
public final class OrderServiceGrpc {

  private OrderServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "ecommerce.order.v1.OrderService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.order.v1.CreateOrderRequest,
      com.ecommerce.contracts.order.v1.CreateOrderResponse> getCreateOrderMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CreateOrder",
      requestType = com.ecommerce.contracts.order.v1.CreateOrderRequest.class,
      responseType = com.ecommerce.contracts.order.v1.CreateOrderResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.order.v1.CreateOrderRequest,
      com.ecommerce.contracts.order.v1.CreateOrderResponse> getCreateOrderMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.order.v1.CreateOrderRequest, com.ecommerce.contracts.order.v1.CreateOrderResponse> getCreateOrderMethod;
    if ((getCreateOrderMethod = OrderServiceGrpc.getCreateOrderMethod) == null) {
      synchronized (OrderServiceGrpc.class) {
        if ((getCreateOrderMethod = OrderServiceGrpc.getCreateOrderMethod) == null) {
          OrderServiceGrpc.getCreateOrderMethod = getCreateOrderMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.order.v1.CreateOrderRequest, com.ecommerce.contracts.order.v1.CreateOrderResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CreateOrder"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.order.v1.CreateOrderRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.order.v1.CreateOrderResponse.getDefaultInstance()))
              .setSchemaDescriptor(new OrderServiceMethodDescriptorSupplier("CreateOrder"))
              .build();
        }
      }
    }
    return getCreateOrderMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.order.v1.GetOrderRequest,
      com.ecommerce.contracts.order.v1.GetOrderResponse> getGetOrderMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetOrder",
      requestType = com.ecommerce.contracts.order.v1.GetOrderRequest.class,
      responseType = com.ecommerce.contracts.order.v1.GetOrderResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.order.v1.GetOrderRequest,
      com.ecommerce.contracts.order.v1.GetOrderResponse> getGetOrderMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.order.v1.GetOrderRequest, com.ecommerce.contracts.order.v1.GetOrderResponse> getGetOrderMethod;
    if ((getGetOrderMethod = OrderServiceGrpc.getGetOrderMethod) == null) {
      synchronized (OrderServiceGrpc.class) {
        if ((getGetOrderMethod = OrderServiceGrpc.getGetOrderMethod) == null) {
          OrderServiceGrpc.getGetOrderMethod = getGetOrderMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.order.v1.GetOrderRequest, com.ecommerce.contracts.order.v1.GetOrderResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetOrder"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.order.v1.GetOrderRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.order.v1.GetOrderResponse.getDefaultInstance()))
              .setSchemaDescriptor(new OrderServiceMethodDescriptorSupplier("GetOrder"))
              .build();
        }
      }
    }
    return getGetOrderMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.order.v1.ListOrdersRequest,
      com.ecommerce.contracts.order.v1.ListOrdersResponse> getListOrdersMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListOrders",
      requestType = com.ecommerce.contracts.order.v1.ListOrdersRequest.class,
      responseType = com.ecommerce.contracts.order.v1.ListOrdersResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.order.v1.ListOrdersRequest,
      com.ecommerce.contracts.order.v1.ListOrdersResponse> getListOrdersMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.order.v1.ListOrdersRequest, com.ecommerce.contracts.order.v1.ListOrdersResponse> getListOrdersMethod;
    if ((getListOrdersMethod = OrderServiceGrpc.getListOrdersMethod) == null) {
      synchronized (OrderServiceGrpc.class) {
        if ((getListOrdersMethod = OrderServiceGrpc.getListOrdersMethod) == null) {
          OrderServiceGrpc.getListOrdersMethod = getListOrdersMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.order.v1.ListOrdersRequest, com.ecommerce.contracts.order.v1.ListOrdersResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListOrders"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.order.v1.ListOrdersRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.order.v1.ListOrdersResponse.getDefaultInstance()))
              .setSchemaDescriptor(new OrderServiceMethodDescriptorSupplier("ListOrders"))
              .build();
        }
      }
    }
    return getListOrdersMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.order.v1.CancelOrderRequest,
      com.ecommerce.contracts.order.v1.CancelOrderResponse> getCancelOrderMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CancelOrder",
      requestType = com.ecommerce.contracts.order.v1.CancelOrderRequest.class,
      responseType = com.ecommerce.contracts.order.v1.CancelOrderResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.order.v1.CancelOrderRequest,
      com.ecommerce.contracts.order.v1.CancelOrderResponse> getCancelOrderMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.order.v1.CancelOrderRequest, com.ecommerce.contracts.order.v1.CancelOrderResponse> getCancelOrderMethod;
    if ((getCancelOrderMethod = OrderServiceGrpc.getCancelOrderMethod) == null) {
      synchronized (OrderServiceGrpc.class) {
        if ((getCancelOrderMethod = OrderServiceGrpc.getCancelOrderMethod) == null) {
          OrderServiceGrpc.getCancelOrderMethod = getCancelOrderMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.order.v1.CancelOrderRequest, com.ecommerce.contracts.order.v1.CancelOrderResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CancelOrder"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.order.v1.CancelOrderRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.order.v1.CancelOrderResponse.getDefaultInstance()))
              .setSchemaDescriptor(new OrderServiceMethodDescriptorSupplier("CancelOrder"))
              .build();
        }
      }
    }
    return getCancelOrderMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static OrderServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<OrderServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<OrderServiceStub>() {
        @java.lang.Override
        public OrderServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new OrderServiceStub(channel, callOptions);
        }
      };
    return OrderServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static OrderServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<OrderServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<OrderServiceBlockingStub>() {
        @java.lang.Override
        public OrderServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new OrderServiceBlockingStub(channel, callOptions);
        }
      };
    return OrderServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static OrderServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<OrderServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<OrderServiceFutureStub>() {
        @java.lang.Override
        public OrderServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new OrderServiceFutureStub(channel, callOptions);
        }
      };
    return OrderServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void createOrder(com.ecommerce.contracts.order.v1.CreateOrderRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.order.v1.CreateOrderResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCreateOrderMethod(), responseObserver);
    }

    /**
     */
    default void getOrder(com.ecommerce.contracts.order.v1.GetOrderRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.order.v1.GetOrderResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetOrderMethod(), responseObserver);
    }

    /**
     */
    default void listOrders(com.ecommerce.contracts.order.v1.ListOrdersRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.order.v1.ListOrdersResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListOrdersMethod(), responseObserver);
    }

    /**
     */
    default void cancelOrder(com.ecommerce.contracts.order.v1.CancelOrderRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.order.v1.CancelOrderResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCancelOrderMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service OrderService.
   */
  public static abstract class OrderServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return OrderServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service OrderService.
   */
  public static final class OrderServiceStub
      extends io.grpc.stub.AbstractAsyncStub<OrderServiceStub> {
    private OrderServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected OrderServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new OrderServiceStub(channel, callOptions);
    }

    /**
     */
    public void createOrder(com.ecommerce.contracts.order.v1.CreateOrderRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.order.v1.CreateOrderResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCreateOrderMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void getOrder(com.ecommerce.contracts.order.v1.GetOrderRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.order.v1.GetOrderResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetOrderMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void listOrders(com.ecommerce.contracts.order.v1.ListOrdersRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.order.v1.ListOrdersResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListOrdersMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void cancelOrder(com.ecommerce.contracts.order.v1.CancelOrderRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.order.v1.CancelOrderResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCancelOrderMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service OrderService.
   */
  public static final class OrderServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<OrderServiceBlockingStub> {
    private OrderServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected OrderServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new OrderServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public com.ecommerce.contracts.order.v1.CreateOrderResponse createOrder(com.ecommerce.contracts.order.v1.CreateOrderRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCreateOrderMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.order.v1.GetOrderResponse getOrder(com.ecommerce.contracts.order.v1.GetOrderRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetOrderMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.order.v1.ListOrdersResponse listOrders(com.ecommerce.contracts.order.v1.ListOrdersRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListOrdersMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.order.v1.CancelOrderResponse cancelOrder(com.ecommerce.contracts.order.v1.CancelOrderRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCancelOrderMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service OrderService.
   */
  public static final class OrderServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<OrderServiceFutureStub> {
    private OrderServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected OrderServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new OrderServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.order.v1.CreateOrderResponse> createOrder(
        com.ecommerce.contracts.order.v1.CreateOrderRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCreateOrderMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.order.v1.GetOrderResponse> getOrder(
        com.ecommerce.contracts.order.v1.GetOrderRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetOrderMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.order.v1.ListOrdersResponse> listOrders(
        com.ecommerce.contracts.order.v1.ListOrdersRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListOrdersMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.order.v1.CancelOrderResponse> cancelOrder(
        com.ecommerce.contracts.order.v1.CancelOrderRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCancelOrderMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_CREATE_ORDER = 0;
  private static final int METHODID_GET_ORDER = 1;
  private static final int METHODID_LIST_ORDERS = 2;
  private static final int METHODID_CANCEL_ORDER = 3;

  private static final class MethodHandlers<Req, Resp> implements
      io.grpc.stub.ServerCalls.UnaryMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ServerStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ClientStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.BidiStreamingMethod<Req, Resp> {
    private final AsyncService serviceImpl;
    private final int methodId;

    MethodHandlers(AsyncService serviceImpl, int methodId) {
      this.serviceImpl = serviceImpl;
      this.methodId = methodId;
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public void invoke(Req request, io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_CREATE_ORDER:
          serviceImpl.createOrder((com.ecommerce.contracts.order.v1.CreateOrderRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.order.v1.CreateOrderResponse>) responseObserver);
          break;
        case METHODID_GET_ORDER:
          serviceImpl.getOrder((com.ecommerce.contracts.order.v1.GetOrderRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.order.v1.GetOrderResponse>) responseObserver);
          break;
        case METHODID_LIST_ORDERS:
          serviceImpl.listOrders((com.ecommerce.contracts.order.v1.ListOrdersRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.order.v1.ListOrdersResponse>) responseObserver);
          break;
        case METHODID_CANCEL_ORDER:
          serviceImpl.cancelOrder((com.ecommerce.contracts.order.v1.CancelOrderRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.order.v1.CancelOrderResponse>) responseObserver);
          break;
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        default:
          throw new AssertionError();
      }
    }
  }

  public static final io.grpc.ServerServiceDefinition bindService(AsyncService service) {
    return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
        .addMethod(
          getCreateOrderMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.order.v1.CreateOrderRequest,
              com.ecommerce.contracts.order.v1.CreateOrderResponse>(
                service, METHODID_CREATE_ORDER)))
        .addMethod(
          getGetOrderMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.order.v1.GetOrderRequest,
              com.ecommerce.contracts.order.v1.GetOrderResponse>(
                service, METHODID_GET_ORDER)))
        .addMethod(
          getListOrdersMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.order.v1.ListOrdersRequest,
              com.ecommerce.contracts.order.v1.ListOrdersResponse>(
                service, METHODID_LIST_ORDERS)))
        .addMethod(
          getCancelOrderMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.order.v1.CancelOrderRequest,
              com.ecommerce.contracts.order.v1.CancelOrderResponse>(
                service, METHODID_CANCEL_ORDER)))
        .build();
  }

  private static abstract class OrderServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    OrderServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.ecommerce.contracts.order.v1.Order.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("OrderService");
    }
  }

  private static final class OrderServiceFileDescriptorSupplier
      extends OrderServiceBaseDescriptorSupplier {
    OrderServiceFileDescriptorSupplier() {}
  }

  private static final class OrderServiceMethodDescriptorSupplier
      extends OrderServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    OrderServiceMethodDescriptorSupplier(java.lang.String methodName) {
      this.methodName = methodName;
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.MethodDescriptor getMethodDescriptor() {
      return getServiceDescriptor().findMethodByName(methodName);
    }
  }

  private static volatile io.grpc.ServiceDescriptor serviceDescriptor;

  public static io.grpc.ServiceDescriptor getServiceDescriptor() {
    io.grpc.ServiceDescriptor result = serviceDescriptor;
    if (result == null) {
      synchronized (OrderServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new OrderServiceFileDescriptorSupplier())
              .addMethod(getCreateOrderMethod())
              .addMethod(getGetOrderMethod())
              .addMethod(getListOrdersMethod())
              .addMethod(getCancelOrderMethod())
              .build();
        }
      }
    }
    return result;
  }
}
