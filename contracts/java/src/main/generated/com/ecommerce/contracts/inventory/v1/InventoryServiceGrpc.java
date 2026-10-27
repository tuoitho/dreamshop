package com.ecommerce.contracts.inventory.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.65.1)",
    comments = "Source: inventory/v1/inventory.proto")
@io.grpc.stub.annotations.GrpcGenerated
public final class InventoryServiceGrpc {

  private InventoryServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "ecommerce.inventory.v1.InventoryService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.GetAvailabilityRequest,
      com.ecommerce.contracts.inventory.v1.GetAvailabilityResponse> getGetAvailabilityMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetAvailability",
      requestType = com.ecommerce.contracts.inventory.v1.GetAvailabilityRequest.class,
      responseType = com.ecommerce.contracts.inventory.v1.GetAvailabilityResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.GetAvailabilityRequest,
      com.ecommerce.contracts.inventory.v1.GetAvailabilityResponse> getGetAvailabilityMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.GetAvailabilityRequest, com.ecommerce.contracts.inventory.v1.GetAvailabilityResponse> getGetAvailabilityMethod;
    if ((getGetAvailabilityMethod = InventoryServiceGrpc.getGetAvailabilityMethod) == null) {
      synchronized (InventoryServiceGrpc.class) {
        if ((getGetAvailabilityMethod = InventoryServiceGrpc.getGetAvailabilityMethod) == null) {
          InventoryServiceGrpc.getGetAvailabilityMethod = getGetAvailabilityMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.inventory.v1.GetAvailabilityRequest, com.ecommerce.contracts.inventory.v1.GetAvailabilityResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetAvailability"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.inventory.v1.GetAvailabilityRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.inventory.v1.GetAvailabilityResponse.getDefaultInstance()))
              .setSchemaDescriptor(new InventoryServiceMethodDescriptorSupplier("GetAvailability"))
              .build();
        }
      }
    }
    return getGetAvailabilityMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.GetStockRequest,
      com.ecommerce.contracts.inventory.v1.GetStockResponse> getGetStockMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetStock",
      requestType = com.ecommerce.contracts.inventory.v1.GetStockRequest.class,
      responseType = com.ecommerce.contracts.inventory.v1.GetStockResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.GetStockRequest,
      com.ecommerce.contracts.inventory.v1.GetStockResponse> getGetStockMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.GetStockRequest, com.ecommerce.contracts.inventory.v1.GetStockResponse> getGetStockMethod;
    if ((getGetStockMethod = InventoryServiceGrpc.getGetStockMethod) == null) {
      synchronized (InventoryServiceGrpc.class) {
        if ((getGetStockMethod = InventoryServiceGrpc.getGetStockMethod) == null) {
          InventoryServiceGrpc.getGetStockMethod = getGetStockMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.inventory.v1.GetStockRequest, com.ecommerce.contracts.inventory.v1.GetStockResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetStock"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.inventory.v1.GetStockRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.inventory.v1.GetStockResponse.getDefaultInstance()))
              .setSchemaDescriptor(new InventoryServiceMethodDescriptorSupplier("GetStock"))
              .build();
        }
      }
    }
    return getGetStockMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.BatchGetStockRequest,
      com.ecommerce.contracts.inventory.v1.BatchGetStockResponse> getBatchGetStockMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "BatchGetStock",
      requestType = com.ecommerce.contracts.inventory.v1.BatchGetStockRequest.class,
      responseType = com.ecommerce.contracts.inventory.v1.BatchGetStockResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.BatchGetStockRequest,
      com.ecommerce.contracts.inventory.v1.BatchGetStockResponse> getBatchGetStockMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.BatchGetStockRequest, com.ecommerce.contracts.inventory.v1.BatchGetStockResponse> getBatchGetStockMethod;
    if ((getBatchGetStockMethod = InventoryServiceGrpc.getBatchGetStockMethod) == null) {
      synchronized (InventoryServiceGrpc.class) {
        if ((getBatchGetStockMethod = InventoryServiceGrpc.getBatchGetStockMethod) == null) {
          InventoryServiceGrpc.getBatchGetStockMethod = getBatchGetStockMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.inventory.v1.BatchGetStockRequest, com.ecommerce.contracts.inventory.v1.BatchGetStockResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "BatchGetStock"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.inventory.v1.BatchGetStockRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.inventory.v1.BatchGetStockResponse.getDefaultInstance()))
              .setSchemaDescriptor(new InventoryServiceMethodDescriptorSupplier("BatchGetStock"))
              .build();
        }
      }
    }
    return getBatchGetStockMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.ListStocksRequest,
      com.ecommerce.contracts.inventory.v1.ListStocksResponse> getListStocksMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListStocks",
      requestType = com.ecommerce.contracts.inventory.v1.ListStocksRequest.class,
      responseType = com.ecommerce.contracts.inventory.v1.ListStocksResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.ListStocksRequest,
      com.ecommerce.contracts.inventory.v1.ListStocksResponse> getListStocksMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.ListStocksRequest, com.ecommerce.contracts.inventory.v1.ListStocksResponse> getListStocksMethod;
    if ((getListStocksMethod = InventoryServiceGrpc.getListStocksMethod) == null) {
      synchronized (InventoryServiceGrpc.class) {
        if ((getListStocksMethod = InventoryServiceGrpc.getListStocksMethod) == null) {
          InventoryServiceGrpc.getListStocksMethod = getListStocksMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.inventory.v1.ListStocksRequest, com.ecommerce.contracts.inventory.v1.ListStocksResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListStocks"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.inventory.v1.ListStocksRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.inventory.v1.ListStocksResponse.getDefaultInstance()))
              .setSchemaDescriptor(new InventoryServiceMethodDescriptorSupplier("ListStocks"))
              .build();
        }
      }
    }
    return getListStocksMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.SetStockRequest,
      com.ecommerce.contracts.inventory.v1.SetStockResponse> getSetStockMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "SetStock",
      requestType = com.ecommerce.contracts.inventory.v1.SetStockRequest.class,
      responseType = com.ecommerce.contracts.inventory.v1.SetStockResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.SetStockRequest,
      com.ecommerce.contracts.inventory.v1.SetStockResponse> getSetStockMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.SetStockRequest, com.ecommerce.contracts.inventory.v1.SetStockResponse> getSetStockMethod;
    if ((getSetStockMethod = InventoryServiceGrpc.getSetStockMethod) == null) {
      synchronized (InventoryServiceGrpc.class) {
        if ((getSetStockMethod = InventoryServiceGrpc.getSetStockMethod) == null) {
          InventoryServiceGrpc.getSetStockMethod = getSetStockMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.inventory.v1.SetStockRequest, com.ecommerce.contracts.inventory.v1.SetStockResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "SetStock"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.inventory.v1.SetStockRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.inventory.v1.SetStockResponse.getDefaultInstance()))
              .setSchemaDescriptor(new InventoryServiceMethodDescriptorSupplier("SetStock"))
              .build();
        }
      }
    }
    return getSetStockMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.AdjustStockRequest,
      com.ecommerce.contracts.inventory.v1.AdjustStockResponse> getAdjustStockMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "AdjustStock",
      requestType = com.ecommerce.contracts.inventory.v1.AdjustStockRequest.class,
      responseType = com.ecommerce.contracts.inventory.v1.AdjustStockResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.AdjustStockRequest,
      com.ecommerce.contracts.inventory.v1.AdjustStockResponse> getAdjustStockMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.inventory.v1.AdjustStockRequest, com.ecommerce.contracts.inventory.v1.AdjustStockResponse> getAdjustStockMethod;
    if ((getAdjustStockMethod = InventoryServiceGrpc.getAdjustStockMethod) == null) {
      synchronized (InventoryServiceGrpc.class) {
        if ((getAdjustStockMethod = InventoryServiceGrpc.getAdjustStockMethod) == null) {
          InventoryServiceGrpc.getAdjustStockMethod = getAdjustStockMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.inventory.v1.AdjustStockRequest, com.ecommerce.contracts.inventory.v1.AdjustStockResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "AdjustStock"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.inventory.v1.AdjustStockRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.inventory.v1.AdjustStockResponse.getDefaultInstance()))
              .setSchemaDescriptor(new InventoryServiceMethodDescriptorSupplier("AdjustStock"))
              .build();
        }
      }
    }
    return getAdjustStockMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static InventoryServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<InventoryServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<InventoryServiceStub>() {
        @java.lang.Override
        public InventoryServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new InventoryServiceStub(channel, callOptions);
        }
      };
    return InventoryServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static InventoryServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<InventoryServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<InventoryServiceBlockingStub>() {
        @java.lang.Override
        public InventoryServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new InventoryServiceBlockingStub(channel, callOptions);
        }
      };
    return InventoryServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static InventoryServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<InventoryServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<InventoryServiceFutureStub>() {
        @java.lang.Override
        public InventoryServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new InventoryServiceFutureStub(channel, callOptions);
        }
      };
    return InventoryServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void getAvailability(com.ecommerce.contracts.inventory.v1.GetAvailabilityRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.GetAvailabilityResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetAvailabilityMethod(), responseObserver);
    }

    /**
     */
    default void getStock(com.ecommerce.contracts.inventory.v1.GetStockRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.GetStockResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetStockMethod(), responseObserver);
    }

    /**
     */
    default void batchGetStock(com.ecommerce.contracts.inventory.v1.BatchGetStockRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.BatchGetStockResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getBatchGetStockMethod(), responseObserver);
    }

    /**
     */
    default void listStocks(com.ecommerce.contracts.inventory.v1.ListStocksRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.ListStocksResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListStocksMethod(), responseObserver);
    }

    /**
     */
    default void setStock(com.ecommerce.contracts.inventory.v1.SetStockRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.SetStockResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getSetStockMethod(), responseObserver);
    }

    /**
     */
    default void adjustStock(com.ecommerce.contracts.inventory.v1.AdjustStockRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.AdjustStockResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getAdjustStockMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service InventoryService.
   */
  public static abstract class InventoryServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return InventoryServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service InventoryService.
   */
  public static final class InventoryServiceStub
      extends io.grpc.stub.AbstractAsyncStub<InventoryServiceStub> {
    private InventoryServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected InventoryServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new InventoryServiceStub(channel, callOptions);
    }

    /**
     */
    public void getAvailability(com.ecommerce.contracts.inventory.v1.GetAvailabilityRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.GetAvailabilityResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetAvailabilityMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void getStock(com.ecommerce.contracts.inventory.v1.GetStockRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.GetStockResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetStockMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void batchGetStock(com.ecommerce.contracts.inventory.v1.BatchGetStockRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.BatchGetStockResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getBatchGetStockMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void listStocks(com.ecommerce.contracts.inventory.v1.ListStocksRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.ListStocksResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListStocksMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void setStock(com.ecommerce.contracts.inventory.v1.SetStockRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.SetStockResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getSetStockMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void adjustStock(com.ecommerce.contracts.inventory.v1.AdjustStockRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.AdjustStockResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getAdjustStockMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service InventoryService.
   */
  public static final class InventoryServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<InventoryServiceBlockingStub> {
    private InventoryServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected InventoryServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new InventoryServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public com.ecommerce.contracts.inventory.v1.GetAvailabilityResponse getAvailability(com.ecommerce.contracts.inventory.v1.GetAvailabilityRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetAvailabilityMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.inventory.v1.GetStockResponse getStock(com.ecommerce.contracts.inventory.v1.GetStockRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetStockMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.inventory.v1.BatchGetStockResponse batchGetStock(com.ecommerce.contracts.inventory.v1.BatchGetStockRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getBatchGetStockMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.inventory.v1.ListStocksResponse listStocks(com.ecommerce.contracts.inventory.v1.ListStocksRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListStocksMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.inventory.v1.SetStockResponse setStock(com.ecommerce.contracts.inventory.v1.SetStockRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getSetStockMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.inventory.v1.AdjustStockResponse adjustStock(com.ecommerce.contracts.inventory.v1.AdjustStockRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getAdjustStockMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service InventoryService.
   */
  public static final class InventoryServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<InventoryServiceFutureStub> {
    private InventoryServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected InventoryServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new InventoryServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.inventory.v1.GetAvailabilityResponse> getAvailability(
        com.ecommerce.contracts.inventory.v1.GetAvailabilityRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetAvailabilityMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.inventory.v1.GetStockResponse> getStock(
        com.ecommerce.contracts.inventory.v1.GetStockRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetStockMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.inventory.v1.BatchGetStockResponse> batchGetStock(
        com.ecommerce.contracts.inventory.v1.BatchGetStockRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getBatchGetStockMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.inventory.v1.ListStocksResponse> listStocks(
        com.ecommerce.contracts.inventory.v1.ListStocksRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListStocksMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.inventory.v1.SetStockResponse> setStock(
        com.ecommerce.contracts.inventory.v1.SetStockRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getSetStockMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.inventory.v1.AdjustStockResponse> adjustStock(
        com.ecommerce.contracts.inventory.v1.AdjustStockRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getAdjustStockMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_GET_AVAILABILITY = 0;
  private static final int METHODID_GET_STOCK = 1;
  private static final int METHODID_BATCH_GET_STOCK = 2;
  private static final int METHODID_LIST_STOCKS = 3;
  private static final int METHODID_SET_STOCK = 4;
  private static final int METHODID_ADJUST_STOCK = 5;

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
        case METHODID_GET_AVAILABILITY:
          serviceImpl.getAvailability((com.ecommerce.contracts.inventory.v1.GetAvailabilityRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.GetAvailabilityResponse>) responseObserver);
          break;
        case METHODID_GET_STOCK:
          serviceImpl.getStock((com.ecommerce.contracts.inventory.v1.GetStockRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.GetStockResponse>) responseObserver);
          break;
        case METHODID_BATCH_GET_STOCK:
          serviceImpl.batchGetStock((com.ecommerce.contracts.inventory.v1.BatchGetStockRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.BatchGetStockResponse>) responseObserver);
          break;
        case METHODID_LIST_STOCKS:
          serviceImpl.listStocks((com.ecommerce.contracts.inventory.v1.ListStocksRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.ListStocksResponse>) responseObserver);
          break;
        case METHODID_SET_STOCK:
          serviceImpl.setStock((com.ecommerce.contracts.inventory.v1.SetStockRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.SetStockResponse>) responseObserver);
          break;
        case METHODID_ADJUST_STOCK:
          serviceImpl.adjustStock((com.ecommerce.contracts.inventory.v1.AdjustStockRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.inventory.v1.AdjustStockResponse>) responseObserver);
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
          getGetAvailabilityMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.inventory.v1.GetAvailabilityRequest,
              com.ecommerce.contracts.inventory.v1.GetAvailabilityResponse>(
                service, METHODID_GET_AVAILABILITY)))
        .addMethod(
          getGetStockMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.inventory.v1.GetStockRequest,
              com.ecommerce.contracts.inventory.v1.GetStockResponse>(
                service, METHODID_GET_STOCK)))
        .addMethod(
          getBatchGetStockMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.inventory.v1.BatchGetStockRequest,
              com.ecommerce.contracts.inventory.v1.BatchGetStockResponse>(
                service, METHODID_BATCH_GET_STOCK)))
        .addMethod(
          getListStocksMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.inventory.v1.ListStocksRequest,
              com.ecommerce.contracts.inventory.v1.ListStocksResponse>(
                service, METHODID_LIST_STOCKS)))
        .addMethod(
          getSetStockMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.inventory.v1.SetStockRequest,
              com.ecommerce.contracts.inventory.v1.SetStockResponse>(
                service, METHODID_SET_STOCK)))
        .addMethod(
          getAdjustStockMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.inventory.v1.AdjustStockRequest,
              com.ecommerce.contracts.inventory.v1.AdjustStockResponse>(
                service, METHODID_ADJUST_STOCK)))
        .build();
  }

  private static abstract class InventoryServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    InventoryServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.ecommerce.contracts.inventory.v1.Inventory.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("InventoryService");
    }
  }

  private static final class InventoryServiceFileDescriptorSupplier
      extends InventoryServiceBaseDescriptorSupplier {
    InventoryServiceFileDescriptorSupplier() {}
  }

  private static final class InventoryServiceMethodDescriptorSupplier
      extends InventoryServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    InventoryServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (InventoryServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new InventoryServiceFileDescriptorSupplier())
              .addMethod(getGetAvailabilityMethod())
              .addMethod(getGetStockMethod())
              .addMethod(getBatchGetStockMethod())
              .addMethod(getListStocksMethod())
              .addMethod(getSetStockMethod())
              .addMethod(getAdjustStockMethod())
              .build();
        }
      }
    }
    return result;
  }
}
